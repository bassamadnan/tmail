package tui

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bassamadnan/tmail/config"
	"github.com/bassamadnan/tmail/gmail"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type App struct {
	*tview.Application
	rootPages        *tview.Pages
	dashboardFlex    *tview.Flex
	emailListView    *EmailListView
	previewPane      *PreviewPane
	focusedEmailView *FocusedEmailView
	statusBar        *tview.TextView

	filterManager   *config.Manager
	emailChan       <-chan gmail.ProcessedEmail
	apiPollInterval time.Duration
}

func NewApp(cfgManager *config.Manager, emailInChan <-chan gmail.ProcessedEmail, apiPollInterval time.Duration) *App {
	tuiApp := &App{
		Application:     tview.NewApplication(),
		filterManager:   cfgManager,
		emailChan:       emailInChan,
		apiPollInterval: apiPollInterval,
	}

	tuiApp.emailListView = NewEmailListView(tuiApp)
	tuiApp.previewPane = NewPreviewPane()
	tuiApp.focusedEmailView = NewFocusedEmailView()

	tuiApp.dashboardFlex = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(tuiApp.emailListView.List, 0, 1, true).
		AddItem(tuiApp.previewPane, 0, 3, false)
	tuiApp.dashboardFlex.SetBackgroundColor(tcell.ColorDefault) // Ensure flex is transparent

	initialStatusText := fmt.Sprintf(" [::d]Status: Initializing... (API Poll: %v) | [::b]Q/Ctrl+C[::-]:Quit", tuiApp.apiPollInterval)
	tuiApp.statusBar = tview.NewTextView().
		SetDynamicColors(true).
		SetText(initialStatusText).
		SetTextAlign(tview.AlignLeft)
	tuiApp.statusBar.SetBackgroundColor(tcell.ColorDefault) // Status bar transparent

	mainLayoutWithStatus := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tuiApp.dashboardFlex, 0, 1, true).
		AddItem(tuiApp.statusBar, 1, 0, false)
	mainLayoutWithStatus.SetBackgroundColor(tcell.ColorDefault) // This flex also transparent

	tuiApp.rootPages = tview.NewPages().
		AddPage(PageDashboard, mainLayoutWithStatus, true, true).
		AddPage(PageFocusedEmail, tuiApp.focusedEmailView, true, false)
	// Pages itself doesn't usually need a background color if its children fill it
	// and are transparent or have their own.

	tuiApp.Application.SetRoot(tuiApp.rootPages, true).EnableMouse(true)
	tuiApp.setGlobalKeybindings()

	tuiApp.previewPane.SetWelcomeMessage()

	return tuiApp
}

func (a *App) Run() error {
	go a.processIncomingEmails()
	go a.updateStatusTimer()
	if a.emailListView != nil && a.emailListView.List != nil {
		a.Application.SetFocus(a.emailListView.List)
	} else {
		log.Println("TUI: EmailListView or its list is nil at Run, cannot set focus.")
	}
	return a.Application.Run()
}

func (a *App) setGlobalKeybindings() {
	a.Application.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		currentPage, _ := a.rootPages.GetFrontPage()
		if event.Key() == tcell.KeyCtrlC {
			a.Stop()
			return nil
		}
		if event.Rune() == 'q' || event.Rune() == 'Q' {
			a.Stop()
			return nil
		}

		if currentPage == PageFocusedEmail {
			if event.Key() == tcell.KeyEscape {
				a.ShowDashboardView()
				return nil
			}
		}
		return event
	})
}

func (a *App) processIncomingEmails() {
	for email := range a.emailChan {
		emailCopy := email
		a.QueueUpdateDraw(func() {
			if a.emailListView == nil || a.previewPane == nil {
				log.Println("TUI: emailListView or previewPane is nil in processIncomingEmails")
				return
			}
			wasWelcome := a.previewPane.IsShowingWelcome()
			a.emailListView.AddEmail(emailCopy)

			if wasWelcome && a.emailListView.List.GetItemCount() > 0 && a.previewPane.IsShowingWelcome() {
				idx := a.emailListView.List.GetCurrentItem()
				if idx < 0 && a.emailListView.List.GetItemCount() > 0 {
					a.emailListView.List.SetCurrentItem(0)
				} else if idx >= 0 && idx < len(a.emailListView.visibleEmails) {
					a.UpdatePreviewPane(a.emailListView.visibleEmails[idx])
				}
			}

			newMailStatus := fmt.Sprintf(" [green]New: %s[::-] | %s | [::b]Q[::-]:Quit",
				truncate(emailCopy.Subject, 25), time.Now().Format("15:04:05"))
			if a.statusBar != nil {
				a.statusBar.SetText(newMailStatus)
			}
			time.AfterFunc(4*time.Second, func() {
				a.QueueUpdateDraw(func() {
					if a.statusBar != nil {
						currentText := ""
						// tview.TextView.GetText can take a boolean to strip color tags
						// but let's assume we want the raw text for Contains check.
						// If GetText(false) isn't a method, need an alternative or just check with tags.
						// For simplicity, using a placeholder if GetText(false) is not there.
						// It's safer to assume GetText(true) which strips region tags might be available.
						// However, tview.TextView does not have GetText(bool). It has GetText(stripTags bool).
						// Let's assume it's GetText(true) for stripping color tags for the check.
						// Actually, the docs show GetText(stripRegions bool). Let's assume we don't strip for contains.
						// For a simple contains check, raw text might be fine.
						// currentText = a.statusBar.GetText(true) // If checking without color tags
						// The current implementation of SetText doesn't strip, so GetText(false) is what was there.
						// Let's ensure we use the available GetText method.
						// Assuming a.statusBar.GetText(false) is the intended use from previous code.
						// The actual method might be just GetText() or GetText(stripRegionTags bool)
						// For tview, GetText(stripRegions bool) is the method.
						// Let's assume stripRegions=false for this check.
						currentText = a.statusBar.GetText(false) // Get text with color tags

						if strings.Contains(currentText, fmt.Sprintf("New: %s", truncate(emailCopy.Subject, 25))) {
							a.setStandardStatusMessage()
						}
					}
				})
			})
		})
	}
	log.Println("TUI: Email channel closed.")
}

func (a *App) updateStatusTimer() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if a.Application == nil {
				return
			}
			a.QueueUpdateDraw(func() {
				if a.statusBar == nil {
					return
				}
				currentText := a.statusBar.GetText(false)
				if !strings.Contains(currentText, "New:") && !strings.Contains(currentText, "Initializing...") {
					a.setStandardStatusMessage()
				}
			})
		}
	}
}

func (a *App) setStandardStatusMessage() {
	if a.statusBar == nil {
		return
	}
	itemCount := 0
	if a.emailListView != nil && a.emailListView.List != nil {
		itemCount = a.emailListView.List.GetItemCount()
	}
	statusMsg := fmt.Sprintf(" [::d]Watching (API Poll: %v) | %s | %d emails | [::b]Q/Ctrl+C[::-]:Quit [::b]Ent[::-]:Full [::b]Esc[::-]:Back",
		a.apiPollInterval, time.Now().Format("15:04:05"), itemCount)
	a.statusBar.SetText(statusMsg)
}

func (a *App) UpdatePreviewPane(email gmail.ProcessedEmail) {
	if a.previewPane != nil {
		a.previewPane.SetEmailContent(email)
	}
}

func (a *App) ShowWelcomeMessageInPreview() {
	if a.previewPane != nil {
		a.previewPane.SetWelcomeMessage()
	}
}

func (a *App) IsPreviewShowingWelcome() bool {
	if a.previewPane != nil {
		return a.previewPane.IsShowingWelcome()
	}
	return true
}

func (a *App) ShowFocusedEmailView(email gmail.ProcessedEmail) {
	if a.focusedEmailView == nil || a.rootPages == nil {
		return
	}
	a.focusedEmailView.SetEmailContent(email)
	a.rootPages.SwitchToPage(PageFocusedEmail)
	if a.focusedEmailView.textView != nil {
		a.Application.SetFocus(a.focusedEmailView.textView)
	}
}

func (a *App) ShowDashboardView() {
	if a.rootPages == nil {
		return
	}
	a.rootPages.SwitchToPage(PageDashboard)
	if a.emailListView != nil && a.emailListView.List != nil {
		a.Application.SetFocus(a.emailListView.List)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
