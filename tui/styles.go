package tui

import "github.com/charmbracelet/lipgloss"

var (
	// General
	AppStyle = lipgloss.NewStyle().Padding(0, 0)

	// Email List
	EmailListItemStyle = lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1) // Base padding for the item block

	// For selected items, we'll change foregrounds and border colors
	// instead of a full block background to preserve the box structure.
	SelectedEmailListItemStyle = EmailListItemStyle.Copy() // Start with base padding

	// Styles for parts of the list item (normal state)
	NormalBoxCharStyle       = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "245", Dark: "238"}) // Dim gray
	NormalSubjectStyle       = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "0", Dark: "15"})    // Black/White
	NormalSecondaryTextStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "240", Dark: "244"}) // Darker Gray

	// Styles for parts of the list item (selected state)
	SelectedBoxCharStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("99"))             // A brighter border, e.g., a light purple/blue
	SelectedSubjectStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Bold(true) // White/very light, maybe bold
	SelectedSecondaryTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("189"))            // A slightly brighter dim color

	EmailListStyle      = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, true, false, false).BorderForeground(lipgloss.Color("240")).PaddingRight(1)
	EmailListTitleStyle = lipgloss.NewStyle().Bold(true).MarginBottom(1).MarginLeft(1).Foreground(lipgloss.Color("63"))

	// Preview & Focused View
	ContentBoxStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true).Padding(0, 1)
	TitleStyle      = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("63")).Foreground(lipgloss.Color("255")).Padding(0, 1)
	HeaderKeyStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	HeaderValStyle  = lipgloss.NewStyle()
	BodyStyle       = lipgloss.NewStyle().MarginTop(1)

	// Status Bar
	StatusBarSuccessStyle = lipgloss.NewStyle().Background(lipgloss.Color("28")).Foreground(lipgloss.Color("255")).Padding(0, 1)
	StatusBarNormalStyle  = lipgloss.NewStyle().Background(lipgloss.Color("235")).Foreground(lipgloss.Color("250")).Padding(0, 1)
	StatusBarErrorStyle   = lipgloss.NewStyle().Background(lipgloss.Color("196")).Foreground(lipgloss.Color("255")).Padding(0, 1)
)

// Box drawing characters
const (
	BoxTopLeft     = "┌"
	BoxTopRight    = "┐"
	BoxBottomLeft  = "└"
	BoxBottomRight = "┘"
	BoxHorizontal  = "─"
	BoxVertical    = "│"
)
