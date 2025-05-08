package tui

import (
	"time"

	"github.com/bassamadnan/tmail/gmail"
	tea "github.com/charmbracelet/bubbletea"
)

// waitForEmailCmd listens on the email channel and sends a NewEmailMsg when an email arrives.
// It re-queues itself to continue listening unless the channel is closed.
func waitForEmailCmd(emailChan <-chan gmail.ProcessedEmail) tea.Cmd {
	return func() tea.Msg {
		email, ok := <-emailChan
		if !ok {
			// Channel closed, meaning Gmail monitoring has stopped.
			return EmailMonitorStoppedMsg{}
		}
		return NewEmailMsg(email)
	}
}

// statusTickCmd creates a ticker for updating the status bar periodically.
func statusTickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return StatusTickMsg{Time: t}
	})
}
