package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Vector-DNS/vdns-tui/internal/client"
	"github.com/Vector-DNS/vdns-tui/internal/config"
	"github.com/Vector-DNS/vdns-tui/internal/dns"
	"github.com/Vector-DNS/vdns-tui/internal/theme"
)

// DNS lookup result messages. These are produced by the lookup commands
// and handled by the LookupView's Update method.

// LocalDNSResultMsg carries the result of a local DNS lookup.
type LocalDNSResultMsg struct {
	Result *dns.LookupResult
	Err    error
}

// RemoteDNSResultMsg carries the result of a remote API DNS lookup.
type RemoteDNSResultMsg struct {
	Result *client.DNSLookupResponse
	Err    error
}

// LookupView performs DNS lookups and displays results in a tabbed, color-coded interface.
type LookupView struct {
	width, height int
	domain        string
	loading       bool
	activeTab     int
	tabTypes      []string // record types that have results

	// Results
	localResult  *dns.LookupResult
	remoteResult *client.DNSLookupResponse
	useRemote    bool
	err          error

	// Config
	config    *config.Config
	apiClient *client.Client // nil if local mode

	// UI
	spinner  spinner.Model
	tableRows []recordRow // rows for the active tab
	tableType string      // record type for the active tab
}

// NewLookupView creates a new DNS lookup view.
func NewLookupView(cfg *config.Config, apiClient *client.Client) *LookupView {
	s := newSpinner()

	return &LookupView{
		config:    cfg,
		apiClient: apiClient,
		useRemote: cfg.ShouldUseRemote() && apiClient != nil,
		spinner:   s,
	}
}

// Update handles messages for the lookup view.
func (v *LookupView) Update(msg tea.Msg) (Component, tea.Cmd) {
	switch msg := msg.(type) {

	case DomainSubmitMsg:
		v.domain = msg.Domain
		v.loading = true
		v.err = nil
		v.localResult = nil
		v.remoteResult = nil
		v.tabTypes = nil
		v.activeTab = 0

		return v, tea.Batch(v.spinner.Tick, v.performLookup(msg.Domain))

	case LocalDNSResultMsg:
		v.loading = false
		if msg.Err != nil {
			v.err = msg.Err
			return v, nil
		}
		v.localResult = msg.Result
		v.tabTypes = v.computeTabTypes()
		v.activeTab = 0
		v.rebuildTable()
		saveToHistory(v.config, "lookup", v.domain, "", "local", msg.Result.Records)
		return v, nil

	case RemoteDNSResultMsg:
		v.loading = false
		if msg.Err != nil {
			v.err = msg.Err
			return v, nil
		}
		v.remoteResult = msg.Result
		v.tabTypes = v.computeTabTypes()
		v.activeTab = 0
		v.rebuildTable()
		saveToHistory(v.config, "lookup", v.domain, "", "remote", msg.Result.Records)
		return v, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			if len(v.tabTypes) > 0 {
				v.activeTab--
				if v.activeTab < 0 {
					v.activeTab = len(v.tabTypes) - 1
				}
				v.rebuildTable()
			}
		case "right", "l":
			if len(v.tabTypes) > 0 {
				v.activeTab++
				if v.activeTab >= len(v.tabTypes) {
					v.activeTab = 0
				}
				v.rebuildTable()
			}
		default:
			// No table component to delegate to.
		}

	case spinner.TickMsg:
		if v.loading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return v, cmd
		}
	}

	return v, nil
}

// View renders the lookup view.
func (v *LookupView) View(width, height int) string {
	v.width = width
	v.height = height

	if v.loading {
		return v.lookupRenderLoading()
	}

	if v.err != nil {
		return v.lookupRenderError()
	}

	if v.localResult == nil && v.remoteResult == nil {
		return v.lookupRenderEmpty()
	}

	return v.lookupRenderResults()
}

// performLookup returns a tea.Cmd that runs the DNS lookup.
func (v *LookupView) performLookup(domain string) tea.Cmd {
	if v.useRemote && v.apiClient != nil {
		c := v.apiClient
		return func() tea.Msg {
			result, err := c.DNSLookup(client.DNSLookupRequest{Domain: domain})
			return RemoteDNSResultMsg{Result: result, Err: err}
		}
	}
	return func() tea.Msg {
		result, err := dns.Lookup(domain, nil)
		return LocalDNSResultMsg{Result: result, Err: err}
	}
}

// computeTabTypes extracts record types that have results, in canonical order.
func (v *LookupView) computeTabTypes() []string {
	records := v.getRecordCounts()
	if records == nil {
		return nil
	}

	// Use the canonical order from dns.SupportedTypes.
	var tabs []string
	for _, t := range dns.SupportedTypes {
		if count, ok := records[t]; ok && count > 0 {
			tabs = append(tabs, t)
		}
	}

	// Catch any types not in SupportedTypes.
	for t, count := range records {
		if count > 0 && !contains(tabs, t) {
			tabs = append(tabs, t)
		}
	}

	return tabs
}

// getRecordCounts returns a map of record type to count.
func (v *LookupView) getRecordCounts() map[string]int {
	if v.localResult != nil {
		m := make(map[string]int)
		for t, recs := range v.localResult.Records {
			m[t] = len(recs)
		}
		return m
	}
	if v.remoteResult != nil {
		m := make(map[string]int)
		for t, recs := range v.remoteResult.Records {
			m[t] = len(recs)
		}
		return m
	}
	return nil
}

// recordRow holds normalized record data for display.
type recordRow struct {
	value    string
	ttl      string
	priority string
}

// getRecordsForType returns normalized rows for a given record type.
func (v *LookupView) getRecordsForType(rtype string) []recordRow {
	if v.localResult != nil {
		recs, ok := v.localResult.Records[rtype]
		if !ok {
			return nil
		}
		rows := make([]recordRow, len(recs))
		for i, r := range recs {
			rows[i] = recordRow{
				value: r.Value,
				ttl:   fmt.Sprintf("%d", r.TTL),
			}
			if r.Priority != nil {
				rows[i].priority = fmt.Sprintf("%d", *r.Priority)
			}
		}
		return rows
	}
	if v.remoteResult != nil {
		recs, ok := v.remoteResult.Records[rtype]
		if !ok {
			return nil
		}
		rows := make([]recordRow, len(recs))
		for i, r := range recs {
			rows[i] = recordRow{
				value: r.Value,
				ttl:   fmt.Sprintf("%d", r.TTL),
			}
			if r.Priority != nil {
				rows[i].priority = fmt.Sprintf("%d", *r.Priority)
			}
		}
		return rows
	}
	return nil
}

// rebuildTable stores the rows for the currently active tab.
func (v *LookupView) rebuildTable() {
	if len(v.tabTypes) == 0 {
		v.tableRows = nil
		v.tableType = ""
		return
	}
	if v.activeTab >= len(v.tabTypes) {
		return
	}

	v.tableType = v.tabTypes[v.activeTab]
	v.tableRows = v.getRecordsForType(v.tableType)
}

// --- Rendering ---

func (v *LookupView) lookupRenderLoading() string {
	return renderLoadingSpinner(v.spinner.View(), "Looking up "+v.domain+"...", v.width, v.height)
}

func (v *LookupView) lookupRenderEmpty() string {
	return renderEmptyState("Enter a domain name to look up its DNS records\n\nUse the input bar above to submit a domain", v.width, v.height)
}

func (v *LookupView) lookupRenderError() string {
	return renderErrorBox(v.err, v.width, v.height)
}

func (v *LookupView) lookupRenderResults() string {
	var sections []string

	// Header
	sections = append(sections, v.lookupRenderHeader())
	sections = append(sections, "")

	// Tabs
	sections = append(sections, v.lookupRenderTabs())
	sections = append(sections, "")

	// Hand-rendered table
	sections = append(sections, v.lookupRenderTable())

	// Summary strip
	sections = append(sections, "")
	sections = append(sections, v.lookupRenderSummary())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// lookupRenderTable renders the record table using plain lipgloss text.
func (v *LookupView) lookupRenderTable() string {
	if len(v.tableRows) == 0 {
		dim := lipgloss.NewStyle().Foreground(theme.ColorCoolGray)
		return "  " + dim.Render("No records found")
	}

	color := recordColor(v.tableType)
	showPriority := v.tableType == "MX" || v.tableType == "SRV"

	// Column widths.
	numW := 4
	ttlW := 8
	priW := 10
	overhead := numW + ttlW + 8 // padding between columns
	if showPriority {
		overhead += priW
	}
	valW := v.width - overhead - 4
	if valW < 20 {
		valW = 20
	}
	if valW > 80 {
		valW = 80
	}

	headerStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
	dividerStyle := lipgloss.NewStyle().Foreground(theme.ColorSlate600)
	rowStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite)
	firstRowStyle := lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true)

	// Header row.
	header := fmt.Sprintf("  %-*s %-*s %-*s", numW, "#", valW, "Value", ttlW, "TTL")
	if showPriority {
		header += fmt.Sprintf(" %-*s", priW, "Priority")
	}

	// Divider row.
	divider := fmt.Sprintf("  %s %s %s",
		strings.Repeat("\u2500", numW),
		strings.Repeat("\u2500", valW),
		strings.Repeat("\u2500", ttlW),
	)
	if showPriority {
		divider += " " + strings.Repeat("\u2500", priW)
	}

	var lines []string
	lines = append(lines, headerStyle.Render(header))
	lines = append(lines, dividerStyle.Render(divider))

	// Data rows.
	for i, r := range v.tableRows {
		val := r.value
		if len(val) > valW {
			val = val[:valW-3] + "..."
		}
		line := fmt.Sprintf("  %-*d %-*s %-*s", numW, i+1, valW, val, ttlW, r.ttl)
		if showPriority {
			line += fmt.Sprintf(" %-*s", priW, r.priority)
		}
		if i == 0 {
			lines = append(lines, firstRowStyle.Render(line))
		} else {
			lines = append(lines, rowStyle.Render(line))
		}
	}

	return strings.Join(lines, "\n")
}

func (v *LookupView) lookupRenderHeader() string {
	contentWidth := v.width - 6
	if contentWidth < 20 {
		contentWidth = 20
	}

	domain := lipgloss.NewStyle().
		Foreground(theme.ColorWhite).
		Bold(true).
		Render(v.domain)

	meta := v.lookupBuildMetaLine()

	inner := lipgloss.JoinVertical(lipgloss.Left,
		"  "+domain,
		"  "+meta,
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorSlate600).
		Width(contentWidth).
		Render(inner)

	return box
}

func (v *LookupView) lookupBuildMetaLine() string {
	dimStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray)
	valStyle := lipgloss.NewStyle().Foreground(theme.ColorAccentCyan)

	var parts []string

	if v.localResult != nil {
		parts = append(parts, dimStyle.Render("Resolver: ")+valStyle.Render(v.localResult.Resolver))
		parts = append(parts, dimStyle.Render("Query time: ")+valStyle.Render(fmt.Sprintf("%dms", v.localResult.QueryTimeMs)))
		parts = append(parts, lipgloss.NewStyle().Foreground(theme.ColorSlate600).Render("\u2717 DNSSEC"))
		ts := v.localResult.Timestamp.Format(time.DateTime)
		parts = append(parts, dimStyle.Render("At: ")+valStyle.Render(ts))
		parts = append(parts, dimStyle.Render("Mode: ")+valStyle.Render("local"))
	} else if v.remoteResult != nil {
		parts = append(parts, dimStyle.Render("Resolver: ")+valStyle.Render(v.remoteResult.Resolver))
		parts = append(parts, dimStyle.Render("Query time: ")+valStyle.Render(fmt.Sprintf("%dms", v.remoteResult.QueryTimeMs)))
		if v.remoteResult.DNSSEC {
			parts = append(parts, lipgloss.NewStyle().Foreground(theme.ColorEmerald).Bold(true).Render("\u2713 DNSSEC"))
		} else {
			parts = append(parts, lipgloss.NewStyle().Foreground(theme.ColorSlate600).Render("\u2717 DNSSEC"))
		}
		if v.remoteResult.Timestamp != "" {
			parts = append(parts, dimStyle.Render("At: ")+valStyle.Render(v.remoteResult.Timestamp))
		}
		parts = append(parts, dimStyle.Render("Mode: ")+valStyle.Render("remote"))
	}

	return strings.Join(parts, dimStyle.Render("  |  "))
}

func (v *LookupView) lookupRenderTabs() string {
	if len(v.tabTypes) == 0 {
		return ""
	}

	var tabs []string
	for i, t := range v.tabTypes {
		color := recordColor(t)
		count := v.countRecords(t)
		countLabel := fmt.Sprintf(" (%d)", count)

		if i == v.activeTab {
			// Active tab: colored, bold, with underline chars and count.
			label := lipgloss.NewStyle().
				Foreground(color).
				Bold(true).
				Render(t + countLabel)
			underline := lipgloss.NewStyle().
				Foreground(color).
				Render(strings.Repeat("\u2594", lipgloss.Width(t+countLabel)))
			tab := lipgloss.JoinVertical(lipgloss.Center, label, underline)
			tabs = append(tabs, tab)
		} else {
			// Inactive tab: dimmed with count.
			label := lipgloss.NewStyle().
				Foreground(theme.ColorCoolGray).
				Render(t + countLabel)
			spacer := strings.Repeat(" ", lipgloss.Width(t+countLabel))
			tab := lipgloss.JoinVertical(lipgloss.Center, label, spacer)
			tabs = append(tabs, tab)
		}
	}

	sep := lipgloss.NewStyle().
		Foreground(theme.ColorSlate600).
		Render("  |  ")

	return "  " + lipgloss.JoinHorizontal(lipgloss.Top, interleave(tabs, sep)...)
}

func (v *LookupView) lookupRenderSummary() string {
	var badges []string

	// Show all supported types, even those with 0 records.
	for _, t := range dns.SupportedTypes {
		count := v.countRecords(t)
		color := recordColor(t)

		badge := lipgloss.NewStyle().
			Foreground(color).
			Bold(count > 0).
			Render(fmt.Sprintf("%s:%d", t, count))

		badges = append(badges, badge)
	}

	return "  " + strings.Join(badges, "  ")
}

// countRecords returns how many records of a given type exist.
func (v *LookupView) countRecords(rtype string) int {
	if v.localResult != nil {
		return len(v.localResult.Records[rtype])
	}
	if v.remoteResult != nil {
		return len(v.remoteResult.Records[rtype])
	}
	return 0
}

// --- Helpers ---

func recordColor(rtype string) lipgloss.Color {
	if c, ok := theme.RecordTypeColor[rtype]; ok {
		return c
	}
	return theme.ColorCoolGray
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// interleave places sep between each element of items.
func interleave(items []string, sep string) []string {
	if len(items) == 0 {
		return nil
	}
	result := make([]string, 0, len(items)*2-1)
	for i, item := range items {
		if i > 0 {
			result = append(result, sep)
		}
		result = append(result, item)
	}
	return result
}
