package views

import (
	"time"

	"github.com/Vector-DNS/vdns-tui/internal/config"
	"github.com/Vector-DNS/vdns-tui/internal/history"
	"github.com/Vector-DNS/vdns-tui/internal/theme"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

// saveToHistory appends a lookup entry to local history if enabled.
func saveToHistory(cfg *config.Config, command, domain, recordType, mode string, results any) {
	if cfg == nil || !cfg.Local.SaveHistory {
		return
	}
	entry := history.Entry{
		Timestamp:  time.Now(),
		Command:    command,
		Domain:     domain,
		RecordType: recordType,
		Mode:       mode,
		Results:    results,
	}
	_ = history.Append(cfg, entry)
}

// newSpinner creates a spinner with the standard dot style and AccentCyan color.
func newSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.ColorAccentCyan)
	return s
}

// maxInt returns the larger of two ints.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// truncate shortens s to max characters, appending "..." if truncated.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max < 4 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

// renderCentered places content centered within the given dimensions.
func renderCentered(content string, width, height int) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

// renderLoadingSpinner renders a centered loading message with a spinner.
func renderLoadingSpinner(spinnerView, message string, width, height int) string {
	style := lipgloss.NewStyle().Foreground(theme.ColorAccentCyan)
	content := style.Render(spinnerView + "  " + message)
	return renderCentered(content, width, height)
}

// renderErrorBox renders a centered error box with rounded border.
func renderErrorBox(err error, width, height int) string {
	errStyle := lipgloss.NewStyle().Foreground(theme.ColorRose).Bold(true)
	msgStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray)

	content := lipgloss.JoinVertical(lipgloss.Center,
		errStyle.Render("Error"),
		"",
		msgStyle.Render(err.Error()),
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorRose).
		Padding(1, 3).
		Render(content)

	return renderCentered(box, width, height)
}

// renderEmptyState renders a centered placeholder message.
func renderEmptyState(message string, width, height int) string {
	style := lipgloss.NewStyle().
		Foreground(theme.ColorSlate600).
		Italic(true).
		Align(lipgloss.Center)
	return renderCentered(style.Render(message), width, height)
}
