package gmail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bassamadnan/tmail/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const (
	tokenFile          = "token.json"
	credentialsFile    = "credentials.json"
	user               = "me"
	initialFetchCount  = 20 // Number of emails to fetch on startup
	periodicFetchCount = 10 // Number of emails to check in periodic polls
)

type Client struct {
	srv           *gmail.Service
	filterManager *config.Manager
}

func NewClient(ctx context.Context, cfgManager *config.Manager) (*Client, error) {
	b, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %w", err)
	}
	oauthConfig, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %w", err)
	}
	httpClient := getOAuthClient(oauthConfig)
	srv, err := gmail.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("unable to create Gmail service: %w", err)
	}
	return &Client{srv: srv, filterManager: cfgManager}, nil
}

func getOAuthClient(config *oauth2.Config) *http.Client {
	tok, err := tokenFromFile(tokenFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokenFile, tok)
	}
	return config.Client(context.Background(), tok)
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)
	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}
	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to save oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func (c *Client) parseEmailDetails(msg *gmail.Message) ProcessedEmail {
	email := ProcessedEmail{
		ID: msg.Id, MessageID: msg.Id, Snippet: msg.Snippet, InternalDate: msg.InternalDate,
	}
	for _, header := range msg.Payload.Headers {
		switch header.Name {
		case "Subject":
			email.Subject = header.Value
		case "From":
			email.From = header.Value
		case "To":
			email.To = header.Value
		case "Cc":
			email.Cc = header.Value
		case "Date":
			parsedDate, err := time.Parse(time.RFC1123Z, header.Value)
			if err != nil {
				parsedDate, err = time.Parse("Mon, 2 Jan 2006 15:04:05 -0700 (MST)", header.Value)
				if err != nil {
					parsedDate, err = time.Parse("Mon, 2 Jan 2006 15:04:05 -0700", header.Value) // Added this common case
					if err != nil {
						parsedDate, err = time.Parse("2 Jan 2006 15:04:05 -0700", header.Value)
						if err != nil {
							noTZParen := header.Value
							if openParen := strings.LastIndex(noTZParen, " ("); openParen != -1 {
								if closeParen := strings.LastIndex(noTZParen, ")"); closeParen > openParen {
									noTZParen = noTZParen[:openParen] + noTZParen[closeParen+1:]
								}
							}
							noTZParen = strings.TrimSpace(noTZParen)
							parsedDate, err = time.Parse("Mon, 2 Jan 2006 15:04:05 -0700", noTZParen)
							if err != nil {
								parsedDate, err = time.Parse(time.RFC1123, noTZParen)
								if err != nil {
									parsedDate, err = time.Parse(time.RFC822, noTZParen)
									if err != nil {
										log.Printf("Warning: Could not parse date string '%s' (original: '%s'): %v", noTZParen, header.Value, err)
									}
								}
							}
						}
					}
				}
			}
			email.Date = parsedDate
		}
	}
	if msg.Payload != nil {
		email.Body = getPlainTextBody(msg.Payload)
	}
	return email
}

func getPlainTextBody(payload *gmail.MessagePart) string {
	if payload.MimeType == "text/plain" && payload.Body != nil && payload.Body.Data != "" {
		data, err := base64.URLEncoding.DecodeString(payload.Body.Data)
		if err == nil {
			return string(data)
		}
		log.Printf("Error decoding base64 body for text/plain: %v", err)
	}
	if payload.Parts != nil {
		for _, part := range payload.Parts {
			if strings.HasPrefix(strings.ToLower(part.MimeType), "text/") ||
				strings.HasPrefix(strings.ToLower(part.MimeType), "multipart/") {
				if body := getPlainTextBody(part); body != "" {
					return body
				}
			}
		}
	}
	return ""
}

func (c *Client) applyFilters(email *ProcessedEmail) bool {
	filters := c.filterManager.GetFilters()
	for _, sender := range filters.IgnoreSenders {
		if strings.Contains(strings.ToLower(email.From), strings.ToLower(sender)) {
			log.Printf("Filtering email from %s due to sender rule: %s", email.From, sender)
			return true
		}
	}
	for _, keyword := range filters.IgnoreKeywordsInSubject {
		if strings.Contains(strings.ToLower(email.Subject), strings.ToLower(keyword)) {
			log.Printf("Filtering email with subject '%s' due to keyword rule: %s", email.Subject, keyword)
			return true
		}
	}
	return false
}

func (c *Client) StartMonitoring(ctx context.Context, emailChan chan<- ProcessedEmail, initialDelay time.Duration, pollInterval time.Duration) {
	var lastMessageId string
	time.Sleep(initialDelay)

	// Query to get messages in INBOX and NOT in DRAFTS.
	// This will fetch from all categories (Primary, Social, Promotions, etc.) within the inbox.
	inboxNotDraftQuery := "in:inbox -in:draft"

	log.Printf("Gmail Monitor: Performing initial fetch for last %d emails (inbox, not drafts)...", initialFetchCount)
	initialListCall := c.srv.Users.Messages.List(user).
		MaxResults(initialFetchCount).
		Q(inboxNotDraftQuery) // ADDED: Query to filter

	initialList, err := initialListCall.Do()
	if err != nil {
		log.Printf("Gmail Monitor: Unable to retrieve initial list of messages: %v.", err)
	} else if len(initialList.Messages) == 0 {
		log.Println("Gmail Monitor: No messages found in initial fetch (inbox, not drafts).")
	} else {
		log.Printf("Gmail Monitor: Fetched %d initial messages (inbox, not drafts).", len(initialList.Messages))
		if len(initialList.Messages) > 0 {
			lastMessageId = initialList.Messages[0].Id
			log.Printf("Gmail Monitor: Baseline for future polls set to message ID %s.", lastMessageId)
		}

		for i := len(initialList.Messages) - 1; i >= 0; i-- {
			msgID := initialList.Messages[i].Id
			fullMsg, err := c.srv.Users.Messages.Get(user, msgID).Format("full").Do()
			if err != nil {
				log.Printf("Gmail Monitor: Unable to retrieve full initial message %s: %v", msgID, err)
				continue
			}
			processedEmail := c.parseEmailDetails(fullMsg)
			if !c.applyFilters(&processedEmail) {
				select {
				case emailChan <- processedEmail:
					log.Printf("Gmail Monitor: Sent initial email '%s' to TUI.", processedEmail.Subject)
				case <-ctx.Done():
					log.Println("Gmail Monitor: Context cancelled while sending initial email.")
					return
				}
			}
		}
	}
	log.Println("Gmail Monitor: Initial message processing complete. Starting periodic checks (inbox, not drafts)...")

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Gmail Monitor: Stopping.")
			return
		case <-ticker.C:
			log.Printf("Gmail Monitor: Checking for new messages (inbox, not drafts)...")
			newListCall := c.srv.Users.Messages.List(user).
				MaxResults(periodicFetchCount).
				Q(inboxNotDraftQuery) // ADDED: Query to filter

			newList, err := newListCall.Do()
			if err != nil {
				log.Printf("Gmail Monitor: Error checking for new messages: %v", err)
				continue
			}
			if len(newList.Messages) == 0 {
				log.Println("Gmail Monitor: No new messages found this poll (inbox, not drafts).")
				continue
			}

			var newMessagesToProcess []*gmail.Message
			foundLastMessage := false
			if lastMessageId == "" && len(newList.Messages) > 0 {
				log.Println("Gmail Monitor: No previous lastMessageId, processing all fetched messages as new.")
				newMessagesToProcess = newList.Messages
			} else if lastMessageId != "" {
				for _, m := range newList.Messages {
					if m.Id == lastMessageId {
						foundLastMessage = true
						break
					}
					newMessagesToProcess = append(newMessagesToProcess, m)
				}
			}

			if !foundLastMessage && lastMessageId != "" && len(newMessagesToProcess) == periodicFetchCount {
				log.Printf("Gmail Monitor: All %d fetched messages are new and different from last ID %s. This matches periodicFetchCount, so there might be more new emails than fetched.", len(newMessagesToProcess), lastMessageId)
			} else if len(newMessagesToProcess) > 0 {
				log.Printf("Gmail Monitor: Found %d new messages to process.", len(newMessagesToProcess))
			}

			for i := len(newMessagesToProcess) - 1; i >= 0; i-- {
				msgID := newMessagesToProcess[i].Id
				fullMsg, err := c.srv.Users.Messages.Get(user, msgID).Format("full").Do()
				if err != nil {
					log.Printf("Gmail Monitor: Unable to retrieve full message %s: %v", msgID, err)
					continue
				}
				processedEmail := c.parseEmailDetails(fullMsg)
				if !c.applyFilters(&processedEmail) {
					select {
					case emailChan <- processedEmail:
						log.Printf("Gmail Monitor: Sent new email '%s' to TUI.", processedEmail.Subject)
					case <-ctx.Done():
						log.Println("Gmail Monitor: Context cancelled while sending email.")
						return
					}
				}
			}

			if len(newMessagesToProcess) > 0 {
				lastMessageId = newList.Messages[0].Id
				log.Printf("Gmail Monitor: Updated lastMessageId to %s", lastMessageId)
			}
		}
	}
}
