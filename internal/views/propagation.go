package views

import (
	"fmt"
	"net"
	"strings"

	"github.com/Vector-DNS/vdns-tui/internal/client"
	"github.com/Vector-DNS/vdns-tui/internal/config"
	"github.com/Vector-DNS/vdns-tui/internal/dns"
	"github.com/Vector-DNS/vdns-tui/internal/theme"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var propagationRecordTypes = []string{"A", "AAAA", "CNAME", "MX", "NS"}

// PropagationView displays DNS propagation check results across resolvers.
type PropagationView struct {
	width  int
	height int
	domain string

	recordType    string
	recordTypeIdx int

	loading bool
	spinner spinner.Model

	localResults []dns.PropagationResult
	remoteResult *client.PropagationResponse
	consistent   bool
	useRemote    bool
	err          error

	config    *config.Config
	apiClient *client.Client

	scrollOffset int
}

// NewPropagationView creates a new propagation view.
func NewPropagationView(cfg *config.Config, apiClient *client.Client) *PropagationView {
	s := newSpinner()

	return &PropagationView{
		recordType:    "A",
		recordTypeIdx: 0,
		spinner:       s,
		config:        cfg,
		apiClient:     apiClient,
		useRemote:     cfg.ShouldUseRemote(),
	}
}

// Update handles messages for the propagation view.
func (v *PropagationView) Update(msg tea.Msg) (Component, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height

	case DomainSubmitMsg:
		v.domain = msg.Domain
		v.loading = true
		v.err = nil
		v.localResults = nil
		v.remoteResult = nil
		return v, tea.Batch(v.spinner.Tick, v.runPropagation())

	case LocalPropagationResultMsg:
		v.loading = false
		if msg.Err != nil {
			v.err = msg.Err
		} else {
			v.localResults = msg.Results
			v.consistent = msg.Consistent
			v.scrollOffset = 0
			saveToHistory(v.config, "propagation", v.domain, v.recordType, "local", msg.Results)
		}

	case RemotePropagationResultMsg:
		v.loading = false
		if msg.Err != nil {
			v.err = msg.Err
		} else {
			v.remoteResult = msg.Result
			v.consistent = msg.Result.Consistent
			v.scrollOffset = 0
			saveToHistory(v.config, "propagation", v.domain, v.recordType, "remote", msg.Result)
		}

	case spinner.TickMsg:
		if v.loading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return v, cmd
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			v.recordTypeIdx = (v.recordTypeIdx - 1 + len(propagationRecordTypes)) % len(propagationRecordTypes)
			v.recordType = propagationRecordTypes[v.recordTypeIdx]
			if v.domain != "" {
				return v, v.propRerun()
			}
		case "right", "l":
			v.recordTypeIdx = (v.recordTypeIdx + 1) % len(propagationRecordTypes)
			v.recordType = propagationRecordTypes[v.recordTypeIdx]
			if v.domain != "" {
				return v, v.propRerun()
			}
		case "up", "k":
			if v.scrollOffset > 0 {
				v.scrollOffset--
			}
		case "down", "j":
			entries := v.propCollectEntries()
			maxOffset := len(entries) - (v.height - 14)
			if maxOffset < 0 {
				maxOffset = 0
			}
			if v.scrollOffset < maxOffset {
				v.scrollOffset++
			}
		}
	}
	return v, nil
}

// propRerun triggers a new propagation check with the current domain and record type.
func (v *PropagationView) propRerun() tea.Cmd {
	v.loading = true
	v.err = nil
	v.localResults = nil
	v.remoteResult = nil
	return tea.Batch(v.spinner.Tick, v.runPropagation())
}

// runPropagation returns a command that performs the propagation check.
func (v *PropagationView) runPropagation() tea.Cmd {
	domain := v.domain
	recordType := v.recordType

	if v.useRemote && v.apiClient != nil {
		return func() tea.Msg {
			result, err := v.apiClient.Propagation(client.PropagationRequest{
				Domain: domain,
				Type:   recordType,
			})
			return RemotePropagationResultMsg{Result: result, Err: err}
		}
	}

	return func() tea.Msg {
		results, consistent := dns.CheckPropagation(domain, recordType, dns.DefaultResolvers)
		return LocalPropagationResultMsg{
			Results:    results,
			Consistent: consistent,
		}
	}
}

// View renders the propagation view.
func (v *PropagationView) View(width, height int) string {
	v.width = width
	v.height = height

	if v.domain == "" && !v.loading {
		return v.propRenderEmpty()
	}
	if v.err != nil {
		return v.propRenderError()
	}

	var sections []string
	sections = append(sections, v.propRenderHeader())
	sections = append(sections, "")

	if v.loading {
		// Show loading in the table area so user can still see their selection.
		msg := fmt.Sprintf("Checking propagation across %d resolvers...", len(dns.DefaultResolvers))
		loadingStyle := lipgloss.NewStyle().Foreground(theme.ColorAccentCyan)
		sections = append(sections, "  "+loadingStyle.Render(v.spinner.View()+"  "+msg))
	} else if v.localResults != nil || v.remoteResult != nil {
		sections = append(sections, v.propRenderTable())
		sections = append(sections, "")
		sections = append(sections, v.propRenderSummary())
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Top, content)
}

// propRenderEmpty shows the placeholder state.
func (v *PropagationView) propRenderEmpty() string {
	hint := lipgloss.NewStyle().Foreground(theme.ColorCoolGray)
	typeHint := lipgloss.NewStyle().Foreground(theme.ColorSlate600).Italic(true)
	content := lipgloss.JoinVertical(lipgloss.Center,
		hint.Render("Enter a domain and press Enter to check DNS propagation"),
		"",
		typeHint.Render("Use left/right arrows to change record type"),
	)
	return renderCentered(content, v.width, v.height)
}

// propRenderError shows the error state.
func (v *PropagationView) propRenderError() string {
	return renderErrorBox(v.err, v.width, v.height)
}

// propRenderHeader renders the domain, record type selector, and consistency status.
func (v *PropagationView) propRenderHeader() string {
	titleStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimaryBlue).Bold(true)
	domainStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(theme.ColorSlate600)

	// Record type selector
	var typeParts []string
	for i, rt := range propagationRecordTypes {
		if i == v.recordTypeIdx {
			color := theme.RecordTypeColor[rt]
			if color == "" {
				color = theme.ColorWhite
			}
			active := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#0F172A")).
				Background(color).
				Bold(true).
				Padding(0, 1).
				Render(rt)
			typeParts = append(typeParts, active)
		} else {
			inactive := dimStyle.Padding(0, 1).Render(rt)
			typeParts = append(typeParts, inactive)
		}
	}
	typeSelector := lipgloss.JoinHorizontal(lipgloss.Center, typeParts...)

	// Consistency badge - only show when we have results
	var statusBadge string
	if v.localResults != nil || v.remoteResult != nil {
		if v.consistent {
			statusBadge = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#0F172A")).
				Background(theme.ColorEmerald).
				Bold(true).
				Padding(0, 1).
				Render("Consistent")
		} else {
			statusBadge = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#0F172A")).
				Background(theme.ColorRose).
				Bold(true).
				Padding(0, 1).
				Render("Inconsistent")
		}
	}

	// Build the row: domain + type selector + optional badge
	headerContent := lipgloss.JoinHorizontal(lipgloss.Center,
		domainStyle.Render(v.domain),
		lipgloss.NewStyle().Width(3).Render(""),
		typeSelector,
	)
	if statusBadge != "" {
		headerContent = lipgloss.JoinHorizontal(lipgloss.Center,
			headerContent,
			lipgloss.NewStyle().Width(3).Render(""),
			statusBadge,
		)
	}

	headerWidth := v.width - 4
	if headerWidth < 40 {
		headerWidth = 40
	}
	if headerWidth > 80 {
		headerWidth = 80
	}

	// Title inside the box, then the selector row below it
	innerContent := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("Propagation Check"),
		headerContent,
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorPrimaryBlue).
		Padding(0, 2).
		Width(headerWidth)

	return box.Render(innerContent)
}

// resolverEntry holds parsed data for a single resolver result row.
type resolverEntry struct {
	name    string
	address string
	values  []string
	ttl     string
	success bool
	errMsg  string
}

// propCollectEntries gathers resolver entries from local or remote results.
func (v *PropagationView) propCollectEntries() []resolverEntry {
	var entries []resolverEntry

	if v.localResults != nil {
		for _, r := range v.localResults {
			host, _, err := net.SplitHostPort(r.Resolver.Address)
			if err != nil {
				host = r.Resolver.Address
			}
			ttl := "-"
			if r.TTL != nil {
				ttl = fmt.Sprintf("%d", *r.TTL)
			}
			entries = append(entries, resolverEntry{
				name:    r.Resolver.Name,
				address: host,
				values:  r.Values,
				ttl:     ttl,
				success: r.Success,
				errMsg:  r.Error,
			})
		}
	} else if v.remoteResult != nil {
		for _, r := range v.remoteResult.Results {
			ttl := "-"
			if r.TTL != nil {
				ttl = fmt.Sprintf("%d", *r.TTL)
			}
			addr := r.Resolver
			if h, _, err := net.SplitHostPort(addr); err == nil {
				addr = h
			}
			entries = append(entries, resolverEntry{
				name:    r.Name,
				address: addr,
				values:  r.Values,
				ttl:     ttl,
				success: r.Success,
				errMsg:  r.Error,
			})
		}
	}

	return entries
}

// propRenderTable renders the resolver results as a hand-drawn table.
func (v *PropagationView) propRenderTable() string {
	entries := v.propCollectEntries()
	if len(entries) == 0 {
		return ""
	}

	// Column widths
	colResolver := 24
	colIP := 17
	colStatus := 6
	colTTL := 5
	// Values column takes remaining space
	colValues := v.width - colResolver - colIP - colStatus - colTTL - 12 // 12 for padding/gaps
	if colValues < 20 {
		colValues = 20
	}
	if colValues > 50 {
		colValues = 50
	}

	headerStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimaryBlue).Bold(true)
	separatorStyle := lipgloss.NewStyle().Foreground(theme.ColorSlate600)
	nameStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite)
	ipStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray)
	okStyle := lipgloss.NewStyle().Foreground(theme.ColorEmerald).Bold(true)
	failStyle := lipgloss.NewStyle().Foreground(theme.ColorRose).Bold(true)
	valStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite)
	ttlStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray)

	pad := func(s string, w int) string {
		if lipgloss.Width(s) >= w {
			return s
		}
		return s + strings.Repeat(" ", w-lipgloss.Width(s))
	}

	truncate := func(s string, w int) string {
		if len(s) <= w {
			return s
		}
		if w <= 3 {
			return s[:w]
		}
		return s[:w-3] + "..."
	}

	var lines []string

	// Header
	header := fmt.Sprintf("  %s %s %s %s %s",
		pad(headerStyle.Render("Resolver"), colResolver),
		pad(headerStyle.Render("IP"), colIP),
		pad(headerStyle.Render("Status"), colStatus),
		pad(headerStyle.Render("Values"), colValues),
		headerStyle.Render("TTL"),
	)
	lines = append(lines, header)

	// Separator
	sep := fmt.Sprintf("  %s %s %s %s %s",
		separatorStyle.Render(strings.Repeat("-", colResolver)),
		separatorStyle.Render(strings.Repeat("-", colIP)),
		separatorStyle.Render(strings.Repeat("-", colStatus)),
		separatorStyle.Render(strings.Repeat("-", colValues)),
		separatorStyle.Render(strings.Repeat("-", colTTL)),
	)
	lines = append(lines, sep)

	// Visible rows with scroll offset
	visibleRows := v.height - 14
	if visibleRows < 3 {
		visibleRows = 3
	}
	start := v.scrollOffset
	if start > len(entries) {
		start = len(entries)
	}
	end := start + visibleRows
	if end > len(entries) {
		end = len(entries)
	}

	for _, e := range entries[start:end] {
		status := okStyle.Render(pad("OK", colStatus))
		valStr := strings.Join(e.values, ", ")
		if !e.success {
			status = failStyle.Render(pad("FAIL", colStatus))
			valStr = e.errMsg
		}
		if len(e.values) == 0 && e.success {
			valStr = "(no records)"
		}
		valStr = truncate(valStr, colValues)

		row := fmt.Sprintf("  %s %s %s %s %s",
			pad(nameStyle.Render(truncate(e.name, colResolver)), colResolver),
			pad(ipStyle.Render(truncate(e.address, colIP)), colIP),
			status,
			pad(valStyle.Render(valStr), colValues),
			ttlStyle.Render(e.ttl),
		)
		lines = append(lines, row)
	}

	// Scroll indicator
	if len(entries) > visibleRows {
		scrollInfo := lipgloss.NewStyle().Foreground(theme.ColorSlate600).Italic(true)
		lines = append(lines, "  "+scrollInfo.Render(
			fmt.Sprintf("Showing %d-%d of %d (j/k to scroll)", start+1, end, len(entries)),
		))
	}

	return strings.Join(lines, "\n")
}

// propRenderSummary renders the summary bar with resolver count and progress gauge.
func (v *PropagationView) propRenderSummary() string {
	total := 0
	responding := 0

	if v.localResults != nil {
		total = len(v.localResults)
		for _, r := range v.localResults {
			if r.Success {
				responding++
			}
		}
	} else if v.remoteResult != nil {
		total = len(v.remoteResult.Results)
		for _, r := range v.remoteResult.Results {
			if r.Success {
				responding++
			}
		}
	}

	if total == 0 {
		return ""
	}

	pct := float64(responding) / float64(total)
	percentage := int(pct * 100)

	labelStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray)
	countStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite).Bold(true)
	pctStyle := lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true)

	resolverText := fmt.Sprintf("Resolvers: %s",
		countStyle.Render(fmt.Sprintf("%d/%d responding", responding, total)))

	// Configure progress bar color based on consistency level.
	var prog progress.Model
	if percentage >= 90 {
		prog = progress.New(
			progress.WithGradient(string(theme.ColorEmerald), string(theme.ColorAccentCyan)),
			progress.WithWidth(30),
			progress.WithoutPercentage(),
		)
	} else if percentage >= 60 {
		prog = progress.New(
			progress.WithGradient(string(theme.ColorAmber), string(theme.ColorRose)),
			progress.WithWidth(30),
			progress.WithoutPercentage(),
		)
	} else {
		prog = progress.New(
			progress.WithGradient(string(theme.ColorRose), string(theme.ColorRose)),
			progress.WithWidth(30),
			progress.WithoutPercentage(),
		)
	}

	gauge := fmt.Sprintf("%s %s",
		prog.ViewAs(pct),
		pctStyle.Render(fmt.Sprintf("%d%% consistent", percentage)),
	)

	summary := lipgloss.JoinHorizontal(lipgloss.Center,
		labelStyle.Render(resolverText),
		lipgloss.NewStyle().Width(4).Render(""),
		gauge,
	)

	return lipgloss.Place(v.width-4, 1, lipgloss.Center, lipgloss.Center, summary)
}
