package tui

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/bassamadnan/tmail/config"
	"github.com/bassamadnan/tmail/gmail"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewState int

const (
	viewLoading viewState = iota
	viewDashboard
	viewFocusedEmail
)

const (
	emailListItemHeight = 4
	minListPaneWidth    = 30
	minPreviewPaneWidth = 40
)

type Model struct {
	configManager   *config.Manager
	emailChan       <-chan gmail.ProcessedEmail
	apiPollInterval time.Duration

	allEmails        []gmail.ProcessedEmail
	selectedIdx      int
	viewportTopLine  int
	previewScrollPos int

	currentView viewState

	width, height int
	statusBarText string
	statusIsError bool
	statusIsTemp  bool

	err                error
	isGmailMonitorDone bool
}

func NewInitialModel(cfgManager *config.Manager, emailChan <-chan gmail.ProcessedEmail, pollInterval time.Duration) Model {
	return Model{
		configManager:   cfgManager,
		emailChan:       emailChan,
		apiPollInterval: pollInterval,
		currentView:     viewLoading,
		statusBarText:   "Initializing, connecting to Gmail...",
		allEmails:       []gmail.ProcessedEmail{},
		selectedIdx:     0,
	}
}

func (m Model) Init() tea.Cmd {
	log.Println("TUI Model Init called")
	return tea.Batch(
		waitForEmailCmd(m.emailChan),
		statusTickCmd(1*time.Second),
	)
}

func (m Model) getVisibleEmailListHeight() int {
	statusBarHeight := 1
	listTitleRenderedHeight := lipgloss.Height(EmailListTitleStyle.Render(" "))
	availableHeight := m.height - statusBarHeight - listTitleRenderedHeight
	if availableHeight < 0 {
		availableHeight = 0
	}
	return availableHeight
}

func (m Model) getNumItemsThatFitInList() int {
	h := m.getVisibleEmailListHeight()
	if emailListItemHeight == 0 {
		return 0
	}
	numFit := h / emailListItemHeight
	if numFit < 0 {
		numFit = 0
	} // Ensure it's not negative
	return numFit
}

// getVisiblePreviewBodyHeight estimates the number of text lines available for the email body in the preview pane.
// THIS IS CALLED AFTER RENDERING HEADERS in the refined renderPreviewPane.
func (m Model) getVisiblePreviewBodyHeight(paneTotalHeight int, renderedHeaderHeight int) int {
	previewTitleHeight := lipgloss.Height(TitleStyle.Render(" ")) // Height of the title bar itself

	// Available height for body is total pane height minus title, minus rendered headers, minus container paddings
	availableHeight := paneTotalHeight - previewTitleHeight - renderedHeaderHeight - ContentBoxStyle.GetVerticalPadding()
	if availableHeight < 0 {
		availableHeight = 0
	}
	return availableHeight
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureSelectedVisible()
		if m.currentView == viewLoading && m.width > 0 {
			if len(m.allEmails) > 0 || m.isGmailMonitorDone {
				m.currentView = viewDashboard
				m.setStandardStatus()
			} else {
				m.updateStatusBar("Waiting for initial emails...")
			}
		}

	case tea.KeyMsg:
		switch m.currentView {
		case viewDashboard:
			switch msg.String() {
			case "ctrl+c", "q":
				m.updateStatusBar("Quitting...")
				return m, tea.Quit
			case "up", "k":
				if m.selectedIdx > 0 {
					m.selectedIdx--
					m.ensureSelectedVisible()
					m.previewScrollPos = 0
				}
			case "down", "j":
				if m.selectedIdx < len(m.allEmails)-1 {
					m.selectedIdx++
					m.ensureSelectedVisible()
					m.previewScrollPos = 0
				}
			case "enter":
				if len(m.allEmails) > 0 && m.selectedIdx >= 0 && m.selectedIdx < len(m.allEmails) {
					m.currentView = viewFocusedEmail
					m.setStandardStatus()
				}
			case "K": // Preview scroll up (Shift+K)
				if m.previewScrollPos > 0 {
					m.previewScrollPos--
				}
			case "J": // Preview scroll down (Shift+J)
				// This scrolling logic for 'J' can be complex without a proper viewport.
				// The key is to know how many lines of body text are *actually* visible.
				// The current getVisiblePreviewBodyHeight is an estimate made after header rendering.
				if len(m.allEmails) > 0 && m.selectedIdx >= 0 && m.selectedIdx < len(m.allEmails) {
					email := m.allEmails[m.selectedIdx]
					bodyLines := strings.Split(strings.ReplaceAll(email.Body, "\r\n", "\n"), "\n")

					// We need a way to get the *actual* rendered header height for a more accurate calculation here.
					// For simplicity now, we'll keep it as is, but this is where a viewport shines.
					// A more robust check would be:
					// if m.previewScrollPos < len(bodyLines) - (lines_that_actually_fit_on_screen_for_body)
					if m.previewScrollPos < len(bodyLines)-1 { // Simplified: just don't scroll past last line
						m.previewScrollPos++
					}
				}
			}
		case viewFocusedEmail:
			switch msg.String() {
			case "ctrl+c", "q":
				m.updateStatusBar("Quitting...")
				return m, tea.Quit
			case "esc":
				m.currentView = viewDashboard
				m.setStandardStatus()
			}
		case viewLoading:
			switch msg.String() {
			case "ctrl+c", "q":
				m.updateStatusBar("Quitting...")
				return m, tea.Quit
			}
		}

	case NewEmailMsg:
		newEmail := gmail.ProcessedEmail(msg)
		oldSelectedEmailID := ""
		if len(m.allEmails) > 0 && m.selectedIdx >= 0 && m.selectedIdx < len(m.allEmails) {
			oldSelectedEmailID = m.allEmails[m.selectedIdx].ID
		}

		m.allEmails = append(m.allEmails, newEmail)
		sort.SliceStable(m.allEmails, func(i, j int) bool {
			return m.allEmails[i].InternalDate > m.allEmails[j].InternalDate
		})

		newIdxFound := false
		if oldSelectedEmailID != "" {
			for i, e := range m.allEmails {
				if e.ID == oldSelectedEmailID {
					m.selectedIdx = i
					newIdxFound = true
					break
				}
			}
		}
		if !newIdxFound || len(m.allEmails) == 1 {
			m.selectedIdx = 0
			if len(m.allEmails) > 0 {
				for i, e := range m.allEmails {
					if e.ID == newEmail.ID {
						m.selectedIdx = i
						break
					}
				}
			}
		}
		if m.selectedIdx >= len(m.allEmails) && len(m.allEmails) > 0 {
			m.selectedIdx = len(m.allEmails) - 1
		}
		if m.selectedIdx < 0 && len(m.allEmails) > 0 {
			m.selectedIdx = 0
		}

		if m.currentView == viewLoading && m.width > 0 {
			m.currentView = viewDashboard
			m.setStandardStatus()
		} else {
			m.showTemporaryStatus(fmt.Sprintf("New: %s", truncate(newEmail.Subject, 30)), 4*time.Second, &cmds)
		}
		m.ensureSelectedVisible()
		cmds = append(cmds, waitForEmailCmd(m.emailChan))

	case EmailMonitorStoppedMsg:
		m.isGmailMonitorDone = true
		if m.currentView == viewLoading {
			m.currentView = viewDashboard
			m.updateStatusBar("Email monitoring stopped. No new emails will be fetched.")
		} else if !m.statusIsTemp {
			m.setStandardStatus()
		}
		log.Println("TUI: Email monitor stopped message received.")

	case ErrorMsg:
		m.err = msg.Err
		m.updateStatusError(fmt.Sprintf("Error: %v", msg.Err))

	case StatusTickMsg:
		if !m.statusIsTemp && m.currentView != viewLoading {
			m.setStandardStatus()
		}
		cmds = append(cmds, statusTickCmd(1*time.Second))

	case clearTempStatusMsg:
		if m.statusIsTemp {
			m.statusIsTemp = false
			m.setStandardStatus()
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) showTemporaryStatus(text string, duration time.Duration, cmds *[]tea.Cmd) {
	m.statusBarText = text
	m.statusIsError = false
	m.statusIsTemp = true
	*cmds = append(*cmds, tea.Tick(duration, func(t time.Time) tea.Msg {
		return clearTempStatusMsg{}
	}))
}

func (m *Model) updateStatusBar(text string) {
	m.statusBarText = text
	m.statusIsError = false
	m.statusIsTemp = false
}

func (m *Model) updateStatusError(text string) {
	m.statusBarText = text
	m.statusIsError = true
	m.statusIsTemp = false
}

func (m *Model) setStandardStatus() {
	if m.statusIsTemp {
		return
	}

	monitorStatus := "Watching"
	if m.isGmailMonitorDone {
		monitorStatus = "Monitor Off"
	}

	statusMsg := fmt.Sprintf(" %s (API Poll: %v) | %s | %d emails ",
		monitorStatus, m.apiPollInterval, time.Now().Format("15:04:05"), len(m.allEmails))

	keyHints := "[Q/Ctrl+C]:Quit"
	switch m.currentView {
	case viewDashboard:
		keyHints += " | [↑↓/jk]:Nav | [Enter]:Full | [KJ]:Scroll Preview"
	case viewFocusedEmail:
		keyHints += " | [Esc]:Back"
	case viewLoading:
		keyHints = "[Q/Ctrl+C]:Quit"
	}
	m.updateStatusBar(statusMsg + "| " + keyHints)
}

func (m *Model) ensureSelectedVisible() {
	if len(m.allEmails) == 0 {
		m.viewportTopLine = 0
		return
	}

	itemsThatFit := m.getNumItemsThatFitInList()
	if itemsThatFit <= 0 {
		m.viewportTopLine = m.selectedIdx
		return
	}

	if m.selectedIdx < m.viewportTopLine {
		m.viewportTopLine = m.selectedIdx
	} else if m.selectedIdx >= m.viewportTopLine+itemsThatFit {
		m.viewportTopLine = m.selectedIdx - itemsThatFit + 1
	}

	if m.viewportTopLine < 0 {
		m.viewportTopLine = 0
	}
	maxPossibleViewportTop := len(m.allEmails) - itemsThatFit
	if maxPossibleViewportTop < 0 {
		maxPossibleViewportTop = 0
	}
	if m.viewportTopLine > maxPossibleViewportTop {
		m.viewportTopLine = maxPossibleViewportTop
	}
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing terminal size..."
	}
	if m.err != nil {
		return fmt.Sprintf("\n   Application Error: %v\n\n   Press Ctrl+C to quit.", m.err)
	}

	var mainUIView string
	statusBarHeight := 1
	// contentHeight is the total height available for the main view area (list + preview, or focused)
	contentHeight := m.height - statusBarHeight
	if contentHeight < 0 {
		contentHeight = 0
	}

	switch m.currentView {
	case viewLoading:
		loadingText := "Loading emails..."
		if m.statusBarText != "" && m.statusBarText != "Initializing, connecting to Gmail..." {
			loadingText = m.statusBarText
		}
		mainUIView = lipgloss.Place(m.width, contentHeight, lipgloss.Center, lipgloss.Center, loadingText)
	case viewDashboard:
		listPaneTargetWidth := int(float64(m.width) * 0.35)
		actualListPaneWidth := listPaneTargetWidth
		if actualListPaneWidth < minListPaneWidth {
			actualListPaneWidth = minListPaneWidth
		}
		if actualListPaneWidth > m.width-minPreviewPaneWidth && m.width > minPreviewPaneWidth { // ensure preview has its min if possible
			actualListPaneWidth = m.width - minPreviewPaneWidth
		}
		if actualListPaneWidth < 0 {
			actualListPaneWidth = 0
		}
		if actualListPaneWidth > m.width {
			actualListPaneWidth = m.width
		}

		actualPreviewPaneWidth := m.width - actualListPaneWidth
		if actualPreviewPaneWidth < 0 {
			actualPreviewPaneWidth = 0
		}

		if m.width < minListPaneWidth+minPreviewPaneWidth { // Handle very narrow screens
			if m.width < minListPaneWidth { // Only space for list (or less)
				actualListPaneWidth = m.width
				actualPreviewPaneWidth = 0
			} else { // Space for min list, rest for preview
				actualListPaneWidth = minListPaneWidth
				actualPreviewPaneWidth = m.width - actualListPaneWidth
			}
		}

		// Crucially, renderEmailList and renderPreviewPane now take contentHeight,
		// which is the total height they are allowed to occupy.
		// Their internal styles should then respect this with .Height(contentHeight)
		emailListRendered := m.renderEmailList(actualListPaneWidth, contentHeight)
		previewPaneRendered := m.renderPreviewPane(actualPreviewPaneWidth, contentHeight)

		mainUIView = lipgloss.JoinHorizontal(lipgloss.Top, emailListRendered, previewPaneRendered)

	case viewFocusedEmail:
		// Similar to above, focused view should also respect contentHeight
		mainUIView = m.renderFocusedEmailView(m.width, contentHeight)
	}

	statusBarRendered := m.renderStatusBar()
	return AppStyle.Render(lipgloss.JoinVertical(lipgloss.Left, mainUIView, statusBarRendered))
}

func (m Model) renderEmailList(paneWidth, paneHeight int) string {
	title := EmailListTitleStyle.Render("Emails")
	listItemsContainerHeight := paneHeight - lipgloss.Height(title)
	if listItemsContainerHeight < 0 {
		listItemsContainerHeight = 0
	}

	var listItemsContent strings.Builder
	itemTextContentWidth := paneWidth - EmailListItemStyle.GetPaddingLeft() - EmailListItemStyle.GetPaddingRight() - 2 - 2
	if itemTextContentWidth < 10 {
		itemTextContentWidth = 10
	}

	numItemsToDisplay := 0
	if emailListItemHeight > 0 {
		numItemsToDisplay = listItemsContainerHeight / emailListItemHeight
	}
	if numItemsToDisplay < 0 {
		numItemsToDisplay = 0
	}

	startIdx := m.viewportTopLine
	endIdx := startIdx + numItemsToDisplay
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx > len(m.allEmails) {
		startIdx = len(m.allEmails)
	}
	if endIdx > len(m.allEmails) {
		endIdx = len(m.allEmails)
	}
	if endIdx < startIdx {
		endIdx = startIdx
	}

	visibleEmailItemStrings := []string{}
	if paneWidth > 0 && paneHeight > 0 {
		for i := startIdx; i < endIdx; i++ {
			email := m.allEmails[i]
			isSelected := (i == m.selectedIdx)
			itemStr := formatEmailListItem(email, isSelected, itemTextContentWidth)
			visibleEmailItemStrings = append(visibleEmailItemStrings, itemStr)
		}
	}
	listItemsContent.WriteString(strings.Join(visibleEmailItemStrings, "\n"))

	fullListRender := lipgloss.JoinVertical(lipgloss.Left, title, listItemsContent.String())
	// Ensure the EmailListStyle itself is constrained by paneWidth and paneHeight
	return EmailListStyle.Width(paneWidth).Height(paneHeight).Render(fullListRender)
}

func (m Model) renderPreviewPane(paneWidth, paneHeight int) string {
	var finalContentToRender string // This will be the content INSIDE the box (headers + body)
	var titleText string

	if paneWidth <= 0 || paneHeight <= 0 {
		return "" // No space to render anything
	}

	// Prepare the title first, as its height is fixed
	styledTitle := TitleStyle.Render("Placeholder") // Placeholder to get title style height

	if len(m.allEmails) == 0 || m.selectedIdx < 0 || m.selectedIdx >= len(m.allEmails) {
		titleText = "Home"
		welcomeMsg := "\n[tmail]\n\nNo email selected or list is empty."
		// The welcome message also needs to be constrained.
		// MaxHeight for the content area: paneHeight - title height - box padding
		maxContentHeight := paneHeight - lipgloss.Height(styledTitle) - ContentBoxStyle.GetVerticalPadding()
		if maxContentHeight < 0 {
			maxContentHeight = 0
		}
		finalContentToRender = lipgloss.NewStyle().
			Width(paneWidth - ContentBoxStyle.GetHorizontalPadding()). // Width for text inside box
			MaxHeight(maxContentHeight).                               // Constrain height
			Padding(1).Render(welcomeMsg)
	} else {
		email := m.allEmails[m.selectedIdx]
		titleText = fmt.Sprintf("Preview: %s", truncate(email.Subject, paneWidth-(TitleStyle.GetHorizontalPadding()+12)))

		// Render headers to know their exact height
		var headerBuilder strings.Builder
		headerBuilder.WriteString(fmt.Sprintf("%s %s\n", HeaderKeyStyle.Render("From:"), HeaderValStyle.Render(truncate(email.From, paneWidth-10))))
		dateStr := "N/A"
		if !email.Date.IsZero() {
			dateStr = email.Date.Local().Format(time.RFC1123)
		}
		headerBuilder.WriteString(fmt.Sprintf("%s %s\n", HeaderKeyStyle.Render("Date:"), HeaderValStyle.Render(dateStr)))
		headerBuilder.WriteString(fmt.Sprintf("%s %s\n", HeaderKeyStyle.Render("Subject:"), HeaderValStyle.Render(truncate(email.Subject, paneWidth-12))))
		headerBuilder.WriteString("\n" + strings.Repeat("─", paneWidth/2)) // Separator, its height is 1

		renderedHeaders := headerBuilder.String()
		renderedHeaderHeight := lipgloss.Height(renderedHeaders) // Get actual height of rendered headers + separator

		// Now calculate available height for the body
		bodyDisplayHeight := m.getVisiblePreviewBodyHeight(paneHeight, renderedHeaderHeight)

		bodyLines := strings.Split(strings.ReplaceAll(email.Body, "\r\n", "\n"), "\n")
		startLine := m.previewScrollPos
		if startLine < 0 {
			startLine = 0
		}
		// Adjust startLine if it's scrolled too far down for the available bodyDisplayHeight
		if len(bodyLines) > bodyDisplayHeight && startLine > len(bodyLines)-bodyDisplayHeight && bodyDisplayHeight > 0 {
			startLine = len(bodyLines) - bodyDisplayHeight
		} else if startLine >= len(bodyLines) && len(bodyLines) > 0 { // Scrolled past everything
			startLine = len(bodyLines) - 1
		}
		if len(bodyLines) == 0 {
			startLine = 0
		}

		endLine := startLine + bodyDisplayHeight
		if endLine > len(bodyLines) {
			endLine = len(bodyLines)
		}

		visibleBody := ""
		if startLine < endLine && startLine < len(bodyLines) {
			visibleBody = strings.Join(bodyLines[startLine:endLine], "\n")
		}

		// Combine rendered headers and the calculated visible body
		// The BodyStyle might add margins, consider that if heights are still off.
		finalContentToRender = lipgloss.JoinVertical(lipgloss.Left,
			renderedHeaders, // Already has a newline from separator
			BodyStyle.Render(visibleBody),
		)
		// Style for the content block *inside* the ContentBoxStyle borders
		finalContentToRender = lipgloss.NewStyle().
			Width(paneWidth - ContentBoxStyle.GetHorizontalPadding()).                                   // Content width inside the box
			MaxHeight(paneHeight - lipgloss.Height(styledTitle) - ContentBoxStyle.GetVerticalPadding()). // Max height for content
			Render(finalContentToRender)
	}

	// Update the actual title text on the pre-styled title
	styledTitle = TitleStyle.Render(titleText)

	// The ContentBoxStyle provides the outer border and is constrained by paneWidth, paneHeight.
	// It joins the styledTitle and finalContentToRender.
	return ContentBoxStyle.Width(paneWidth).Height(paneHeight).Render(
		lipgloss.JoinVertical(lipgloss.Top, styledTitle, finalContentToRender),
	)
}

func (m Model) renderFocusedEmailView(paneWidth, paneHeight int) string {
	var finalContentToRender string
	var titleText string

	if paneWidth <= 0 || paneHeight <= 0 {
		return ""
	}

	styledTitle := TitleStyle.Render("Placeholder") // For height calculation

	if len(m.allEmails) == 0 || m.selectedIdx < 0 || m.selectedIdx >= len(m.allEmails) {
		titleText = "Error"
		maxContentHeight := paneHeight - lipgloss.Height(styledTitle) - ContentBoxStyle.GetVerticalPadding()
		if maxContentHeight < 0 {
			maxContentHeight = 0
		}
		finalContentToRender = lipgloss.NewStyle().
			Width(paneWidth - ContentBoxStyle.GetHorizontalPadding()).
			MaxHeight(maxContentHeight).
			Padding(1).Render("No email selected.")
	} else {
		email := m.allEmails[m.selectedIdx]
		titleText = fmt.Sprintf("Full View: %s", truncate(email.Subject, paneWidth-(TitleStyle.GetHorizontalPadding()+15)))

		var contentBuilder strings.Builder
		contentBuilder.WriteString(fmt.Sprintf("%s %s\n", HeaderKeyStyle.Render("From:"), HeaderValStyle.Render(email.From)))
		contentBuilder.WriteString(fmt.Sprintf("%s %s\n", HeaderKeyStyle.Render("To:"), HeaderValStyle.Render(email.To)))
		if email.Cc != "" {
			contentBuilder.WriteString(fmt.Sprintf("%s %s\n", HeaderKeyStyle.Render("Cc:"), HeaderValStyle.Render(email.Cc)))
		}
		dateStr := "N/A"
		if !email.Date.IsZero() {
			dateStr = email.Date.Local().Format(time.RFC1123Z)
		}
		contentBuilder.WriteString(fmt.Sprintf("%s %s\n", HeaderKeyStyle.Render("Date:"), HeaderValStyle.Render(dateStr)))
		contentBuilder.WriteString(fmt.Sprintf("%s %s\n\n", HeaderKeyStyle.Render("Subject:"), HeaderValStyle.Render(email.Subject)))
		contentBuilder.WriteString(strings.Repeat("─", paneWidth/2) + "\n\n")

		fullBody := strings.ReplaceAll(email.Body, "\r\n", "\n")
		contentBuilder.WriteString(BodyStyle.Render(fullBody)) // BodyStyle might add margins

		maxContentHeight := paneHeight - lipgloss.Height(styledTitle) - ContentBoxStyle.GetVerticalPadding()
		if maxContentHeight < 0 {
			maxContentHeight = 0
		}
		finalContentToRender = lipgloss.NewStyle().
			Width(paneWidth - ContentBoxStyle.GetHorizontalPadding()).
			MaxHeight(maxContentHeight). // Important for very long emails
			// Padding(0,1) was here, but ContentBoxStyle already has padding(0,1)
			Render(contentBuilder.String())
	}

	styledTitle = TitleStyle.Render(titleText)
	return ContentBoxStyle.Width(paneWidth).Height(paneHeight).Render(
		lipgloss.JoinVertical(lipgloss.Top, styledTitle, finalContentToRender),
	)
}

func (m Model) renderStatusBar() string {
	styleToUse := StatusBarNormalStyle
	if m.statusIsError {
		styleToUse = StatusBarErrorStyle
	} else if m.statusIsTemp {
		styleToUse = StatusBarSuccessStyle
	}
	return styleToUse.Width(m.width).Render(truncate(m.statusBarText, m.width))
}
