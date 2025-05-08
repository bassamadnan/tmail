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
	// If maxLen is too small for "...", just truncate to maxLen.
	// Ensure maxLen is not negative or zero to avoid slice panic.
	if maxLen <= 0 {
		return ""
	}
	if maxLen < 3 {
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
// contentWidth is the width for the text *inside* the box lines (excluding the 2 spaces for padding next to vertical bars).
func formatEmailListItem(email gmail.ProcessedEmail, isSelected bool, itemContentTextWidth int) string {
	// Determine styles based on selection state
	var boxCharStyle, subjectStyle, secondaryTextStyle lipgloss.Style
	var itemBlockStyle lipgloss.Style // The overall style for the 4-line block

	if isSelected {
		boxCharStyle = SelectedBoxCharStyle
		subjectStyle = SelectedSubjectStyle
		secondaryTextStyle = SelectedSecondaryTextStyle
		itemBlockStyle = SelectedEmailListItemStyle // This style now mainly provides padding and perhaps a very subtle overall tint if desired.
	} else {
		boxCharStyle = NormalBoxCharStyle
		subjectStyle = NormalSubjectStyle
		secondaryTextStyle = NormalSecondaryTextStyle
		itemBlockStyle = EmailListItemStyle
	}

	// Prepare subject text: Truncate and pad to ensure it fills itemContentTextWidth
	subject := email.Subject
	if subject == "" {
		subject = "(No Subject)"
	}
	// Truncate the raw subject string first
	truncatedSubject := truncate(subject, itemContentTextWidth)
	// Then pad the truncated string to the full itemContentTextWidth
	paddedSubjectText := fmt.Sprintf("%-*s", itemContentTextWidth, truncatedSubject)

	// Prepare From/Date text
	fromShort := email.From
	if idx := strings.Index(fromShort, "<"); idx > 0 {
		fromShort = strings.TrimSpace(fromShort[:idx])
	}
	if fromShort == "" {
		fromShort = "(Unknown Sender)"
	}
	dateStr := formatEmailDate(email.Date)

	// Attempt to fit "From ... Date" into itemContentTextWidth
	// Max length for 'from', accounting for date (5 chars) and a space (1 char) = 6
	maxFromLen := itemContentTextWidth - len(dateStr) - 1
	if maxFromLen < 1 { // Not enough space for 'from' part, just show date or part of it
		fromShort = "" // Or truncate fromShort to what very little space is left
		if len(dateStr) > itemContentTextWidth {
			dateStr = truncate(dateStr, itemContentTextWidth)
		}
	} else {
		fromShort = truncate(fromShort, maxFromLen)
	}

	// Construct the fromDate string, ensuring it doesn't exceed itemContentTextWidth
	var fromToDateCombined string
	if fromShort != "" {
		fromToDateCombined = fmt.Sprintf("%s %s", fromShort, dateStr)
	} else {
		fromToDateCombined = dateStr
	}
	// If combined is too long, truncate (though logic above should prevent this mostly)
	if len(fromToDateCombined) > itemContentTextWidth {
		fromToDateCombined = truncate(fromToDateCombined, itemContentTextWidth)
	}
	// Pad the final fromToDateCombined string
	paddedFromToDateText := fmt.Sprintf("%-*s", itemContentTextWidth, fromToDateCombined)

	// Construct the 4 lines for the box, applying styles to parts
	// itemContentTextWidth is for the text. The horizontal bar needs to span this + 2 spaces.
	horizontalBar := strings.Repeat(BoxHorizontal, itemContentTextWidth+2)

	line1 := fmt.Sprintf("%s%s%s",
		boxCharStyle.Render(BoxTopLeft),
		boxCharStyle.Render(horizontalBar),
		boxCharStyle.Render(BoxTopRight),
	)
	line2 := fmt.Sprintf("%s %s %s",
		boxCharStyle.Render(BoxVertical),
		subjectStyle.Render(paddedSubjectText), // Render padded text with subject style
		boxCharStyle.Render(BoxVertical),
	)
	line3 := fmt.Sprintf("%s %s %s",
		boxCharStyle.Render(BoxVertical),
		secondaryTextStyle.Render(paddedFromToDateText), // Render padded text with secondary style
		boxCharStyle.Render(BoxVertical),
	)
	line4 := fmt.Sprintf("%s%s%s",
		boxCharStyle.Render(BoxBottomLeft),
		boxCharStyle.Render(horizontalBar),
		boxCharStyle.Render(BoxBottomRight),
	)

	// Join the lines and apply the overall item block style (mainly for padding)
	return itemBlockStyle.Render(strings.Join([]string{line1, line2, line3, line4}, "\n"))
}
