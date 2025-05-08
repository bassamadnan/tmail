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
	emailListItemHeight = 4 // Each item in the list takes 4 lines
	minListPaneWidth    = 30
	minPreviewPaneWidth = 40
)

type Model struct {
	configManager   *config.Manager
	emailChan       <-chan gmail.ProcessedEmail
	apiPollInterval time.Duration

	allEmails             []gmail.ProcessedEmail
	selectedIdx           int
	viewportTopLine       int // For scrolling the email list view
	previewScrollPos      int // For scrolling the preview pane content
	focusedEmailScrollPos int // For scrolling the focused email view content

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
		configManager:         cfgManager,
		emailChan:             emailChan,
		apiPollInterval:       pollInterval,
		currentView:           viewLoading,
		statusBarText:         "Initializing, connecting to Gmail...",
		allEmails:             []gmail.ProcessedEmail{},
		selectedIdx:           0,
		viewportTopLine:       0,
		previewScrollPos:      0,
		focusedEmailScrollPos: 0,
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
	}
	return numFit
}

func (m Model) getVisiblePreviewBodyHeight(paneTotalHeight int, renderedHeaderHeight int) int {
	previewTitleHeight := lipgloss.Height(TitleStyle.Render(" "))
	availableHeight := paneTotalHeight - previewTitleHeight - renderedHeaderHeight - ContentBoxStyle.GetVerticalPadding()
	if availableHeight < 0 {
		availableHeight = 0
	}
	return availableHeight
}

// getFocusedViewContentRenderHeight estimates available lines for the scrollable content in focused view
func (m Model) getFocusedViewContentRenderHeight(paneTotalHeight int) int {
	// Assuming similar title and ContentBoxStyle padding as preview
	titleHeight := lipgloss.Height(TitleStyle.Render(" "))
	// Focused view might not have an explicit footer like the old tview Frame,
	// so we just subtract title and box padding from the total pane height.
	availableHeight := paneTotalHeight - titleHeight - ContentBoxStyle.GetVerticalPadding()
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

	case tea.MouseMsg:
		// --- MOUSE EVENT HANDLING ---
		listPaneBoundaryX := int(float64(m.width) * 0.35) // Simplified boundary
		if listPaneBoundaryX < minListPaneWidth {
			listPaneBoundaryX = minListPaneWidth
		}
		if listPaneBoundaryX > m.width-minPreviewPaneWidth && m.width > minPreviewPaneWidth {
			listPaneBoundaryX = m.width - minPreviewPaneWidth
		}
		if listPaneBoundaryX < 0 {
			listPaneBoundaryX = 0
		}

		switch msg.Type {
		case tea.MouseWheelUp:
			if m.currentView == viewDashboard {
				if msg.X < listPaneBoundaryX { // Over email list
					if m.viewportTopLine > 0 {
						m.viewportTopLine--
					}
				} else { // Over preview pane
					if m.previewScrollPos > 0 {
						m.previewScrollPos--
					}
				}
			} else if m.currentView == viewFocusedEmail {
				if m.focusedEmailScrollPos > 0 {
					m.focusedEmailScrollPos--
				}
			}
			return m, nil

		case tea.MouseWheelDown:
			if m.currentView == viewDashboard {
				if msg.X < listPaneBoundaryX { // Over email list
					itemsThatFit := m.getNumItemsThatFitInList()
					if len(m.allEmails) > itemsThatFit && m.viewportTopLine < len(m.allEmails)-itemsThatFit {
						m.viewportTopLine++
					}
				} else { // Over preview pane
					// Simplified boundary for preview scroll down
					if len(m.allEmails) > 0 && m.selectedIdx >= 0 && m.selectedIdx < len(m.allEmails) {
						emailContent := m.allEmails[m.selectedIdx].Body // Just an example, need full content lines
						bodyLines := strings.Split(strings.ReplaceAll(emailContent, "\r\n", "\n"), "\n")
						if m.previewScrollPos < len(bodyLines)-1 {
							m.previewScrollPos++
						}
					}
				}
			} else if m.currentView == viewFocusedEmail {
				// Simplified boundary for focused scroll down
				// A more robust check considers the number of lines the content actually renders to.
				m.focusedEmailScrollPos++
			}
			return m, nil

		case tea.MouseLeft: // CLICK TO SELECT
			if m.currentView == viewDashboard && msg.X < listPaneBoundaryX { // Click is in the list pane
				// Calculate which item was clicked. msg.Y is the row, 0-indexed from top of screen.
				// We need Y relative to the start of the list items area.
				listTitleRenderedHeight := lipgloss.Height(EmailListTitleStyle.Render(" "))
				listStartY := listTitleRenderedHeight // Y where email items start (after status bar and title)

				clickedItemIndex := (msg.Y - listStartY) / emailListItemHeight
				actualClickedIdx := m.viewportTopLine + clickedItemIndex

				if actualClickedIdx >= 0 && actualClickedIdx < len(m.allEmails) {
					if m.selectedIdx != actualClickedIdx { // Only update if selection changes
						m.selectedIdx = actualClickedIdx
						m.previewScrollPos = 0      // Reset preview scroll
						m.focusedEmailScrollPos = 0 // Reset focused scroll (good practice)
						m.ensureSelectedVisible()   // Should not be strictly needed if already visible, but good for consistency
						m.setStandardStatus()       // Update status if needed
					}
				}
			}
			return m, nil
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
					m.focusedEmailScrollPos = 0 // Reset focused view scroll too
				}
			case "down", "j":
				if m.selectedIdx < len(m.allEmails)-1 {
					m.selectedIdx++
					m.ensureSelectedVisible()
					m.previewScrollPos = 0
					m.focusedEmailScrollPos = 0 // Reset focused view scroll too
				}
			case "enter":
				if len(m.allEmails) > 0 && m.selectedIdx >= 0 && m.selectedIdx < len(m.allEmails) {
					m.currentView = viewFocusedEmail
					m.focusedEmailScrollPos = 0 // Reset scroll when entering focused view
					m.setStandardStatus()
				}
			case "K":
				if m.previewScrollPos > 0 {
					m.previewScrollPos--
				}
			case "J":
				if len(m.allEmails) > 0 && m.selectedIdx >= 0 && m.selectedIdx < len(m.allEmails) {
					email := m.allEmails[m.selectedIdx]
					bodyLines := strings.Split(strings.ReplaceAll(email.Body, "\r\n", "\n"), "\n")
					if m.previewScrollPos < len(bodyLines)-1 {
						m.previewScrollPos++
					}
				}
			}
		case viewFocusedEmail:
			// ADDED: Key-based scrolling for focused view
			switch msg.String() {
			case "ctrl+c", "q":
				m.updateStatusBar("Quitting...")
				return m, tea.Quit
			case "esc":
				m.currentView = viewDashboard
				m.setStandardStatus()
			case "up", "k": // Scroll focused view up
				if m.focusedEmailScrollPos > 0 {
					m.focusedEmailScrollPos--
				}
			case "down", "j": // Scroll focused view down
				// Simplified boundary, similar to mouse wheel
				m.focusedEmailScrollPos++
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

// ... (showTemporaryStatus, updateStatusBar, updateStatusError, setStandardStatus, ensureSelectedVisible, View, renderEmailList, renderPreviewPane methods remain THE SAME) ...
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
		keyHints += " | [↑↓/jk]:Nav | [Enter]:Full | [KJ]:Scroll Preview | [MouseWheel/Click]:Interact"
	case viewFocusedEmail:
		keyHints += " | [Esc]:Back | [↑↓/jk/MouseWheel]:Scroll"
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
		if actualListPaneWidth > m.width-minPreviewPaneWidth && m.width > minPreviewPaneWidth {
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

		if m.width < minListPaneWidth+minPreviewPaneWidth {
			if m.width < minListPaneWidth {
				actualListPaneWidth = m.width
				actualPreviewPaneWidth = 0
			} else {
				actualListPaneWidth = minListPaneWidth
				actualPreviewPaneWidth = m.width - actualListPaneWidth
			}
		}

		emailListRendered := m.renderEmailList(actualListPaneWidth, contentHeight)
		previewPaneRendered := m.renderPreviewPane(actualPreviewPaneWidth, contentHeight)

		mainUIView = lipgloss.JoinHorizontal(lipgloss.Top, emailListRendered, previewPaneRendered)

	case viewFocusedEmail:
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
	if paneWidth > 0 && paneHeight > 0 && len(m.allEmails) > 0 {
		for i := startIdx; i < endIdx; i++ {
			if i >= 0 && i < len(m.allEmails) {
				email := m.allEmails[i]
				isSelected := (i == m.selectedIdx)
				itemStr := formatEmailListItem(email, isSelected, itemTextContentWidth)
				visibleEmailItemStrings = append(visibleEmailItemStrings, itemStr)
			}
		}
	}
	listItemsContent.WriteString(strings.Join(visibleEmailItemStrings, "\n"))

	fullListRender := lipgloss.JoinVertical(lipgloss.Left, title, listItemsContent.String())
	return EmailListStyle.Width(paneWidth).Height(paneHeight).Render(fullListRender)
}

func (m Model) renderPreviewPane(paneWidth, paneHeight int) string {
	var finalContentToRender string
	var titleText string

	if paneWidth <= 0 || paneHeight <= 0 {
		return ""
	}

	styledTitle := TitleStyle.Render("Placeholder")

	if len(m.allEmails) == 0 || m.selectedIdx < 0 || m.selectedIdx >= len(m.allEmails) {
		titleText = "Home"
		welcomeMsg := "\n[tmail]\n\nNo email selected or list is empty."
		maxContentHeight := paneHeight - lipgloss.Height(styledTitle) - ContentBoxStyle.GetVerticalPadding()
		if maxContentHeight < 0 {
			maxContentHeight = 0
		}
		finalContentToRender = lipgloss.NewStyle().
			Width(paneWidth - ContentBoxStyle.GetHorizontalPadding()).
			MaxHeight(maxContentHeight).
			Padding(1).Render(welcomeMsg)
	} else {
		email := m.allEmails[m.selectedIdx]
		titleText = fmt.Sprintf("Preview: %s", truncate(email.Subject, paneWidth-(TitleStyle.GetHorizontalPadding()+12)))

		var headerBuilder strings.Builder
		headerBuilder.WriteString(fmt.Sprintf("%s %s\n", HeaderKeyStyle.Render("From:"), HeaderValStyle.Render(truncate(email.From, paneWidth-10))))
		dateStr := "N/A"
		if !email.Date.IsZero() {
			dateStr = email.Date.Local().Format(time.RFC1123)
		}
		headerBuilder.WriteString(fmt.Sprintf("%s %s\n", HeaderKeyStyle.Render("Date:"), HeaderValStyle.Render(dateStr)))
		headerBuilder.WriteString(fmt.Sprintf("%s %s\n", HeaderKeyStyle.Render("Subject:"), HeaderValStyle.Render(truncate(email.Subject, paneWidth-12))))
		headerBuilder.WriteString("\n" + strings.Repeat("─", paneWidth/2))

		renderedHeaders := headerBuilder.String()
		renderedHeaderHeight := lipgloss.Height(renderedHeaders)

		bodyDisplayHeight := m.getVisiblePreviewBodyHeight(paneHeight, renderedHeaderHeight)

		bodyLines := strings.Split(strings.ReplaceAll(email.Body, "\r\n", "\n"), "\n")
		startLine := m.previewScrollPos
		if startLine < 0 {
			startLine = 0
		}
		if len(bodyLines) > bodyDisplayHeight && startLine > len(bodyLines)-bodyDisplayHeight && bodyDisplayHeight > 0 {
			startLine = len(bodyLines) - bodyDisplayHeight
		} else if startLine >= len(bodyLines) && len(bodyLines) > 0 {
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

		finalContentToRender = lipgloss.JoinVertical(lipgloss.Left,
			renderedHeaders,
			BodyStyle.Render(visibleBody),
		)
		finalContentToRender = lipgloss.NewStyle().
			Width(paneWidth - ContentBoxStyle.GetHorizontalPadding()).
			MaxHeight(paneHeight - lipgloss.Height(styledTitle) - ContentBoxStyle.GetVerticalPadding()).
			Render(finalContentToRender)
	}

	styledTitle = TitleStyle.Render(titleText)
	return ContentBoxStyle.Width(paneWidth).Height(paneHeight).Render(
		lipgloss.JoinVertical(lipgloss.Top, styledTitle, finalContentToRender),
	)
}

func (m Model) renderFocusedEmailView(paneWidth, paneHeight int) string {
	var finalContent string // This will be the scrollable content part
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
		finalContent = lipgloss.NewStyle().
			Width(paneWidth - ContentBoxStyle.GetHorizontalPadding()).
			MaxHeight(maxContentHeight).
			Padding(1).Render("No email selected.")
	} else {
		email := m.allEmails[m.selectedIdx]
		titleText = fmt.Sprintf("Full View: %s", truncate(email.Subject, paneWidth-(TitleStyle.GetHorizontalPadding()+15)))

		// Build the full content string that will be scrolled
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
		fullBodyText := strings.ReplaceAll(email.Body, "\r\n", "\n")
		contentBuilder.WriteString(BodyStyle.Render(fullBodyText)) // Render with BodyStyle for consistent look

		fullContentString := contentBuilder.String()
		fullContentLines := strings.Split(fullContentString, "\n")

		// Calculate how many lines of this content can be displayed
		displayHeight := m.getFocusedViewContentRenderHeight(paneHeight)

		startLine := m.focusedEmailScrollPos
		if startLine < 0 {
			startLine = 0
		}
		// Adjust startLine if it's scrolled too far down
		if len(fullContentLines) > displayHeight && startLine > len(fullContentLines)-displayHeight && displayHeight > 0 {
			startLine = len(fullContentLines) - displayHeight
		} else if startLine >= len(fullContentLines) && len(fullContentLines) > 0 {
			startLine = len(fullContentLines) - 1
		}
		if len(fullContentLines) == 0 {
			startLine = 0
		}

		endLine := startLine + displayHeight
		if endLine > len(fullContentLines) {
			endLine = len(fullContentLines)
		}

		visibleContent := ""
		if startLine < endLine && startLine < len(fullContentLines) {
			visibleContent = strings.Join(fullContentLines[startLine:endLine], "\n")
		}

		// The final content to be rendered inside the box (after the title)
		finalContent = lipgloss.NewStyle().
			Width(paneWidth - ContentBoxStyle.GetHorizontalPadding()). // Constrain width
			// MaxHeight is implicitly handled by slicing the lines
			Render(visibleContent)
	}

	styledTitle = TitleStyle.Render(titleText) // Update actual title text
	// The ContentBoxStyle frames the title and the finalContent (scrolled portion)
	return ContentBoxStyle.Width(paneWidth).Height(paneHeight).Render(
		lipgloss.JoinVertical(lipgloss.Top, styledTitle, finalContent),
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
