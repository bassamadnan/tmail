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

// For robust newline removal (handles \r\n, \n, \r)
var newlineRegex = regexp.MustCompile(`\r\n|\r|\n`)

// truncate shortens a string to a max length, adding "..." if truncated.
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
func formatEmailDate(t time.Time) string {
	if t.IsZero() {
		return "???"
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	emailDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())

	if emailDay.Equal(today) {
		return t.Local().Format("15:04")
	}
	return t.Local().Format("Jan 2")
}

// sanitizeStringForLineAggressive removes newlines and other non-printable characters.
func sanitizeStringForLineAggressive(s string) string {
	// First, handle explicit newline sequences by replacing them with a space
	s = newlineRegex.ReplaceAllString(s, " ")

	// Then, iterate through runes to keep only printable characters
	// and replace others with a space.
	var builder strings.Builder
	for _, r := range s {
		if unicode.IsPrint(r) { // unicode.IsPrint checks if the rune is printable
			builder.WriteRune(r)
		} else {
			builder.WriteRune(' ') // Replace non-printable with a space
		}
	}
	s = builder.String()

	// Finally, collapse multiple spaces that might have resulted from replacements
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

	// Use aggressive sanitization
	subject := sanitizeStringForLineAggressive(email.Subject)
	if subject == "" {
		subject = "(No Subject)"
	}
	truncatedSubject := truncate(subject, itemContentTextWidth)
	paddedSubjectText := fmt.Sprintf("%-*s", itemContentTextWidth, truncatedSubject)

	fromShort := sanitizeStringForLineAggressive(email.From)
	if idx := strings.Index(fromShort, "<"); idx > 0 {
		fromShort = strings.TrimSpace(fromShort[:idx])
	}
	if fromShort == "" {
		fromShort = "(Unknown Sender)"
	}
	dateStr := formatEmailDate(email.Date)

	maxFromLen := itemContentTextWidth - (len(dateStr) + 1)
	if maxFromLen < 1 {
		fromShort = ""
		if len(dateStr) > itemContentTextWidth {
			dateStr = truncate(dateStr, itemContentTextWidth)
		}
	} else {
		fromShort = truncate(fromShort, maxFromLen)
	}

	var fromToDateCombined string
	if fromShort != "" {
		fromToDateCombined = fmt.Sprintf("%s %s", fromShort, dateStr)
	} else {
		fromToDateCombined = dateStr
	}
	if len(fromToDateCombined) > itemContentTextWidth {
		fromToDateCombined = truncate(fromToDateCombined, itemContentTextWidth)
	}
	paddedFromToDateText := fmt.Sprintf("%-*s", itemContentTextWidth, fromToDateCombined)

	horizontalBar := strings.Repeat(BoxHorizontal, itemContentTextWidth+2)

	line1 := fmt.Sprintf("%s%s%s",
		boxCharStyle.Render(BoxTopLeft),
		boxCharStyle.Render(horizontalBar),
		boxCharStyle.Render(BoxTopRight),
	)
	line2 := fmt.Sprintf("%s %s %s",
		boxCharStyle.Render(BoxVertical),
		subjectStyle.Render(paddedSubjectText),
		boxCharStyle.Render(BoxVertical),
	)
	line3 := fmt.Sprintf("%s %s %s",
		boxCharStyle.Render(BoxVertical),
		secondaryTextStyle.Render(paddedFromToDateText),
		boxCharStyle.Render(BoxVertical),
	)
	line4 := fmt.Sprintf("%s%s%s",
		boxCharStyle.Render(BoxBottomLeft),
		boxCharStyle.Render(horizontalBar),
		boxCharStyle.Render(BoxBottomRight),
	)

	return itemBlockStyle.Render(strings.Join([]string{line1, line2, line3, line4}, "\n"))
}
