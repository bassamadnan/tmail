package tui

import "github.com/charmbracelet/lipgloss"

var (
	// General
	AppStyle = lipgloss.NewStyle().Padding(0, 0) // Renamed to avoid conflict if appStyle was local

	// Email List
	EmailListItemStyle         = lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1)
	SelectedEmailListItemStyle = EmailListItemStyle.Copy().Background(lipgloss.Color("240")).Foreground(lipgloss.Color("255"))
	EmailListStyle             = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, true, false, false).BorderForeground(lipgloss.Color("240")).PaddingRight(1)
	EmailListTitleStyle        = lipgloss.NewStyle().Bold(true).MarginBottom(1).MarginLeft(1).Foreground(lipgloss.Color("63"))

	// Preview & Focused View
	ContentBoxStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true).Padding(0, 1) // Adjusted padding slightly for consistency
	TitleStyle      = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("63")).Foreground(lipgloss.Color("255")).Padding(0, 1)
	HeaderKeyStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	HeaderValStyle  = lipgloss.NewStyle()
	BodyStyle       = lipgloss.NewStyle().MarginTop(1)

	// Status Bar
	StatusBarSuccessStyle = lipgloss.NewStyle().Background(lipgloss.Color("28")).Foreground(lipgloss.Color("255")).Padding(0, 1)
	StatusBarNormalStyle  = lipgloss.NewStyle().Background(lipgloss.Color("235")).Foreground(lipgloss.Color("250")).Padding(0, 1)
	StatusBarErrorStyle   = lipgloss.NewStyle().Background(lipgloss.Color("196")).Foreground(lipgloss.Color("255")).Padding(0, 1)
)

// Box drawing characters for email list items
const (
	BoxTopLeft     = "┌"
	BoxTopRight    = "┐"
	BoxBottomLeft  = "└"
	BoxBottomRight = "┘"
	BoxHorizontal  = "─"
	BoxVertical    = "│"
)
