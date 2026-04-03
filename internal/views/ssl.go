package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/Vector-DNS/vdns-tui/internal/client"
	"github.com/Vector-DNS/vdns-tui/internal/config"
	"github.com/Vector-DNS/vdns-tui/internal/theme"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SSLView displays SSL certificate information for a domain (remote only).
type SSLView struct {
	width, height int
	domain        string
	loading       bool
	spinner       spinner.Model
	gauge         progress.Model
	result        *client.SSLResponse
	err           error
	config        *config.Config
	apiClient     *client.Client
}

// NewSSLView creates a new SSL view.
func NewSSLView(cfg *config.Config, apiClient *client.Client) *SSLView {
	s := newSpinner()

	g := progress.New(
		progress.WithWidth(36),
		progress.WithoutPercentage(),
	)

	return &SSLView{
		config:    cfg,
		apiClient: apiClient,
		spinner:   s,
		gauge:     g,
	}
}

// Update handles messages for the SSL view.
func (v *SSLView) Update(msg tea.Msg) (Component, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height

	case DomainSubmitMsg:
		if !v.config.IsLoggedIn() {
			return v, func() tea.Msg {
				return StatusMsg("Login required - run vdns login to use SSL check")
			}
		}
		v.domain = msg.Domain
		v.loading = true
		v.result = nil
		v.err = nil
		return v, tea.Batch(v.spinner.Tick, v.fetchSSL(msg.Domain))

	case SSLResultMsg:
		v.loading = false
		v.result = msg.Result
		v.err = msg.Err
		if msg.Err == nil {
			saveToHistory(v.config, "ssl", v.domain, "", "remote", msg.Result)
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

// fetchSSL returns a command that queries the SSL API.
func (v *SSLView) fetchSSL(domain string) tea.Cmd {
	c := v.apiClient
	return func() tea.Msg {
		result, err := c.SSL(domain)
		return SSLResultMsg{Result: result, Err: err}
	}
}

// View renders the SSL view.
func (v *SSLView) View(width, height int) string {
	v.width = width
	v.height = height

	if !v.config.IsLoggedIn() {
		return LoginRequiredView(width, height, "SSL")
	}

	if v.loading {
		return v.sslRenderLoading()
	}

	if v.err != nil {
		return v.sslRenderError()
	}

	if v.result == nil {
		return v.sslRenderEmpty()
	}

	return v.sslRenderResult()
}

func (v *SSLView) sslRenderLoading() string {
	return renderLoadingSpinner(v.spinner.View(), "Checking SSL certificate for "+v.domain+"...", v.width, v.height)
}

func (v *SSLView) sslRenderError() string {
	return renderErrorBox(v.err, v.width, v.height)
}

func (v *SSLView) sslRenderEmpty() string {
	return renderEmptyState("Enter a domain name and press Enter to check its SSL certificate", v.width, v.height)
}

func (v *SSLView) sslRenderResult() string {
	r := v.result

	labelStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray).Width(14)
	valueStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite)
	naStyle := lipgloss.NewStyle().Foreground(theme.ColorSlate600).Italic(true)
	sectionStyle := lipgloss.NewStyle().Foreground(theme.ColorSlate600)

	boxWidth := 52

	val := func(s *string) string {
		if s != nil && *s != "" {
			return valueStyle.Render(*s)
		}
		return naStyle.Render("N/A")
	}

	var lines []string
	lines = append(lines, "")

	// Status badge
	var statusColor lipgloss.Color
	var statusIcon, statusText string
	if r.IsValid {
		statusColor = theme.ColorEmerald
		statusIcon = "●"
		statusText = "VALID"
	} else {
		statusColor = theme.ColorRose
		statusIcon = "●"
		statusText = "INVALID"
	}

	badgeStyle := lipgloss.NewStyle().
		Foreground(statusColor).
		Bold(true)

	badge := fmt.Sprintf("  %s %s", badgeStyle.Render(statusIcon+" "+statusText), "")
	lines = append(lines, badge)

	// Show error if present
	if r.Error != nil && *r.Error != "" {
		errStyle := lipgloss.NewStyle().Foreground(theme.ColorRose)
		lines = append(lines, fmt.Sprintf("  %s", errStyle.Render(*r.Error)))
	}

	lines = append(lines, "")

	// Certificate details section
	dividerWidth := boxWidth - 6
	if dividerWidth < 10 {
		dividerWidth = 10
	}
	certDivider := fmt.Sprintf("  %s %s %s",
		sectionStyle.Render("──"),
		lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true).Render("Certificate"),
		sectionStyle.Render(strings.Repeat("─", dividerWidth-18)),
	)
	lines = append(lines, certDivider)

	lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render("Subject"), val(r.Subject)))
	lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render("Issuer"), val(r.Issuer)))
	lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render("Protocol"), v.sslColorProtocol(r.Protocol)))
	lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render("Chain"), valueStyle.Render(fmt.Sprintf("%d certificates", r.ChainLength))))

	lines = append(lines, "")

	// Validity section
	validityDivider := fmt.Sprintf("  %s %s %s",
		sectionStyle.Render("──"),
		lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true).Render("Validity"),
		sectionStyle.Render(strings.Repeat("─", dividerWidth-16)),
	)
	lines = append(lines, validityDivider)

	timeVal := func(t *time.Time) string {
		if t != nil {
			return valueStyle.Render(t.Format("02 Jan 2006"))
		}
		return naStyle.Render("N/A")
	}
	lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render("Valid From"), timeVal(r.ValidFrom)))
	lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render("Expires At"), timeVal(r.ExpiresAt)))

	// Remaining days + progress gauge
	if r.ValidFrom != nil && r.ExpiresAt != nil {
		daysRemaining := int(time.Until(*r.ExpiresAt).Hours() / 24)
		remainingLabel := fmt.Sprintf("%d days", daysRemaining)

		var remainColor lipgloss.Color
		switch {
		case daysRemaining < 0:
			remainColor = theme.ColorRose
			remainingLabel = "EXPIRED"
		case daysRemaining < 30:
			remainColor = theme.ColorRose
		case daysRemaining <= 60:
			remainColor = theme.ColorAmber
		default:
			remainColor = theme.ColorEmerald
		}

		remainStyle := lipgloss.NewStyle().Foreground(remainColor).Bold(true)
		lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render("Remaining"), remainStyle.Render(remainingLabel)))
		lines = append(lines, "")

		// Progress gauge using bubbles/progress
		totalDays := r.ExpiresAt.Sub(*r.ValidFrom).Hours() / 24
		var pct float64
		if totalDays > 0 {
			pct = float64(daysRemaining) / totalDays
		}
		if pct < 0 {
			pct = 0
		}
		if pct > 1 {
			pct = 1
		}

		v.gauge.FullColor = string(remainColor)
		v.gauge.EmptyColor = string(theme.ColorSlate600)

		pctLabel := lipgloss.NewStyle().Foreground(remainColor).Render(fmt.Sprintf(" %d%%", int(pct*100)))
		lines = append(lines, "  "+v.gauge.ViewAs(pct)+pctLabel)
	}

	lines = append(lines, "")

	content := strings.Join(lines, "\n")

	// Build box with title inside
	titleStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimaryBlue).Bold(true)
	domainStyle := lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true)

	titleLine := titleStyle.Render("SSL Certificate") + " " + sectionStyle.Render("─") + " " + domainStyle.Render(v.domain)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorSlate600).
		Padding(0, 2).
		Width(boxWidth)

	box := boxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, titleLine, content))
	return lipgloss.Place(v.width, v.height, lipgloss.Center, lipgloss.Center, box)
}

// sslColorProtocol returns a color-styled protocol string.
func (v *SSLView) sslColorProtocol(protocol *string) string {
	if protocol == nil || *protocol == "" {
		return lipgloss.NewStyle().Foreground(theme.ColorSlate600).Italic(true).Render("N/A")
	}

	p := *protocol
	var color lipgloss.Color
	switch {
	case strings.Contains(p, "1.3"):
		color = theme.ColorEmerald
	case strings.Contains(p, "1.2"):
		color = theme.ColorAmber
	default:
		color = theme.ColorRose
	}

	return lipgloss.NewStyle().Foreground(color).Bold(true).Render(p)
}
