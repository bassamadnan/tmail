package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bassamadnan/tmail/gmail"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	PageDashboard    = "dashboard"
	PageFocusedEmail = "focusedEmail"
)

type EmailListView struct {
	*tview.List
	app           *App
	allEmails     []gmail.ProcessedEmail
	visibleEmails []gmail.ProcessedEmail
}

func NewEmailListView(app *App) *EmailListView {
	list := tview.NewList().
		ShowSecondaryText(true).
		SetSecondaryTextColor(tcell.ColorDimGray)

	list.SetBackgroundColor(tcell.ColorDefault) // Explicitly transparent
	list.SetSelectedStyle(tcell.StyleDefault.
		Foreground(tcell.ColorWhite).     // Text color for selected item
		Background(tcell.ColorSteelBlue). // Background for selected item
		Attributes(tcell.AttrBold))       // Make selected text bold

	list.SetBorder(true).SetTitle("Emails") // Border around the whole list

	elv := &EmailListView{
		List:          list,
		app:           app,
		allEmails:     []gmail.ProcessedEmail{},
		visibleEmails: []gmail.ProcessedEmail{},
	}

	list.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if elv.app == nil {
			return
		}
		if index >= 0 && index < len(elv.visibleEmails) {
			elv.app.UpdatePreviewPane(elv.visibleEmails[index])
		} else if elv.List.GetItemCount() == 0 {
			elv.app.ShowWelcomeMessageInPreview()
		}
	})

	list.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if elv.app == nil {
			return
		}
		if index >= 0 && index < len(elv.visibleEmails) {
			elv.app.ShowFocusedEmailView(elv.visibleEmails[index])
		}
	})

	return elv
}

func (elv *EmailListView) AddEmail(email gmail.ProcessedEmail) {
	elv.allEmails = append([]gmail.ProcessedEmail{email}, elv.allEmails...)
	sort.SliceStable(elv.allEmails, func(i, j int) bool {
		return elv.allEmails[i].InternalDate > elv.allEmails[j].InternalDate
	})
	elv.visibleEmails = make([]gmail.ProcessedEmail, len(elv.allEmails))
	copy(elv.visibleEmails, elv.allEmails)
	elv.updateListItems()

	if elv.app == nil {
		return
	}

	if elv.List.GetItemCount() == 1 && elv.List.GetCurrentItem() < 0 {
		elv.List.SetCurrentItem(0)
	} else if elv.List.GetItemCount() > 0 && elv.app.IsPreviewShowingWelcome() {
		currentItem := elv.List.GetCurrentItem()
		if currentItem < 0 {
			elv.List.SetCurrentItem(0)
		} else if currentItem < len(elv.visibleEmails) {
			elv.app.UpdatePreviewPane(elv.visibleEmails[currentItem])
		}
	}
}

func (elv *EmailListView) updateListItems() {
	currentSelection := elv.List.GetCurrentItem()
	elv.List.Clear()
	for _, email := range elv.visibleEmails {
		subject := email.Subject
		if len(subject) > 25 {
			subject = subject[:22] + "..."
		}
		if subject == "" {
			subject = "(No Subject)"
		}

		dateStr := ""
		if !email.Date.IsZero() {
			now := time.Now()
			if email.Date.Year() == now.Year() && email.Date.Month() == now.Month() && email.Date.Day() == now.Day() {
				dateStr = email.Date.Local().Format("15:04")
			} else {
				dateStr = email.Date.Local().Format("Jan02")
			}
		} else {
			dateStr = "???"
		}

		fromShort := email.From
		if idx := strings.Index(fromShort, "<"); idx > 0 {
			fromShort = strings.TrimSpace(fromShort[:idx])
		}
		if len(fromShort) > 15 {
			fromShort = fromShort[:12] + "..."
		}

		// Main text: Subject
		// Secondary text: From · Date, then a subtle separator line for spacing
		mainText := fmt.Sprintf("[white]%s", subject) // Ensure subject is white if terminal bg is dark
		// Using a thin Unicode line for separation. Adjust character or remove if not desired.
		// Add a newline before the separator for more visual space.
		separatorLine := strings.Repeat("─", 20) // Adjust length as needed
		secondaryText := fmt.Sprintf("[::d]%s · %s\n%s", fromShort, dateStr, separatorLine)

		elv.List.AddItem(mainText, secondaryText, 0, nil)
	}

	if elv.List.GetItemCount() > 0 {
		if currentSelection < 0 || currentSelection >= elv.List.GetItemCount() {
			currentSelection = 0
		}
		elv.List.SetCurrentItem(currentSelection)
	} else {
		if elv.app != nil {
			elv.app.ShowWelcomeMessageInPreview()
		}
	}
}

type PreviewPane struct {
	*tview.TextView
	isWelcome bool
}

func NewPreviewPane() *PreviewPane {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	tv.SetBackgroundColor(tcell.ColorDefault) // Transparent background
	tv.SetBorder(true).SetTitle("Preview")
	return &PreviewPane{TextView: tv, isWelcome: true}
}

func (pp *PreviewPane) SetEmailContent(email gmail.ProcessedEmail) {
	pp.isWelcome = false
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("[::b]From:[::-] %s\n", email.From))
	dateStr := "N/A"
	if !email.Date.IsZero() {
		dateStr = email.Date.Local().Format(time.RFC1123)
	}
	builder.WriteString(fmt.Sprintf("[::b]Date:[::-] %s\n", dateStr))
	builder.WriteString(fmt.Sprintf("[::b]Subject:[::-] %s\n\n", email.Subject))
	builder.WriteString(strings.Repeat("─", 60) + "\n\n")
	body := strings.ReplaceAll(email.Body, "\r\n", "\n")
	builder.WriteString(body)
	pp.SetText(builder.String()).ScrollToBeginning().SetTextAlign(tview.AlignLeft) // Ensure left alignment
	pp.SetTitle(fmt.Sprintf("Preview: %s", truncate(email.Subject, 40)))
}

func (pp *PreviewPane) SetWelcomeMessage() {
	pp.isWelcome = true
	// Removed SetTextAlign(tview.AlignCenter) to revert to default left alignment
	pp.SetText("\n[lightblue::b]tmail[-::-]\n\nNo email selected or list is empty.\n\n[::d]Navigate emails with ↑ ↓ keys.\nPress Enter to open in full view.\nPress Q or Ctrl+C to quit.[::-]").
		ScrollToBeginning()
	pp.SetTitle("Home")
}

func (pp *PreviewPane) IsShowingWelcome() bool {
	return pp.isWelcome
}

type FocusedEmailView struct {
	*tview.Frame
	textView *tview.TextView
}

func NewFocusedEmailView() *FocusedEmailView {
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	textView.SetBackgroundColor(tcell.ColorDefault) // Transparent background

	frame := tview.NewFrame(textView).
		AddText("", true, tview.AlignCenter, tcell.ColorYellow).
		AddText("Press Esc to go back", false, tview.AlignCenter, tcell.ColorDimGray)
	frame.SetBorder(true).SetBackgroundColor(tcell.ColorDefault) // Frame itself is transparent

	return &FocusedEmailView{
		Frame:    frame,
		textView: textView,
	}
}

func (fev *FocusedEmailView) SetEmailContent(email gmail.ProcessedEmail) {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("[::b]From:[::-] %s\n", email.From))
	builder.WriteString(fmt.Sprintf("[::b]To:[::-] %s\n", email.To))
	if email.Cc != "" {
		builder.WriteString(fmt.Sprintf("[::b]Cc:[::-] %s\n", email.Cc))
	}
	builder.WriteString(fmt.Sprintf("[::b]Date:[::-] %s\n", email.Date.Local().Format(time.RFC1123)))
	builder.WriteString(fmt.Sprintf("[::b]Subject:[::-] %s\n\n", email.Subject))
	builder.WriteString(strings.Repeat("─", 70) + "\n\n")
	body := strings.ReplaceAll(email.Body, "\r\n", "\n")
	builder.WriteString(body)
	fev.textView.SetText(builder.String()).ScrollToBeginning() // Default alignment is left
	fev.Frame.Clear().
		AddText(fmt.Sprintf("Subject: %s", truncate(email.Subject, 60)), true, tview.AlignCenter, tcell.ColorYellow).
		AddText("Press Esc to go back", false, tview.AlignCenter, tcell.ColorDimGray).
		SetPrimitive(fev.textView)
}
