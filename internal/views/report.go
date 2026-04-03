package views

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Vector-DNS/vdns-tui/internal/client"
	"github.com/Vector-DNS/vdns-tui/internal/config"
	"github.com/Vector-DNS/vdns-tui/internal/dns"
	"github.com/Vector-DNS/vdns-tui/internal/theme"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// reportDoneMsg signals a single report task completed.
type reportDoneMsg struct {
	task   string
	err    error
	result any
}

// ReportView runs all lookups for a domain and displays a combined report.
type ReportView struct {
	width, height int
	domain        string
	config        *config.Config
	apiClient     *client.Client
	spinner       spinner.Model
	scrollOffset  int

	running      bool
	confirmed    bool // user confirmed the rate limit warning
	tasksTotal   int
	tasksDone    int
	currentTask  string

	// Results
	dnsResult         *dns.LookupResult
	propagation       []dns.PropagationResult
	propConsistent    bool
	benchmark         []dns.BenchmarkResult
	whoisResult       *client.WhoisResponse
	sslResult         *client.SSLResponse
	availResult       *client.AvailabilityResponse
	errors            []string
}

// NewReportView creates a new full report view.
func NewReportView(cfg *config.Config, apiClient *client.Client) *ReportView {
	return &ReportView{
		config:    cfg,
		apiClient: apiClient,
		spinner:   newSpinner(),
	}
}

// Update handles messages.
func (v *ReportView) Update(msg tea.Msg) (Component, tea.Cmd) {
	switch msg := msg.(type) {
	case DomainSubmitMsg:
		v.domain = msg.Domain
		v.confirmed = false
		v.clearResults()
		// If remote, show warning first. If local, start immediately.
		if v.config.IsLoggedIn() && v.apiClient != nil {
			return v, nil // wait for Enter confirmation
		}
		return v, v.startReport()

	case reportDoneMsg:
		v.tasksDone++
		v.storeResult(msg)
		if v.tasksDone >= v.tasksTotal {
			v.running = false
			saveToHistory(v.config, "report", v.domain, "", v.mode(), nil)
		}
		return v, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if v.domain != "" && !v.confirmed && !v.running && v.config.IsLoggedIn() {
				v.confirmed = true
				return v, v.startReport()
			}
		case "up", "k":
			if v.scrollOffset > 0 {
				v.scrollOffset--
			}
		case "down", "j":
			v.scrollOffset++
		}

	case spinner.TickMsg:
		if v.running {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return v, cmd
		}
	}
	return v, nil
}

func (v *ReportView) mode() string {
	if v.config.IsLoggedIn() && v.apiClient != nil {
		return "remote"
	}
	return "local"
}

func (v *ReportView) clearResults() {
	v.dnsResult = nil
	v.propagation = nil
	v.benchmark = nil
	v.whoisResult = nil
	v.sslResult = nil
	v.availResult = nil
	v.errors = nil
	v.tasksDone = 0
	v.scrollOffset = 0
}

func (v *ReportView) startReport() tea.Cmd {
	v.running = true
	v.confirmed = true
	v.clearResults()

	domain := v.domain
	isRemote := v.config.IsLoggedIn() && v.apiClient != nil

	// Always run: DNS lookup, propagation, benchmark
	v.tasksTotal = 3
	cmds := []tea.Cmd{
		v.spinner.Tick,
		v.runTask("DNS Lookup", func() (any, error) {
			return dns.Lookup(domain, nil)
		}),
		v.runTask("Propagation", func() (any, error) {
			results, consistent := dns.CheckPropagation(domain, "A", dns.DefaultResolvers)
			return []any{results, consistent}, nil
		}),
		v.runTask("Benchmark", func() (any, error) {
			return dns.Benchmark(domain, dns.DefaultResolvers), nil
		}),
	}

	// Remote-only tasks
	if isRemote {
		v.tasksTotal += 3
		c := v.apiClient
		cmds = append(cmds,
			v.runTask("WHOIS", func() (any, error) {
				return c.Whois(client.WhoisRequest{Domain: domain})
			}),
			v.runTask("SSL", func() (any, error) {
				return c.SSL(domain)
			}),
			v.runTask("Availability", func() (any, error) {
				return c.Availability(client.AvailabilityRequest{Domain: domain})
			}),
		)
	}

	return tea.Batch(cmds...)
}

func (v *ReportView) runTask(name string, fn func() (any, error)) tea.Cmd {
	return func() tea.Msg {
		result, err := fn()
		return reportDoneMsg{task: name, err: err, result: result}
	}
}

func (v *ReportView) storeResult(msg reportDoneMsg) {
	if msg.err != nil {
		v.errors = append(v.errors, fmt.Sprintf("%s: %s", msg.task, msg.err.Error()))
		return
	}
	switch msg.task {
	case "DNS Lookup":
		if r, ok := msg.result.(*dns.LookupResult); ok {
			v.dnsResult = r
		}
	case "Propagation":
		if parts, ok := msg.result.([]any); ok && len(parts) == 2 {
			if results, ok := parts[0].([]dns.PropagationResult); ok {
				v.propagation = results
			}
			if consistent, ok := parts[1].(bool); ok {
				v.propConsistent = consistent
			}
		}
	case "Benchmark":
		if r, ok := msg.result.([]dns.BenchmarkResult); ok {
			v.benchmark = r
		}
	case "WHOIS":
		if r, ok := msg.result.(*client.WhoisResponse); ok {
			v.whoisResult = r
		}
	case "SSL":
		if r, ok := msg.result.(*client.SSLResponse); ok {
			v.sslResult = r
		}
	case "Availability":
		if r, ok := msg.result.(*client.AvailabilityResponse); ok {
			v.availResult = r
		}
	}
}

// View renders the full report.
func (v *ReportView) View(width, height int) string {
	v.width = width
	v.height = height

	if v.domain == "" {
		return renderEmptyState("Enter a domain to generate a full report\n\nThis runs DNS, Propagation, and Benchmark lookups.\nWith remote mode, also WHOIS, SSL, and Availability.", width, height)
	}

	// Rate limit warning for remote mode
	if v.config.IsLoggedIn() && !v.confirmed && !v.running {
		return v.renderWarning()
	}

	var lines []string

	// Title
	titleStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimaryBlue).Bold(true)
	domainStyle := lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(theme.ColorSlate600)

	lines = append(lines, titleStyle.Render("Full Report")+"  "+domainStyle.Render(v.domain))
	lines = append(lines, "")

	// Progress
	if v.running {
		progress := fmt.Sprintf("%s  Running %d/%d tasks...", v.spinner.View(), v.tasksDone, v.tasksTotal)
		lines = append(lines, lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Render(progress))
		lines = append(lines, "")
	} else {
		lines = append(lines, dimStyle.Render(fmt.Sprintf("Completed %d/%d tasks", v.tasksDone, v.tasksTotal)))
		lines = append(lines, "")
	}

	// Errors
	if len(v.errors) > 0 {
		errStyle := lipgloss.NewStyle().Foreground(theme.ColorRose)
		for _, e := range v.errors {
			lines = append(lines, errStyle.Render("  ! "+e))
		}
		lines = append(lines, "")
	}

	// DNS Results
	if v.dnsResult != nil {
		lines = append(lines, v.renderDNSSection()...)
	}

	// Propagation
	if v.propagation != nil {
		lines = append(lines, v.renderPropSection()...)
	}

	// Benchmark
	if v.benchmark != nil {
		lines = append(lines, v.renderBenchSection()...)
	}

	// WHOIS
	if v.whoisResult != nil {
		lines = append(lines, v.renderWhoisSection()...)
	}

	// SSL
	if v.sslResult != nil {
		lines = append(lines, v.renderSSLSection()...)
	}

	// Availability
	if v.availResult != nil {
		lines = append(lines, v.renderAvailSection()...)
	}

	// Scroll
	visible := height - 2
	if visible < 1 {
		visible = 1
	}
	maxScroll := len(lines) - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if v.scrollOffset > maxScroll {
		v.scrollOffset = maxScroll
	}
	end := v.scrollOffset + visible
	if end > len(lines) {
		end = len(lines)
	}

	content := strings.Join(lines[v.scrollOffset:end], "\n")

	if len(lines) > visible {
		scrollHint := dimStyle.Render(fmt.Sprintf("  [%d-%d of %d lines]  j/k to scroll", v.scrollOffset+1, end, len(lines)))
		content += "\n" + scrollHint
	}

	return content
}

func (v *ReportView) renderWarning() string {
	warnStyle := lipgloss.NewStyle().Foreground(theme.ColorAmber).Bold(true)
	textStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray)
	hintStyle := lipgloss.NewStyle().Foreground(theme.ColorAccentCyan)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorAmber).
		Padding(1, 3).
		Width(60)

	content := lipgloss.JoinVertical(lipgloss.Center,
		warnStyle.Render("Rate Limit Warning"),
		"",
		textStyle.Render("Running a full report in remote mode will use"),
		textStyle.Render("6 API calls (DNS, Propagation, Benchmark,"),
		textStyle.Render("WHOIS, SSL, and Availability)."),
		"",
		textStyle.Render("This counts against your daily rate limit."),
		"",
		hintStyle.Render("Press Enter to continue, or Esc to cancel."),
	)

	return renderCentered(box.Render(content), v.width, v.height)
}

func (v *ReportView) sectionHeader(title string) string {
	style := lipgloss.NewStyle().Foreground(theme.ColorPrimaryBlue).Bold(true)
	dim := lipgloss.NewStyle().Foreground(theme.ColorSlate600)
	return style.Render(title) + " " + dim.Render(strings.Repeat("─", maxInt(0, 50-len(title))))
}

func (v *ReportView) kv(label, value string) string {
	l := lipgloss.NewStyle().Foreground(theme.ColorCoolGray).Width(16).Render(label)
	val := lipgloss.NewStyle().Foreground(theme.ColorWhite).Render(value)
	return "  " + l + val
}

func (v *ReportView) renderDNSSection() []string {
	r := v.dnsResult
	lines := []string{v.sectionHeader("DNS Records")}
	lines = append(lines, v.kv("Resolver", r.Resolver))
	lines = append(lines, v.kv("Query Time", fmt.Sprintf("%dms", r.QueryTimeMs)))

	for _, rtype := range dns.SupportedTypes {
		recs, ok := r.Records[rtype]
		if !ok || len(recs) == 0 {
			continue
		}
		color := theme.RecordTypeColor[rtype]
		typeStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
		lines = append(lines, "  "+typeStyle.Render(fmt.Sprintf("%s (%d)", rtype, len(recs))))
		for _, rec := range recs {
			val := fmt.Sprintf("    %s", rec.Value)
			if rec.Priority != nil {
				val = fmt.Sprintf("    %d %s", *rec.Priority, rec.Value)
			}
			dim := lipgloss.NewStyle().Foreground(theme.ColorSlate600)
			lines = append(lines, lipgloss.NewStyle().Foreground(theme.ColorWhite).Render(val)+dim.Render(fmt.Sprintf("  TTL:%d", rec.TTL)))
		}
	}
	lines = append(lines, "")
	return lines
}

func (v *ReportView) renderPropSection() []string {
	lines := []string{v.sectionHeader("Propagation (A Record)")}

	total := len(v.propagation)
	responding := 0
	for _, r := range v.propagation {
		if r.Success {
			responding++
		}
	}

	badge := "Consistent"
	badgeColor := theme.ColorEmerald
	if !v.propConsistent {
		badge = "Inconsistent"
		badgeColor = theme.ColorRose
	}
	badgeStyle := lipgloss.NewStyle().Foreground(badgeColor).Bold(true)

	lines = append(lines, v.kv("Responding", fmt.Sprintf("%d/%d", responding, total)))
	lines = append(lines, v.kv("Status", badgeStyle.Render(badge)))
	lines = append(lines, "")
	return lines
}

func (v *ReportView) renderBenchSection() []string {
	lines := []string{v.sectionHeader("Benchmark")}

	var successful []dns.BenchmarkResult
	for _, r := range v.benchmark {
		if r.Success {
			successful = append(successful, r)
		}
	}

	if len(successful) == 0 {
		dim := lipgloss.NewStyle().Foreground(theme.ColorSlate600)
		lines = append(lines, "  "+dim.Render("No successful results"))
		lines = append(lines, "")
		return lines
	}

	sort.Slice(successful, func(i, j int) bool {
		return successful[i].LatencyMs < successful[j].LatencyMs
	})

	fastest := successful[0]
	slowest := successful[len(successful)-1]

	fastStyle := lipgloss.NewStyle().Foreground(theme.ColorEmerald).Bold(true)
	slowStyle := lipgloss.NewStyle().Foreground(theme.ColorRose)

	lines = append(lines, v.kv("Fastest", fastStyle.Render(fmt.Sprintf("%s %dms", fastest.Resolver.Name, fastest.LatencyMs))))
	lines = append(lines, v.kv("Slowest", slowStyle.Render(fmt.Sprintf("%s %dms", slowest.Resolver.Name, slowest.LatencyMs))))
	lines = append(lines, v.kv("Success", fmt.Sprintf("%d/%d", len(successful), len(v.benchmark))))

	// Top 5 bar chart
	dim := lipgloss.NewStyle().Foreground(theme.ColorSlate600)
	lines = append(lines, "  "+dim.Render("Top 5:"))
	maxMs := slowest.LatencyMs
	if maxMs == 0 {
		maxMs = 1
	}
	count := 5
	if count > len(successful) {
		count = len(successful)
	}
	for _, r := range successful[:count] {
		barW := int(float64(r.LatencyMs) / float64(maxMs) * 20)
		bar := lipgloss.NewStyle().Foreground(theme.ColorEmerald).Render(strings.Repeat("█", barW))
		empty := lipgloss.NewStyle().Foreground(theme.ColorSlate800).Render(strings.Repeat("░", 20-barW))
		resolverName := r.Resolver.Name
		if len(resolverName) > 20 {
			resolverName = resolverName[:17] + "..."
		}
		name := fmt.Sprintf("%-20s", resolverName)
		nameStyled := lipgloss.NewStyle().Foreground(theme.ColorWhite).Render(name)
		ms := lipgloss.NewStyle().Foreground(theme.ColorCoolGray).Render(fmt.Sprintf("%dms", r.LatencyMs))
		lines = append(lines, fmt.Sprintf("    %s %s%s %s", nameStyled, bar, empty, ms))
	}
	lines = append(lines, "")
	return lines
}

func (v *ReportView) renderWhoisSection() []string {
	r := v.whoisResult.Result
	lines := []string{v.sectionHeader("WHOIS")}

	deref := func(s *string) string {
		if s != nil {
			return *s
		}
		return "-"
	}

	lines = append(lines, v.kv("Registrar", deref(r.Registrar)))
	lines = append(lines, v.kv("Created", deref(r.CreatedDate)))
	lines = append(lines, v.kv("Expires", deref(r.ExpiresDate)))
	lines = append(lines, v.kv("DNSSEC", deref(r.DNSSEC)))
	if len(r.Nameservers) > 0 {
		lines = append(lines, v.kv("Nameservers", strings.Join(r.Nameservers, ", ")))
	}
	lines = append(lines, "")
	return lines
}

func (v *ReportView) renderSSLSection() []string {
	r := v.sslResult
	lines := []string{v.sectionHeader("SSL Certificate")}

	deref := func(s *string) string {
		if s != nil {
			return *s
		}
		return "-"
	}

	status := "Valid"
	statusColor := theme.ColorEmerald
	if !r.IsValid {
		status = "Invalid"
		statusColor = theme.ColorRose
	}
	statusStyle := lipgloss.NewStyle().Foreground(statusColor).Bold(true)
	lines = append(lines, v.kv("Status", statusStyle.Render(status)))
	lines = append(lines, v.kv("Subject", deref(r.Subject)))
	lines = append(lines, v.kv("Issuer", deref(r.Issuer)))
	lines = append(lines, v.kv("Protocol", deref(r.Protocol)))
	if r.ExpiresAt != nil {
		days := int(time.Until(*r.ExpiresAt).Hours() / 24)
		lines = append(lines, v.kv("Expires In", fmt.Sprintf("%d days", days)))
	}
	lines = append(lines, "")
	return lines
}

func (v *ReportView) renderAvailSection() []string {
	r := v.availResult.Result
	lines := []string{v.sectionHeader("Availability")}

	if r.Available {
		avail := lipgloss.NewStyle().Foreground(theme.ColorEmerald).Bold(true).Render("AVAILABLE")
		lines = append(lines, v.kv("Status", avail))
	} else {
		taken := lipgloss.NewStyle().Foreground(theme.ColorRose).Bold(true).Render("TAKEN")
		lines = append(lines, v.kv("Status", taken))
	}
	if len(r.RDAPStatus) > 0 {
		lines = append(lines, v.kv("RDAP", strings.Join(r.RDAPStatus, ", ")))
	}
	lines = append(lines, "")
	return lines
}
