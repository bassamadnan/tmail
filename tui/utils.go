package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/bassamadnan/tmail/gmail"
	"github.com/charmbracelet/lipgloss"
)

// truncate shortens a string to a max length, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 3 { // Need space for "..."
		if maxLen <= 0 {
			return ""
		}
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// formatEmailDate formats the date for display in the email list.
func formatEmailDate(t time.Time) string {
	if t.IsZero() {
		return "???"
	}
	now := time.Now()
	if t.Year() == now.Year() && t.Month() == now.Month() && t.Day() == now.Day() {
		return t.Local().Format("15:04") // Time only for today
	}
	return t.Local().Format("Jan02") // Date for other days
}

// formatEmailListItem formats a single email for the list view.
// contentWidth is the width for the text inside the box lines.
func formatEmailListItem(email gmail.ProcessedEmail, isSelected bool, contentWidth int) string {
	styleToUse := EmailListItemStyle // Use the package-level var
	if isSelected {
		styleToUse = SelectedEmailListItemStyle // Use the package-level var
	}

	subject := truncate(email.Subject, contentWidth)
	if subject == "" {
		subject = "(No Subject)"
	}
	subjectText := fmt.Sprintf("%-*s", contentWidth, subject)

	fromShort := email.From
	if idx := strings.Index(fromShort, "<"); idx > 0 {
		fromShort = strings.TrimSpace(fromShort[:idx])
	}
	if fromShort == "" {
		fromShort = "(Unknown Sender)"
	}
	// Estimate space for date part (e.g., "Jan02" or "15:04" which is 5 chars) + a space
	fromMaxLen := contentWidth - 6
	if fromMaxLen < 1 {
		fromMaxLen = 1
	} // Ensure at least 1 char for from part
	fromShort = truncate(fromShort, fromMaxLen)

	dateStr := formatEmailDate(email.Date)

	fromAndDate := fmt.Sprintf("%s %s", fromShort, dateStr)
	// Ensure the combined string fits, pad if shorter
	if len(fromAndDate) > contentWidth {
		fromAndDate = truncate(fromAndDate, contentWidth)
	}
	fromToDateText := fmt.Sprintf("%-*s", contentWidth, fromAndDate)

	boxColor := lipgloss.AdaptiveColor{Light: "245", Dark: "238"}
	subjectColor := lipgloss.AdaptiveColor{Light: "0", Dark: "15"}
	secondaryColor := lipgloss.AdaptiveColor{Light: "240", Dark: "244"}

	if isSelected {
		// For selected items, we often want the foreground colors to contrast with the selection background
		// Assuming SelectedEmailListItemStyle has a dark background
		boxColor = lipgloss.AdaptiveColor{Light: "252", Dark: "252"}       // Lighter gray for box on selection
		subjectColor = lipgloss.AdaptiveColor{Light: "231", Dark: "231"}   // White/very light for subject
		secondaryColor = lipgloss.AdaptiveColor{Light: "248", Dark: "248"} // Light gray for secondary
	}

	boxStyle := lipgloss.NewStyle().Foreground(boxColor)
	subjectContentStyle := lipgloss.NewStyle().Foreground(subjectColor)
	secondaryContentStyle := lipgloss.NewStyle().Foreground(secondaryColor)

	// Use package-level consts for box drawing characters
	line1 := boxStyle.Render(fmt.Sprintf("%s%s%s", BoxTopLeft, strings.Repeat(BoxHorizontal, contentWidth+2), BoxTopRight))
	line2 := fmt.Sprintf("%s %s %s", boxStyle.Render(BoxVertical), subjectContentStyle.Render(subjectText), boxStyle.Render(BoxVertical))
	line3 := fmt.Sprintf("%s %s %s", boxStyle.Render(BoxVertical), secondaryContentStyle.Render(fromToDateText), boxStyle.Render(BoxVertical))
	line4 := boxStyle.Render(fmt.Sprintf("%s%s%s", BoxBottomLeft, strings.Repeat(BoxHorizontal, contentWidth+2), BoxBottomRight))

	return styleToUse.Render(strings.Join([]string{line1, line2, line3, line4}, "\n"))
}
