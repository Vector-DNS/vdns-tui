package views

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Vector-DNS/vdns-tui/internal/config"
	"github.com/Vector-DNS/vdns-tui/internal/history"
	"github.com/Vector-DNS/vdns-tui/internal/theme"
)

// HistoryResultMsg carries loaded history entries back to the view.
type HistoryResultMsg struct {
	Entries []history.Entry
	Err     error
}

// historyClearedMsg signals that history was successfully cleared.
type historyClearedMsg struct{ Err error }

// commandColor maps history command names to brand colors.
var commandColor = map[string]lipgloss.Color{
	"lookup":       theme.ColorPrimaryBlue,
	"whois":        theme.ColorViolet,
	"ssl":          theme.ColorEmerald,
	"propagation":  theme.ColorAccentCyan,
	"availability": theme.ColorAmber,
	"benchmark":    theme.ColorRose,
}

// HistoryView displays lookup history in a browsable list.
type HistoryView struct {
	width, height int
	loading       bool
	spinner       spinner.Model
	entries       []history.Entry // displayed entries (newest first)
	cursor        int             // selected index
	scrollOffset  int             // first visible row
	err           error
	config        *config.Config
	searchMode    bool
	searchDomain  string
	confirmClear  bool
	loaded        bool
	showDetail    bool           // true when viewing a single entry
	detailEntry   *history.Entry // the entry being viewed
	detailScroll  int
}

// NewHistoryView creates a new history browser view.
func NewHistoryView(cfg *config.Config) *HistoryView {
	s := newSpinner()

	return &HistoryView{
		config:  cfg,
		spinner: s,
	}
}

// Update handles messages for the history view.
func (v *HistoryView) Update(msg tea.Msg) (Component, tea.Cmd) {
	switch msg := msg.(type) {

	case DomainSubmitMsg:
		// Search history for the submitted domain.
		v.searchMode = true
		v.searchDomain = msg.Domain
		v.loading = true
		v.err = nil
		v.confirmClear = false
		cfg := v.config
		domain := msg.Domain
		return v, tea.Batch(v.spinner.Tick, func() tea.Msg {
			entries, err := history.Search(cfg, domain)
			return HistoryResultMsg{Entries: entries, Err: err}
		})

	case HistoryResultMsg:
		v.loading = false
		v.loaded = true
		if msg.Err != nil {
			v.err = msg.Err
			return v, nil
		}
		v.entries = reverseEntries(msg.Entries)
		v.cursor = 0
		v.scrollOffset = 0
		return v, nil

	case historyClearedMsg:
		v.confirmClear = false
		if msg.Err != nil {
			v.err = msg.Err
			return v, nil
		}
		v.entries = nil
		v.cursor = 0
		v.scrollOffset = 0
		return v, nil

	case tea.KeyMsg:
		// Detail view navigation
		if v.showDetail {
			switch msg.String() {
			case "esc", "backspace", "q":
				v.showDetail = false
				v.detailEntry = nil
				v.detailScroll = 0
			case "up", "k":
				if v.detailScroll > 0 {
					v.detailScroll--
				}
			case "down", "j":
				v.detailScroll++
			}
			return v, nil
		}

		switch msg.String() {
		case "enter":
			if v.cursor >= 0 && v.cursor < len(v.entries) {
				e := v.entries[v.cursor]
				v.detailEntry = &e
				v.showDetail = true
				v.detailScroll = 0
			}
			return v, nil

		case "c":
			if v.confirmClear {
				v.confirmClear = false
				cfg := v.config
				return v, func() tea.Msg {
					err := history.Clear(cfg)
					return historyClearedMsg{Err: err}
				}
			}
			v.confirmClear = true
			return v, nil

		case "r":
			v.searchMode = false
			v.searchDomain = ""
			v.confirmClear = false
			return v, v.loadHistory()

		case "up", "k":
			v.confirmClear = false
			if v.cursor > 0 {
				v.cursor--
				v.ensureVisible()
			}
			return v, nil

		case "down", "j":
			v.confirmClear = false
			if v.cursor < len(v.entries)-1 {
				v.cursor++
				v.ensureVisible()
			}
			return v, nil

		case "home", "g":
			v.confirmClear = false
			v.cursor = 0
			v.scrollOffset = 0
			return v, nil

		case "end", "G":
			v.confirmClear = false
			if len(v.entries) > 0 {
				v.cursor = len(v.entries) - 1
				v.ensureVisible()
			}
			return v, nil

		default:
			v.confirmClear = false
		}

	case spinner.TickMsg:
		if v.loading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return v, cmd
		}
	}

	// On first render, load history if we haven't yet.
	if !v.loaded && !v.loading {
		return v, v.loadHistory()
	}

	return v, nil
}

// ensureVisible adjusts scrollOffset so the cursor is within the visible area.
func (v *HistoryView) ensureVisible() {
	visible := v.visibleRows()
	if visible < 1 {
		visible = 1
	}
	if v.cursor < v.scrollOffset {
		v.scrollOffset = v.cursor
	}
	if v.cursor >= v.scrollOffset+visible {
		v.scrollOffset = v.cursor - visible + 1
	}
}

// visibleRows returns how many entry rows fit in the current view height,
// accounting for the header/status lines.
func (v *HistoryView) visibleRows() int {
	// Title (1) + column header (1) + separator (1) + bottom status (1) = 4 lines of chrome
	rows := v.height - 4
	if v.searchMode || v.confirmClear {
		rows-- // status line takes one more row
	}
	if rows < 1 {
		rows = 1
	}
	return rows
}

// loadHistory fires a command to load the latest history entries.
func (v *HistoryView) loadHistory() tea.Cmd {
	v.loading = true
	v.err = nil
	cfg := v.config
	return tea.Batch(v.spinner.Tick, func() tea.Msg {
		entries, err := history.List(cfg, 50)
		return HistoryResultMsg{Entries: entries, Err: err}
	})
}

// View renders the history view.
func (v *HistoryView) View(width, height int) string {
	v.width = width
	v.height = height

	if v.showDetail && v.detailEntry != nil {
		return v.renderDetail()
	}

	if v.loading {
		return renderLoadingSpinner(v.spinner.View(), "Loading history...", width, height)
	}

	if v.err != nil {
		return renderErrorBox(v.err, width, height)
	}

	if !v.loaded {
		return renderLoadingSpinner(v.spinner.View(), "Loading history...", width, height)
	}

	// Styles
	titleStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimaryBlue).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(theme.ColorSlate600)
	headerStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray).Bold(true)
	selectedStyle := lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true)
	domainStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite)
	timeStyle := lipgloss.NewStyle().Foreground(theme.ColorSlate600)
	countStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray)

	var lines []string

	// Title
	title := titleStyle.Render("Lookup History")
	count := countStyle.Render(fmt.Sprintf("(%d entries)", len(v.entries)))
	lines = append(lines, title+"  "+count)

	// Status line (search filter / clear confirmation)
	if v.searchMode {
		searchBadge := lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true).
			Render("Filtering: " + v.searchDomain)
		resetHint := dimStyle.Render("  [r] clear filter")
		lines = append(lines, searchBadge+resetHint)
	}
	if v.confirmClear {
		clearHint := lipgloss.NewStyle().Foreground(theme.ColorRose).Bold(true).
			Render("Press c again to clear all history")
		lines = append(lines, clearHint)
	}

	// Empty state
	if len(v.entries) == 0 {
		emptyMsg := dimStyle.Render("No history entries yet. Run some lookups first.")
		if v.searchMode {
			emptyMsg = dimStyle.Render("No entries found for " + v.searchDomain)
		}
		lines = append(lines, "", emptyMsg)
		content := lipgloss.JoinVertical(lipgloss.Left, lines...)
		return content
	}

	// Column header
	domainCol := maxInt(20, width/3)
	cmdCol := 14
	typeCol := 8
	timeCol := maxInt(12, width-domainCol-cmdCol-typeCol-6)

	hdrDomain := headerStyle.Width(domainCol).Render("DOMAIN")
	hdrCmd := headerStyle.Width(cmdCol).Render("COMMAND")
	hdrType := headerStyle.Width(typeCol).Render("TYPE")
	hdrTime := headerStyle.Width(timeCol).Render("TIME")
	lines = append(lines, hdrDomain+hdrCmd+hdrType+hdrTime)

	// Separator
	sep := dimStyle.Render(repeatChar('-', maxInt(width, 1)))
	lines = append(lines, sep)

	// Visible entries
	visible := v.visibleRows()
	end := v.scrollOffset + visible
	if end > len(v.entries) {
		end = len(v.entries)
	}

	for i := v.scrollOffset; i < end; i++ {
		e := v.entries[i]

		// Command with color
		cmdColor, ok := commandColor[e.Command]
		if !ok {
			cmdColor = theme.ColorCoolGray
		}
		cmdStyled := lipgloss.NewStyle().Foreground(cmdColor).Width(cmdCol).Render(e.Command)

		// Record type
		recType := e.RecordType
		if recType == "" {
			recType = "-"
		}
		recStyled := dimStyle.Width(typeCol).Render(recType)

		// Time
		timeStr := formatRelativeTime(e.Timestamp)
		timeStyled := timeStyle.Width(timeCol).Render(timeStr)

		// Domain
		domainText := truncate(e.Domain, domainCol-2)

		// Highlight selected row
		var row string
		if i == v.cursor {
			pointer := selectedStyle.Render("> ")
			domainStyled := selectedStyle.Width(domainCol - 2).Render(domainText)
			row = pointer + domainStyled + cmdStyled + recStyled + timeStyled
		} else {
			pointer := "  "
			domainStyled := domainStyle.Width(domainCol - 2).Render(domainText)
			row = pointer + domainStyled + cmdStyled + recStyled + timeStyled
		}

		lines = append(lines, row)
	}

	// Bottom status
	navHint := dimStyle.Render("j/k navigate  c clear  r reload")
	if len(v.entries) > visible {
		scrollInfo := dimStyle.Render(fmt.Sprintf("  [%d-%d of %d]", v.scrollOffset+1, end, len(v.entries)))
		navHint += scrollInfo
	}
	lines = append(lines, navHint)

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// formatRelativeTime converts a timestamp to a human-friendly relative string.
func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}

	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		return fmt.Sprintf("%dm ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		return fmt.Sprintf("%dh ago", hours)
	case diff < 48*time.Hour:
		return "yesterday"
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	default:
		return t.Format("2006-01-02 15:04")
	}
}

// reverseEntries returns a reversed copy of entries so newest appear first.
func reverseEntries(entries []history.Entry) []history.Entry {
	n := len(entries)
	reversed := make([]history.Entry, n)
	for i, e := range entries {
		reversed[n-1-i] = e
	}
	return reversed
}

// repeatChar repeats a character n times.
func repeatChar(ch byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ch
	}
	return string(b)
}

// renderDetail renders a detailed view of a single history entry.
func (v *HistoryView) renderDetail() string {
	e := v.detailEntry
	titleStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimaryBlue).Bold(true)
	domainStyle := lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray).Width(14)
	valueStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite)
	dimStyle := lipgloss.NewStyle().Foreground(theme.ColorSlate600)

	cmdColor, ok := commandColor[e.Command]
	if !ok {
		cmdColor = theme.ColorCoolGray
	}

	var lines []string
	lines = append(lines, titleStyle.Render("History Entry")+"  "+domainStyle.Render(e.Domain))
	lines = append(lines, "")
	lines = append(lines, labelStyle.Render("Command")+lipgloss.NewStyle().Foreground(cmdColor).Bold(true).Render(e.Command))
	lines = append(lines, labelStyle.Render("Domain")+valueStyle.Render(e.Domain))
	if e.RecordType != "" {
		lines = append(lines, labelStyle.Render("Record Type")+valueStyle.Render(e.RecordType))
	}
	lines = append(lines, labelStyle.Render("Mode")+valueStyle.Render(e.Mode))
	lines = append(lines, labelStyle.Render("Timestamp")+valueStyle.Render(e.Timestamp.Format("2006-01-02 15:04:05")))
	lines = append(lines, labelStyle.Render("Relative")+dimStyle.Render(formatRelativeTime(e.Timestamp)))
	lines = append(lines, "")

	// Show results if stored
	if e.Results != nil {
		lines = append(lines, titleStyle.Render("Results")+" "+dimStyle.Render(strings.Repeat("─", 40)))
		lines = append(lines, "")

		// Try to pretty-print the results as JSON
		data, err := json.MarshalIndent(e.Results, "  ", "  ")
		if err == nil {
			jsonStr := string(data)
			for _, jline := range strings.Split(jsonStr, "\n") {
				lines = append(lines, "  "+dimStyle.Render(jline))
			}
		} else {
			lines = append(lines, "  "+valueStyle.Render(fmt.Sprintf("%v", e.Results)))
		}
		lines = append(lines, "")
	} else {
		lines = append(lines, dimStyle.Render("  No result data stored for this entry."))
		lines = append(lines, "")
	}

	lines = append(lines, dimStyle.Render("  Esc to go back  j/k to scroll"))

	// Scroll
	visible := v.height - 2
	if visible < 1 {
		visible = 1
	}
	maxScroll := len(lines) - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if v.detailScroll > maxScroll {
		v.detailScroll = maxScroll
	}
	end := v.detailScroll + visible
	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[v.detailScroll:end], "\n")
}
