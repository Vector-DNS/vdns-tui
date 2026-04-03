package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Vector-DNS/vdns-tui/internal/config"
	"github.com/Vector-DNS/vdns-tui/internal/dns"
	"github.com/Vector-DNS/vdns-tui/internal/theme"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// BenchmarkView displays DNS resolver benchmark results with bar charts.
type BenchmarkView struct {
	width   int
	height  int
	domain  string
	loading bool
	spinner spinner.Model
	results []dns.BenchmarkResult
	err     error
	config  *config.Config
}

// NewBenchmarkView creates a new benchmark view.
func NewBenchmarkView(cfg *config.Config) *BenchmarkView {
	s := newSpinner()

	return &BenchmarkView{
		spinner: s,
		config:  cfg,
	}
}

// Update handles messages for the benchmark view.
func (v *BenchmarkView) Update(msg tea.Msg) (Component, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height

	case DomainSubmitMsg:
		v.domain = msg.Domain
		v.loading = true
		v.err = nil
		v.results = nil
		return v, tea.Batch(v.spinner.Tick, v.runBenchmark())

	case BenchmarkResultMsg:
		v.loading = false
		if msg.Err != nil {
			v.err = msg.Err
		} else {
			v.results = msg.Results
			sort.Slice(v.results, func(i, j int) bool {
				// Successful results first, sorted by latency
				if v.results[i].Success != v.results[j].Success {
					return v.results[i].Success
				}
				return v.results[i].LatencyMs < v.results[j].LatencyMs
			})
			saveToHistory(v.config, "benchmark", v.domain, "", "local", msg.Results)
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

// runBenchmark returns a command that performs the DNS benchmark.
func (v *BenchmarkView) runBenchmark() tea.Cmd {
	domain := v.domain
	return func() tea.Msg {
		results := dns.Benchmark(domain, dns.DefaultResolvers)
		return BenchmarkResultMsg{Results: results}
	}
}

// View renders the benchmark view.
func (v *BenchmarkView) View(width, height int) string {
	v.width = width
	v.height = height

	if v.loading {
		return v.benchRenderLoading()
	}
	if v.err != nil {
		return v.benchRenderError()
	}
	if v.domain == "" || v.results == nil {
		return v.benchRenderEmpty()
	}

	var sections []string
	sections = append(sections, v.benchRenderHeader())
	sections = append(sections, "")
	sections = append(sections, v.benchRenderBarChart())
	sections = append(sections, "")

	// Statistics and sparkline side by side if there's room
	stats := v.benchRenderStatistics()
	sparkline := v.benchRenderSparkline()

	if v.width > 80 {
		gap := lipgloss.NewStyle().Width(3).Render("")
		sections = append(sections, lipgloss.JoinHorizontal(lipgloss.Top, stats, gap, sparkline))
	} else {
		sections = append(sections, stats)
		sections = append(sections, "")
		sections = append(sections, sparkline)
	}

	content := lipgloss.JoinVertical(lipgloss.Center, sections...)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Top, content)
}

// benchRenderLoading shows the spinner with a benchmark message.
func (v *BenchmarkView) benchRenderLoading() string {
	msg := fmt.Sprintf("Benchmarking %d resolvers...", len(dns.DefaultResolvers))
	return renderLoadingSpinner(v.spinner.View(), msg, v.width, v.height)
}

// benchRenderEmpty shows the placeholder state.
func (v *BenchmarkView) benchRenderEmpty() string {
	return renderEmptyState("Enter a domain and press Enter to benchmark DNS resolvers", v.width, v.height)
}

// benchRenderError shows the error state.
func (v *BenchmarkView) benchRenderError() string {
	return renderErrorBox(v.err, v.width, v.height)
}

// benchRenderHeader renders the domain and resolver count.
func (v *BenchmarkView) benchRenderHeader() string {
	titleStyle := lipgloss.NewStyle().Foreground(theme.ColorRose).Bold(true)
	domainStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite).Bold(true)
	countStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray)

	title := titleStyle.Render("DNS Benchmark")
	domain := domainStyle.Render(v.domain)
	count := countStyle.Render(fmt.Sprintf("%d resolvers", len(v.results)))

	header := lipgloss.JoinHorizontal(lipgloss.Center,
		title,
		lipgloss.NewStyle().Width(3).Render(""),
		domain,
		lipgloss.NewStyle().Width(2).Render(""),
		count,
	)

	return lipgloss.Place(v.width-4, 1, lipgloss.Center, lipgloss.Center, header)
}

// benchRenderBarChart renders the horizontal bar chart - the signature visual.
func (v *BenchmarkView) benchRenderBarChart() string {
	if len(v.results) == 0 {
		return ""
	}

	// Find max latency for scaling
	var maxLatency int64
	for _, r := range v.results {
		if r.Success && r.LatencyMs > maxLatency {
			maxLatency = r.LatencyMs
		}
	}
	if maxLatency == 0 {
		maxLatency = 1
	}

	// Calculate dimensions
	nameWidth := 22
	chartBoxWidth := v.width - 8
	if chartBoxWidth < 50 {
		chartBoxWidth = 50
	}
	if chartBoxWidth > 90 {
		chartBoxWidth = 90
	}
	innerWidth := chartBoxWidth - 4
	barMaxWidth := innerWidth - nameWidth - 10 // room for name + latency label
	if barMaxWidth < 10 {
		barMaxWidth = 10
	}

	titleStyle := lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true)
	borderColor := lipgloss.NewStyle().Foreground(theme.ColorSlate600)

	// Build the chart title embedded in the top border
	titleStr := titleStyle.Render("DNS Resolver Latency")
	titleLen := lipgloss.Width(titleStr)
	topDashes := maxInt(0, innerWidth-titleLen-1)

	topLine := fmt.Sprintf("╭─ %s %s╮", titleStr, borderColor.Render(strings.Repeat("─", topDashes)))
	bottomLine := borderColor.Render("╰" + strings.Repeat("─", innerWidth+2) + "╯")

	// Blank line inside box
	blankLine := borderColor.Render("│") +
		strings.Repeat(" ", innerWidth+2) +
		borderColor.Render("│")

	var barLines []string
	barLines = append(barLines, blankLine)

	nameStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite).Width(nameWidth).Align(lipgloss.Right)
	failStyle := lipgloss.NewStyle().Foreground(theme.ColorRose)
	latencyStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray)

	for _, r := range v.results {
		name := r.Resolver.Name
		if len(name) > nameWidth-1 {
			name = name[:nameWidth-4] + "..."
		}

		if !r.Success {
			// Failed resolver
			errText := r.Error
			if len(errText) > barMaxWidth-4 {
				errText = errText[:barMaxWidth-7] + "..."
			}
			line := fmt.Sprintf("%s  %s",
				nameStyle.Render(name),
				failStyle.Render("x "+errText),
			)
			padded := lipgloss.NewStyle().Width(innerWidth+2).Render(line)
			barLines = append(barLines,
				borderColor.Render("│")+padded+borderColor.Render("│"),
			)
			continue
		}

		// Calculate bar length
		barLen := int((r.LatencyMs * int64(barMaxWidth)) / maxLatency)
		if barLen < 1 {
			barLen = 1
		}
		emptyLen := barMaxWidth - barLen

		// Color gradient based on position relative to max
		ratio := float64(r.LatencyMs) / float64(maxLatency)
		barColor := benchLatencyColor(ratio)

		filledStyle := lipgloss.NewStyle().Foreground(barColor)
		emptyBarStyle := lipgloss.NewStyle().Foreground(theme.ColorSlate800)

		bar := filledStyle.Render(strings.Repeat("█", barLen)) +
			emptyBarStyle.Render(strings.Repeat("░", emptyLen))

		latencyLabel := latencyStyle.Render(fmt.Sprintf(" %dms", r.LatencyMs))

		line := fmt.Sprintf("%s  %s%s",
			nameStyle.Render(name),
			bar,
			latencyLabel,
		)

		padded := lipgloss.NewStyle().Width(innerWidth+2).Render(line)
		barLines = append(barLines,
			borderColor.Render("│")+padded+borderColor.Render("│"),
		)
	}

	barLines = append(barLines, blankLine)

	body := strings.Join(barLines, "\n")
	return lipgloss.JoinVertical(lipgloss.Left, topLine, body, bottomLine)
}

// benchRenderStatistics renders the stats box.
func (v *BenchmarkView) benchRenderStatistics() string {
	var successful []int64
	total := len(v.results)
	successCount := 0

	for _, r := range v.results {
		if r.Success {
			successful = append(successful, r.LatencyMs)
			successCount++
		}
	}

	if len(successful) == 0 {
		return ""
	}

	sort.Slice(successful, func(i, j int) bool { return successful[i] < successful[j] })

	fastest := successful[0]
	slowest := successful[len(successful)-1]

	var sum int64
	for _, ms := range successful {
		sum += ms
	}
	avg := sum / int64(len(successful))

	var median int64
	n := len(successful)
	if n%2 == 0 {
		median = (successful[n/2-1] + successful[n/2]) / 2
	} else {
		median = successful[n/2]
	}

	pct := (successCount * 100) / total

	// Find names for fastest/slowest
	var fastestName, slowestName string
	for _, r := range v.results {
		if r.Success && r.LatencyMs == fastest && fastestName == "" {
			fastestName = r.Resolver.Name
		}
		if r.Success && r.LatencyMs == slowest && slowestName == "" {
			slowestName = r.Resolver.Name
		}
	}

	labelStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray)
	valueStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite).Bold(true)
	fastStyle := lipgloss.NewStyle().Foreground(theme.ColorEmerald).Bold(true)
	slowStyle := lipgloss.NewStyle().Foreground(theme.ColorRose).Bold(true)
	titleStyle := lipgloss.NewStyle().Foreground(theme.ColorViolet).Bold(true)
	borderColor := lipgloss.NewStyle().Foreground(theme.ColorSlate600)

	boxWidth := 48
	innerWidth := boxWidth - 4

	titleStr := titleStyle.Render("Statistics")
	titleLen := lipgloss.Width(titleStr)
	topDashes := maxInt(0, innerWidth-titleLen-1)

	topLine := fmt.Sprintf("╭─ %s %s╮", titleStr, borderColor.Render(strings.Repeat("─", topDashes)))
	bottomLine := borderColor.Render("╰" + strings.Repeat("─", innerWidth+2) + "╯")

	statLine := func(label string, name string, val string, style lipgloss.Style) string {
		nameStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray)
		// Truncate long resolver names
		if len(name) > 18 {
			name = name[:15] + "..."
		}
		content := fmt.Sprintf("  %s  %s  %s",
			labelStyle.Width(10).Render(label),
			nameStyle.Width(18).Render(name),
			style.Render(val),
		)
		padded := lipgloss.NewStyle().Width(innerWidth + 2).Render(content)
		return borderColor.Render("│") + padded + borderColor.Render("│")
	}

	blankLine := borderColor.Render("│") + strings.Repeat(" ", innerWidth+2) + borderColor.Render("│")

	lines := []string{
		topLine,
		statLine("Fastest:", fastestName, fmt.Sprintf("%dms", fastest), fastStyle),
		statLine("Slowest:", slowestName, fmt.Sprintf("%dms", slowest), slowStyle),
		statLine("Average:", "", fmt.Sprintf("%dms", avg), valueStyle),
		statLine("Median:", "", fmt.Sprintf("%dms", median), valueStyle),
		blankLine,
		borderColor.Render("│") +
			lipgloss.NewStyle().Width(innerWidth+2).Render(
				fmt.Sprintf("  %s %s",
					labelStyle.Render("Success:"),
					valueStyle.Render(fmt.Sprintf("%d/%d (%d%%)", successCount, total, pct)),
				),
			) +
			borderColor.Render("│"),
		bottomLine,
	}

	return strings.Join(lines, "\n")
}

// benchRenderSparkline renders a latency distribution sparkline.
func (v *BenchmarkView) benchRenderSparkline() string {
	var successful []int64
	for _, r := range v.results {
		if r.Success {
			successful = append(successful, r.LatencyMs)
		}
	}

	if len(successful) < 2 {
		return ""
	}

	sort.Slice(successful, func(i, j int) bool { return successful[i] < successful[j] })

	minVal := successful[0]
	maxVal := successful[len(successful)-1]
	spread := maxVal - minVal
	if spread == 0 {
		spread = 1
	}

	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	var sparkRunes []string
	for _, ms := range successful {
		ratio := float64(ms-minVal) / float64(spread)
		idx := int(ratio * float64(len(blocks)-1))
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}

		// Color each block based on its height
		blockRatio := float64(idx) / float64(len(blocks)-1)
		color := benchLatencyColor(blockRatio)
		sparkRunes = append(sparkRunes,
			lipgloss.NewStyle().Foreground(color).Render(string(blocks[idx])),
		)
	}

	labelStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray)
	return labelStyle.Render("Latency distribution: ") + strings.Join(sparkRunes, "")
}

// benchLatencyColor returns a color on the emerald-amber-rose gradient based on ratio (0.0=fast, 1.0=slow).
func benchLatencyColor(ratio float64) lipgloss.Color {
	if ratio < 0.33 {
		return theme.ColorEmerald
	}
	if ratio < 0.66 {
		return theme.ColorAmber
	}
	return theme.ColorRose
}

