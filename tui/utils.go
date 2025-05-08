package tui

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/bassamadnan/tmail/gmail"
	"github.com/charmbracelet/lipgloss"
)

var newlineRegex = regexp.MustCompile(`\r\n|\r|\n`)

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 0 {
		return ""
	}
	if maxLen < 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// formatEmailDate formats the date for display in the email list.
// NOW: Always returns "Jan 2, 3:04 PM" format.
func formatEmailDate(t time.Time) string {
	if t.IsZero() {
		return "???"
	}
	// Go's reference time: Mon Jan 2 15:04:05 -0700 MST 2006
	// "Jan 2" -> Month Day
	// "3:04 PM" -> Hour (12-hour), Minute, AM/PM marker
	return t.Local().Format("Jan 2, 3:04 PM") // e.g., "May 7, 1:15 PM", "Dec 25, 9:00 AM"
}

// sanitizeStringForLineAggressive removes newlines and other non-printable characters.
func sanitizeStringForLineAggressive(s string) string {
	s = newlineRegex.ReplaceAllString(s, " ")
	var builder strings.Builder
	for _, r := range s {
		if unicode.IsPrint(r) {
			builder.WriteRune(r)
		} else {
			builder.WriteRune(' ')
		}
	}
	s = builder.String()
	return strings.Join(strings.Fields(s), " ")
}

// formatEmailListItem formats a single email for the list view.
// itemContentTextWidth is the width for the text *inside* the box lines.
func formatEmailListItem(email gmail.ProcessedEmail, isSelected bool, itemContentTextWidth int) string {
	var boxCharStyle, subjectStyle, secondaryTextStyle lipgloss.Style
	var itemBlockStyle lipgloss.Style

	if isSelected {
		boxCharStyle = SelectedBoxCharStyle
		subjectStyle = SelectedSubjectStyle
		secondaryTextStyle = SelectedSecondaryTextStyle
		itemBlockStyle = SelectedEmailListItemStyle
	} else {
		boxCharStyle = NormalBoxCharStyle
		subjectStyle = NormalSubjectStyle
		secondaryTextStyle = NormalSecondaryTextStyle
		itemBlockStyle = EmailListItemStyle
	}

	// --- Subject Line Formatting (Line 2) ---
	subject := sanitizeStringForLineAggressive(email.Subject)
	if subject == "" {
		subject = "(No Subject)"
	}
	truncatedSubject := truncate(subject, itemContentTextWidth)
	paddedSubjectText := fmt.Sprintf("%-*s", itemContentTextWidth, truncatedSubject) // Left align subject

	// --- From / Date Line Formatting (Line 3) ---
	fromShort := sanitizeStringForLineAggressive(email.From)
	if idx := strings.Index(fromShort, "<"); idx > 0 {
		fromShort = strings.TrimSpace(fromShort[:idx])
	}
	if fromShort == "" {
		fromShort = "(Unknown Sender)"
	}
	// Get the *full* date/time string first
	dateTimeStr := formatEmailDate(email.Date) // e.g., "May 7, 1:15 PM"

	// Calculate max length for the 'from' part to fit with the date/time and at least one space
	maxFromLen := itemContentTextWidth - len(dateTimeStr) - 1 // -1 for the separating space
	if maxFromLen < 1 {
		// If date/time alone is too long, truncate it (should be rare)
		if len(dateTimeStr) > itemContentTextWidth {
			dateTimeStr = truncate(dateTimeStr, itemContentTextWidth)
		}
		fromShort = "" // No space for sender name
	} else {
		fromShort = truncate(fromShort, maxFromLen)
	}

	// Calculate padding needed to right-align the date/time
	paddingSize := itemContentTextWidth - len(fromShort) - len(dateTimeStr)
	if paddingSize < 0 {
		paddingSize = 0 // Should not happen if truncation above is correct
	}
	padding := strings.Repeat(" ", paddingSize)

	// Construct the From/Date line with right-aligned date/time
	fromToDateLineText := fmt.Sprintf("%s%s%s", fromShort, padding, dateTimeStr)

	// --- Assemble the 4 lines ---
	horizontalBar := strings.Repeat(BoxHorizontal, itemContentTextWidth+2)

	line1 := fmt.Sprintf("%s%s%s",
		boxCharStyle.Render(BoxTopLeft),
		boxCharStyle.Render(horizontalBar),
		boxCharStyle.Render(BoxTopRight),
	)
	line2 := fmt.Sprintf("%s %s %s",
		boxCharStyle.Render(BoxVertical),
		subjectStyle.Render(paddedSubjectText), // Render subject line
		boxCharStyle.Render(BoxVertical),
	)
	line3 := fmt.Sprintf("%s %s %s",
		boxCharStyle.Render(BoxVertical),
		secondaryTextStyle.Render(fromToDateLineText), // Render from/date line
		boxCharStyle.Render(BoxVertical),
	)
	line4 := fmt.Sprintf("%s%s%s",
		boxCharStyle.Render(BoxBottomLeft),
		boxCharStyle.Render(horizontalBar),
		boxCharStyle.Render(BoxBottomRight),
	)

	return itemBlockStyle.Render(strings.Join([]string{line1, line2, line3, line4}, "\n"))
}
