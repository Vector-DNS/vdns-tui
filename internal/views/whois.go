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

// WhoisView displays WHOIS information for a domain (remote only).
type WhoisView struct {
	width, height int
	domain        string
	loading       bool
	spinner       spinner.Model
	result        *client.WhoisResponse
	err           error
	config        *config.Config
	apiClient     *client.Client
}

// NewWhoisView creates a new WHOIS view.
func NewWhoisView(cfg *config.Config, apiClient *client.Client) *WhoisView {
	s := newSpinner()

	return &WhoisView{
		config:    cfg,
		apiClient: apiClient,
		spinner:   s,
	}
}

// Update handles messages for the WHOIS view.
func (v *WhoisView) Update(msg tea.Msg) (Component, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height

	case DomainSubmitMsg:
		if !v.config.IsLoggedIn() {
			return v, func() tea.Msg {
				return StatusMsg("Login required - run vdns login to use WHOIS")
			}
		}
		v.domain = msg.Domain
		v.loading = true
		v.result = nil
		v.err = nil
		return v, tea.Batch(v.spinner.Tick, v.fetchWhois(msg.Domain))

	case WhoisResultMsg:
		v.loading = false
		v.result = msg.Result
		v.err = msg.Err
		if msg.Err == nil {
			saveToHistory(v.config, "whois", v.domain, "", "remote", msg.Result)
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

// fetchWhois returns a command that queries the WHOIS API.
func (v *WhoisView) fetchWhois(domain string) tea.Cmd {
	c := v.apiClient
	return func() tea.Msg {
		result, err := c.Whois(client.WhoisRequest{Domain: domain})
		return WhoisResultMsg{Result: result, Err: err}
	}
}

// View renders the WHOIS view.
func (v *WhoisView) View(width, height int) string {
	v.width = width
	v.height = height

	if !v.config.IsLoggedIn() {
		return LoginRequiredView(v.width, v.height, "WHOIS")
	}

	if v.loading {
		return v.whoisRenderLoading()
	}

	if v.err != nil {
		return v.whoisRenderError()
	}

	if v.result == nil {
		return v.whoisRenderEmpty()
	}

	return v.whoisRenderResult()
}

func (v *WhoisView) whoisRenderLoading() string {
	return renderLoadingSpinner(v.spinner.View(), "Looking up WHOIS for "+v.domain+"...", v.width, v.height)
}

func (v *WhoisView) whoisRenderError() string {
	return renderErrorBox(v.err, v.width, v.height)
}

func (v *WhoisView) whoisRenderEmpty() string {
	return renderEmptyState("Enter a domain name and press Enter to look up WHOIS data", v.width, v.height)
}

func (v *WhoisView) whoisRenderResult() string {
	r := v.result.Result

	labelStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray).Width(14)
	valueStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite)
	naStyle := lipgloss.NewStyle().Foreground(theme.ColorSlate600).Italic(true)
	sectionStyle := lipgloss.NewStyle().Foreground(theme.ColorSlate600)

	boxWidth := 52
	dividerWidth := boxWidth - 6
	if dividerWidth < 10 {
		dividerWidth = 10
	}

	val := func(s *string) string {
		if s != nil && *s != "" {
			return valueStyle.Render(*s)
		}
		return naStyle.Render("N/A")
	}

	// Build rows
	var lines []string
	lines = append(lines, "")

	// Registration section
	regDivider := fmt.Sprintf("  %s %s %s",
		sectionStyle.Render("──"),
		lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true).Render("Registration"),
		sectionStyle.Render(strings.Repeat("─", dividerWidth-19)),
	)
	lines = append(lines, regDivider)

	lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render("Registrar"), val(r.Registrar)))
	lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render("DNSSEC"), val(r.DNSSEC)))

	// Domain age
	if r.CreatedDate != nil {
		if age := whoisDomainAge(*r.CreatedDate); age != "" {
			ageStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray).Italic(true)
			lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render("Registered"), ageStyle.Render(age+" ago")))
		}
	}

	lines = append(lines, "")

	// Dates section
	datesDivider := fmt.Sprintf("  %s %s %s",
		sectionStyle.Render("──"),
		lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true).Render("Dates"),
		sectionStyle.Render(strings.Repeat("─", dividerWidth-11)),
	)
	lines = append(lines, datesDivider)

	lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render("Created"), whoisFormatDate(r.CreatedDate, valueStyle, naStyle)))
	lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render("Updated"), whoisFormatDate(r.UpdatedDate, valueStyle, naStyle)))

	// Expiry with warning color
	expiryVal := val(r.ExpiresDate)
	if r.ExpiresDate != nil {
		expiryVal = v.whoisColorExpiry(*r.ExpiresDate)
	}
	lines = append(lines, fmt.Sprintf("  %s %s", labelStyle.Render("Expires"), expiryVal))

	lines = append(lines, "")

	// Nameservers section
	if len(r.Nameservers) > 0 {
		nsDivider := fmt.Sprintf("  %s %s %s",
			sectionStyle.Render("──"),
			lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true).Render("Nameservers"),
			sectionStyle.Render(strings.Repeat("─", dividerWidth-18)),
		)
		lines = append(lines, nsDivider)
		for _, ns := range r.Nameservers {
			lines = append(lines, fmt.Sprintf("    %s", valueStyle.Render(ns)))
		}
		lines = append(lines, "")
	}

	// Status section with color-coded badges
	if len(r.Status) > 0 {
		statusDivider := fmt.Sprintf("  %s %s %s",
			sectionStyle.Render("──"),
			lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true).Render("Status"),
			sectionStyle.Render(strings.Repeat("─", dividerWidth-12)),
		)
		lines = append(lines, statusDivider)
		for _, s := range r.Status {
			lines = append(lines, fmt.Sprintf("    %s", whoisColorStatus(s)))
		}
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")

	// Build box with title inside
	titleStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimaryBlue).Bold(true)
	domainStyle := lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true)

	titleLine := titleStyle.Render("WHOIS") + " " + sectionStyle.Render("─") + " " + domainStyle.Render(v.domain)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorSlate600).
		Padding(0, 2).
		Width(boxWidth)

	box := boxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, titleLine, content))
	return lipgloss.Place(v.width, v.height, lipgloss.Center, lipgloss.Center, box)
}

// whoisDateFormats are common date formats returned by WHOIS APIs.
var whoisDateFormats = []string{
	time.RFC3339,
	"2006-01-02T15:04:05Z",
	"2006-01-02",
	"2006-01-02 15:04:05",
}

// whoisFormatDate formats a date string to a cleaner display format.
func whoisFormatDate(dateStr *string, valueStyle, naStyle lipgloss.Style) string {
	if dateStr == nil || *dateStr == "" {
		return naStyle.Render("N/A")
	}
	for _, f := range whoisDateFormats {
		if parsed, err := time.Parse(f, *dateStr); err == nil {
			return valueStyle.Render(parsed.Format("02 Jan 2006"))
		}
	}
	return valueStyle.Render(*dateStr)
}

// whoisColorExpiry returns a styled expiry date, colored by how soon it expires.
func (v *WhoisView) whoisColorExpiry(dateStr string) string {
	var parsed time.Time
	var ok bool
	for _, f := range whoisDateFormats {
		var err error
		parsed, err = time.Parse(f, dateStr)
		if err == nil {
			ok = true
			break
		}
	}

	if ok {
		display := parsed.Format("02 Jan 2006")
		days := int(time.Until(parsed).Hours() / 24)
		switch {
		case days < 0:
			style := lipgloss.NewStyle().Foreground(theme.ColorRose).Bold(true)
			return style.Render(display) + " " + style.Render("(EXPIRED)")
		case days <= 7:
			style := lipgloss.NewStyle().Foreground(theme.ColorRose).Bold(true)
			return style.Render(display) + " " + style.Render(fmt.Sprintf("(%dd remaining!)", days))
		case days <= 30:
			style := lipgloss.NewStyle().Foreground(theme.ColorAmber).Bold(true)
			return style.Render(display) + " " + style.Render(fmt.Sprintf("(%dd remaining)", days))
		default:
			return lipgloss.NewStyle().Foreground(theme.ColorWhite).Render(display)
		}
	}

	return lipgloss.NewStyle().Foreground(theme.ColorWhite).Render(dateStr)
}

// whoisColorStatus returns a color-coded status string.
func whoisColorStatus(status string) string {
	lower := strings.ToLower(status)

	var color lipgloss.Color
	switch {
	case strings.Contains(lower, "prohibit"):
		color = theme.ColorRose
	case strings.Contains(lower, "lock"):
		color = theme.ColorAmber
	default:
		color = theme.ColorCoolGray
	}

	return lipgloss.NewStyle().Foreground(color).Render(status)
}

// whoisDomainAge computes a human-readable age from a date string.
func whoisDomainAge(dateStr string) string {
	for _, f := range whoisDateFormats {
		if parsed, err := time.Parse(f, dateStr); err == nil {
			years := int(time.Since(parsed).Hours() / 24 / 365)
			months := int(time.Since(parsed).Hours()/24/30) % 12
			if years > 0 {
				return fmt.Sprintf("%d years, %d months", years, months)
			}
			if months > 0 {
				return fmt.Sprintf("%d months", months)
			}
			days := int(time.Since(parsed).Hours() / 24)
			return fmt.Sprintf("%d days", days)
		}
	}
	return ""
}

// LoginRequiredView renders the shared "login required" message box.
// Exported so other remote-only views can reuse it.
func LoginRequiredView(width, height int, feature string) string {
	titleStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimaryBlue).Bold(true)
	textStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray)
	cmdStyle := lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true)

	content := lipgloss.JoinVertical(lipgloss.Center,
		"",
		titleStyle.Render("Remote Access Required"),
		"",
		textStyle.Render("This feature requires a VectorDNS API key."),
		"",
		textStyle.Render("Run ")+cmdStyle.Render("vdns login")+textStyle.Render(" in your terminal to set up"),
		textStyle.Render("your API key, or set ")+cmdStyle.Render("VDNS_API_KEY")+textStyle.Render(" env var."),
		"",
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorPrimaryBlue).
		Padding(1, 3).
		Width(52).
		Align(lipgloss.Center).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
