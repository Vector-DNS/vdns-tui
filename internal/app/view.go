package app

import (
	"fmt"
	"strings"

	"github.com/Vector-DNS/vdns-tui/internal/theme"
	"github.com/charmbracelet/lipgloss"
)

// View renders the full TUI layout.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	header := m.renderHeader()
	inputBar := m.renderInputBar()
	statusBar := m.renderStatusBar()

	// Height available for sidebar + content.
	headerHeight := lipgloss.Height(header)
	inputHeight := lipgloss.Height(inputBar)
	statusHeight := lipgloss.Height(statusBar)
	bodyHeight := m.height - headerHeight - inputHeight - statusHeight
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	var body string
	if m.sidebarOpen {
		sidebar := m.renderSidebar(bodyHeight)
		contentWidth := m.width - lipgloss.Width(sidebar)
		if contentWidth < 1 {
			contentWidth = 1
		}
		content := m.renderContent(contentWidth, bodyHeight)
		body = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)
	} else {
		body = m.renderContent(m.width, bodyHeight)
	}

	screen := lipgloss.JoinVertical(lipgloss.Left, header, body, inputBar, statusBar)

	// Overlay help modal if active.
	if m.showHelp {
		overlay := m.renderHelpOverlay()
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
	}

	return screen
}

// renderHeader draws a clean top bar with logo and mode indicator.
func (m Model) renderHeader() string {
	logo := LogoStyle.Render("◆ VectorDNS")

	var mode string
	if m.config.ShouldUseRemote() {
		mode = ModeRemoteStyle.Render("REMOTE")
	} else {
		mode = ModeLocalStyle.Render("LOCAL")
	}

	// Rate limit indicator (remote mode only).
	var rateInfo string
	if m.apiClient != nil && m.apiClient.LastRateLimit != nil {
		rl := m.apiClient.LastRateLimit
		rateInfo = " " + RateLimitStyle.Render(fmt.Sprintf("%d/%d", rl.Remaining, rl.Limit))
	}

	right := DividerStyle.Render("[") + mode + DividerStyle.Render("]") + rateInfo

	gap := m.width - lipgloss.Width(logo) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}

	bar := logo + strings.Repeat(" ", gap) + right
	return HeaderStyle.Width(m.width).Render(bar)
}

// renderInputBar draws the domain input bar between content and status bar.
func (m Model) renderInputBar() string {
	var prompt string
	if m.inputFocused {
		prompt = GlowStyle.Render("▸ ")
	} else {
		prompt = DividerStyle.Render("  ")
	}

	input := m.domainInput.View()
	left := " " + prompt + input

	gap := m.width - lipgloss.Width(left)
	if gap < 0 {
		gap = 0
	}

	bar := left + strings.Repeat(" ", gap)
	return InputBarStyle.Width(m.width).Render(bar)
}

// renderSidebar draws the left navigation panel.
func (m Model) renderSidebar(height int) string {
	var items []string

	for i := 0; i < viewCount; i++ {
		icon := viewIcons[i]
		label := fmt.Sprintf(" %s %s", icon, viewNames[i])

		if View(i) == m.activeView {
			items = append(items, SidebarActiveStyle.Render(label))
		} else {
			items = append(items, SidebarItemStyle.Render(label))
		}
	}

	content := strings.Join(items, "\n")
	return SidebarStyle.Height(height).Render(content)
}

// renderContent draws the main content area by delegating to the active view.
func (m Model) renderContent(width, height int) string {
	comp := m.activeComponent()

	var inner string
	if comp != nil {
		cw := width - 4 // ContentStyle padding: 2 left + 2 right
		ch := height - 2 // ContentStyle padding: 1 top + 1 bottom
		if cw < 1 {
			cw = 1
		}
		if ch < 1 {
			ch = 1
		}
		inner = comp.View(cw, ch)
	} else {
		inner = m.renderPlaceholder(width-4, height-2)
	}

	return ContentStyle.Width(width).Height(height).Render(inner)
}

// renderPlaceholder shows a styled placeholder for views not yet implemented.
func (m Model) renderPlaceholder(width, height int) string {
	name := viewNames[int(m.activeView)]
	icon := viewIcons[int(m.activeView)]

	title := TitleStyle.Render(fmt.Sprintf("%s  %s", icon, name))
	sub := SubtitleStyle.Render("coming soon")

	box := BoxStyle.Width(min(40, width)).Render(
		lipgloss.JoinVertical(lipgloss.Center, title, "", sub),
	)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

// renderStatusBar draws the bottom bar with status text and help hints.
func (m Model) renderStatusBar() string {
	var left string

	if m.errorText != "" {
		left = ErrorStyle.Render(m.errorText)
	} else if m.statusText != "" {
		left = m.statusText
	}

	var helpText string
	if m.inputFocused {
		helpText = "Enter submit  Esc cancel  Up/Down navigate"
	} else if m.contentFocus {
		helpText = "/ search  Esc back  Up/Down navigate  q quit"
	} else {
		helpText = "/ search  Up/Down navigate  Enter select  q quit"
	}
	help := DividerStyle.Render(helpText)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(help)
	if gap < 1 {
		gap = 1
	}

	bar := left + strings.Repeat(" ", gap) + help
	return StatusBarStyle.Width(m.width).Render(bar)
}

// renderHelpOverlay creates the centered help modal.
func (m Model) renderHelpOverlay() string {
	titleStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimaryBlue).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true).Width(14)
	descStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite)
	dimStyle := lipgloss.NewStyle().Foreground(theme.ColorSlate600)

	title := titleStyle.Render("Keyboard Shortcuts")

	entries := []struct{ key, desc string }{
		{"/", "Focus domain input"},
		{"Enter", "Submit domain"},
		{"Esc", "Back / Cancel"},
		{"Up/Down", "Navigate views"},
		{"Tab", "Next view"},
		{"1-9", "Jump to view"},
		{"?", "Toggle this help"},
		{"q", "Quit"},
	}

	var lines []string
	for _, e := range entries {
		lines = append(lines, keyStyle.Render(e.key)+descStyle.Render(e.desc))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	inner := lipgloss.JoinVertical(lipgloss.Center,
		title, "", content, "",
		dimStyle.Render("Press any key to close"),
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorPrimaryBlue).
		Background(theme.ColorMidnight).
		Padding(1, 3).
		Render(inner)

	return box
}
