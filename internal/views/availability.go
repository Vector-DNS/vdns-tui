package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/Vector-DNS/vdns-tui/internal/client"
	"github.com/Vector-DNS/vdns-tui/internal/config"
	"github.com/Vector-DNS/vdns-tui/internal/theme"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AvailabilityView displays domain availability information (remote only).
type AvailabilityView struct {
	width, height int
	domain        string
	loading       bool
	spinner       spinner.Model
	result        *client.AvailabilityResponse
	err           error
	config        *config.Config
	apiClient     *client.Client
}

// NewAvailabilityView creates a new availability view.
func NewAvailabilityView(cfg *config.Config, apiClient *client.Client) *AvailabilityView {
	s := newSpinner()

	return &AvailabilityView{
		config:    cfg,
		apiClient: apiClient,
		spinner:   s,
	}
}

// Update handles messages for the availability view.
func (v *AvailabilityView) Update(msg tea.Msg) (Component, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height

	case DomainSubmitMsg:
		if !v.config.IsLoggedIn() {
			return v, func() tea.Msg {
				return StatusMsg("Login required - run vdns login to check availability")
			}
		}
		v.domain = msg.Domain
		v.loading = true
		v.result = nil
		v.err = nil
		return v, tea.Batch(v.spinner.Tick, v.fetchAvailability(msg.Domain))

	case AvailabilityResultMsg:
		v.loading = false
		v.result = msg.Result
		v.err = msg.Err
		if msg.Err == nil {
			saveToHistory(v.config, "availability", v.domain, "", "remote", msg.Result)
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

// fetchAvailability returns a command that queries the availability API.
func (v *AvailabilityView) fetchAvailability(domain string) tea.Cmd {
	c := v.apiClient
	return func() tea.Msg {
		result, err := c.Availability(client.AvailabilityRequest{Domain: domain})
		return AvailabilityResultMsg{Result: result, Err: err}
	}
}

// View renders the availability view.
func (v *AvailabilityView) View(width, height int) string {
	v.width = width
	v.height = height

	if !v.config.IsLoggedIn() {
		return LoginRequiredView(width, height, "Availability")
	}

	if v.loading {
		return v.availRenderLoading()
	}

	if v.err != nil {
		return v.availRenderError()
	}

	if v.result == nil {
		return v.availRenderEmpty()
	}

	return v.availRenderResult()
}

func (v *AvailabilityView) availRenderLoading() string {
	return renderLoadingSpinner(v.spinner.View(), "Checking availability for "+v.domain+"...", v.width, v.height)
}

func (v *AvailabilityView) availRenderError() string {
	return renderErrorBox(v.err, v.width, v.height)
}

func (v *AvailabilityView) availRenderEmpty() string {
	return renderEmptyState("Enter a domain name and press Enter to check availability", v.width, v.height)
}

func (v *AvailabilityView) availRenderResult() string {
	r := v.result.Result

	sectionStyle := lipgloss.NewStyle().Foreground(theme.ColorSlate600)
	valueStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite)
	domainStyle := lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true)

	boxWidth := 52
	contentWidth := boxWidth - 6

	var lines []string
	lines = append(lines, "")
	lines = append(lines, "")

	// Domain name centered
	domainLine := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(
		domainStyle.Render(r.DomainName),
	)
	lines = append(lines, domainLine)
	lines = append(lines, "")

	// Result display
	var resultColor lipgloss.Color
	var resultIcon, resultText string
	if r.Available {
		resultColor = theme.ColorEmerald
		resultIcon = "●"
		resultText = "AVAILABLE"
	} else {
		resultColor = theme.ColorRose
		resultIcon = "●"
		resultText = "TAKEN"
	}

	resultStyle := lipgloss.NewStyle().Foreground(resultColor).Bold(true)
	borderStyle := lipgloss.NewStyle().Foreground(resultColor)

	innerWidth := 24
	label := fmt.Sprintf("%s  %s", resultIcon, resultText)
	paddedLabel := lipgloss.NewStyle().Width(innerWidth).Align(lipgloss.Center).Render(resultStyle.Render(label))

	resultBox := lipgloss.JoinVertical(lipgloss.Center,
		borderStyle.Render(fmt.Sprintf("╔%s╗", strings.Repeat("═", innerWidth+2))),
		borderStyle.Render("║ ")+paddedLabel+borderStyle.Render(" ║"),
		borderStyle.Render(fmt.Sprintf("╚%s╝", strings.Repeat("═", innerWidth+2))),
	)
	centered := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(resultBox)
	lines = append(lines, centered)

	lines = append(lines, "")

	// Queried-at timestamp
	if v.result.QueriedAt != "" {
		ts := v.result.QueriedAt
		if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
			ts = parsed.Format("02 Jan 2006 15:04 MST")
		}
		timeStyle := lipgloss.NewStyle().Foreground(theme.ColorSlate600).Italic(true)
		timeLine := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(
			timeStyle.Render("Checked "+ts),
		)
		lines = append(lines, timeLine)
		lines = append(lines, "")
	}

	// TLD suggestions for taken domains
	if !r.Available {
		altStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray).Italic(true)
		altLine := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(
			altStyle.Render("Try also: .net  .org  .io  .dev  .app"),
		)
		lines = append(lines, altLine)
		lines = append(lines, "")
	}

	// RDAP status section
	if len(r.RDAPStatus) > 0 {
		dividerWidth := contentWidth
		if dividerWidth < 10 {
			dividerWidth = 10
		}
		divider := fmt.Sprintf("  %s %s %s",
			sectionStyle.Render("──"),
			lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true).Render("RDAP Status"),
			sectionStyle.Render(strings.Repeat("─", dividerWidth-19)),
		)
		lines = append(lines, divider)
		for _, s := range r.RDAPStatus {
			lines = append(lines, fmt.Sprintf("    %s", valueStyle.Render(s)))
		}
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")

	// Build box with title inside
	titleStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimaryBlue).Bold(true)

	titleLine := titleStyle.Render("Domain Availability")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorSlate600).
		Padding(0, 2).
		Width(boxWidth)

	box := boxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, titleLine, content))
	return lipgloss.Place(v.width, v.height, lipgloss.Center, lipgloss.Center, box)
}
