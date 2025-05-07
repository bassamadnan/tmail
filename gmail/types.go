package gmail

import "time"

// ProcessedEmail holds the essential information extracted from a Gmail message.
type ProcessedEmail struct {
	ID           string
	MessageID    string // Gmail's internal message ID
	From         string
	To           string
	Cc           string
	Date         time.Time
	Subject      string
	Snippet      string
	Body         string // Full plain text body
	IsUnread     bool   // TODO: Implement unread status tracking
	InternalDate int64  // For sorting
}
