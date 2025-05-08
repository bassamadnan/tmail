package tui

import (
	"time"

	"github.com/bassamadnan/tmail/gmail"
)

// A message to indicate a new email has arrived.
type NewEmailMsg gmail.ProcessedEmail

// A message to indicate an error occurred, typically from a command.
type ErrorMsg struct{ Err error }

// Error makes it compatible with the error interface.
func (e ErrorMsg) Error() string { return e.Err.Error() }

// A message for timed status updates.
type StatusTickMsg struct{ Time time.Time }

// Message to signal that the email channel is closed and monitoring has stopped
type EmailMonitorStoppedMsg struct{}

// Message to clear a temporary status message after a timeout.
type clearTempStatusMsg struct{}
