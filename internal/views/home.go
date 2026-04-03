package views

import (
	"fmt"
	"net"
	"strings"

	"github.com/Vector-DNS/vdns-tui/internal/config"
	"github.com/Vector-DNS/vdns-tui/internal/dns"
	"github.com/Vector-DNS/vdns-tui/internal/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HomeView is the landing/dashboard view shown when the TUI starts.
type HomeView struct {
	width  int
	height int
	config *config.Config
}

// NewHomeView creates a new home view with the given config.
func NewHomeView(cfg *config.Config) *HomeView {
	return &HomeView{
		config: cfg,
	}
}

// Update handles messages for the home view.
func (v *HomeView) Update(msg tea.Msg) (Component, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
	case DomainSubmitMsg:
		return v, func() tea.Msg {
			return StatusMsg("Press 1 or switch to Lookup view to query " + msg.Domain)
		}
	}
	return v, nil
}

// View renders the home dashboard.
func (v *HomeView) View(width, height int) string {
	v.width = width
	v.height = height

	var sections []string

	sections = append(sections, v.renderLogo())
	sections = append(sections, v.renderTagline())
	sections = append(sections, v.renderGradientDivider())
	sections = append(sections, v.renderStatusCards())
	sections = append(sections, "")
	sections = append(sections, v.renderQuickActions())
	sections = append(sections, "")
	sections = append(sections, v.renderFeatureGrid())
	sections = append(sections, "")
	sections = append(sections, v.renderSystemInfo())
	sections = append(sections, "")
	sections = append(sections, v.renderTip())
	sections = append(sections, v.renderVersion())

	content := lipgloss.JoinVertical(lipgloss.Center, sections...)

	return lipgloss.Place(width, height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

// renderLogo renders the brand name with styled text.
func (v *HomeView) renderLogo() string {
	vector := lipgloss.NewStyle().
		Foreground(theme.ColorPrimaryBlue).
		Bold(true).
		Render("Vector")
	dns := lipgloss.NewStyle().
		Foreground(theme.ColorAccentCyan).
		Bold(true).
		Render("DNS")
	diamond := lipgloss.NewStyle().
		Foreground(theme.ColorViolet).
		Bold(true).
		Render("◆")

	return diamond + " " + vector + dns
}

// renderTagline renders the subtitle below the logo.
func (v *HomeView) renderTagline() string {
	style := lipgloss.NewStyle().Foreground(theme.ColorCoolGray).Italic(true)
	return style.Render("DNS Toolkit for your terminal")
}

// renderStatusCards renders mode, profile, and server info in bordered cards.
func (v *HomeView) renderStatusCards() string {
	cfg := v.config

	// Mode card
	var modeLabel string
	var modeStyle lipgloss.Style
	if cfg.ShouldUseRemote() {
		modeLabel = "REMOTE"
		modeStyle = lipgloss.NewStyle().
			Foreground(theme.ColorMidnight).
			Background(theme.ColorEmerald).
			Bold(true).
			Padding(0, 1)
	} else {
		modeLabel = "LOCAL"
		modeStyle = lipgloss.NewStyle().
			Foreground(theme.ColorMidnight).
			Background(theme.ColorAmber).
			Bold(true).
			Padding(0, 1)
	}

	modeCard := v.statusCard("Mode", modeStyle.Render(modeLabel))

	// Profile card
	profile := cfg.ActiveProfile
	if profile == "" {
		profile = "default"
	}
	profileStyle := lipgloss.NewStyle().Foreground(theme.ColorViolet).Bold(true)
	profileCard := v.statusCard("Profile", profileStyle.Render(profile))

	// Server card
	var serverText string
	if cfg.ShouldUseRemote() {
		serverText = cfg.ActiveServer()
	} else {
		serverText = "local resolver"
	}
	serverStyle := lipgloss.NewStyle().Foreground(theme.ColorAccentCyan)
	serverCard := v.statusCard("Server", serverStyle.Render(serverText))

	// Stack vertically on narrow terminals
	if v.width < 80 {
		return lipgloss.JoinVertical(lipgloss.Center, modeCard, profileCard, serverCard)
	}

	gap := lipgloss.NewStyle().Width(2).Render("")
	return lipgloss.JoinHorizontal(lipgloss.Center, modeCard, gap, profileCard, gap, serverCard)
}

// statusCard renders a single status card with a label and value.
func (v *HomeView) statusCard(label, value string) string {
	labelStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray)
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorSlate600).
		Padding(0, 2).
		Width(28).
		Align(lipgloss.Center)

	content := lipgloss.JoinVertical(lipgloss.Center,
		labelStyle.Render(label),
		value,
	)
	return cardStyle.Render(content)
}

// renderQuickActions renders the quick start keybinding reference box.
func (v *HomeView) renderQuickActions() string {
	keyStyle := lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite)

	actions := []struct{ key, desc string }{
		{"/", "Search for a domain"},
		{"Up/Down", "Navigate views"},
		{"Enter", "Select view"},
		{"?", "Show all shortcuts"},
		{"q", "Quit"},
	}

	var lines []string
	for _, a := range actions {
		line := fmt.Sprintf("  %s  %s",
			keyStyle.Width(12).Render(a.key),
			descStyle.Render(a.desc),
		)
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")

	titleStyle := lipgloss.NewStyle().
		Foreground(theme.ColorPrimaryBlue).
		Bold(true)

	boxWidth := 50
	if v.width-4 < boxWidth {
		boxWidth = v.width - 4
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorSlate600).
		Padding(1, 2).
		Width(boxWidth)

	fullContent := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("Quick Start"),
		"",
		content,
	)

	return boxStyle.Render(fullContent)
}

// renderFeatureGrid renders the available features in a vertical list with
// icon, name, mode badge, and description.
func (v *HomeView) renderFeatureGrid() string {
	loggedIn := v.config.IsLoggedIn()

	type feature struct {
		icon   string
		name   string
		color  lipgloss.Color
		remote bool
		desc   string
	}

	features := []feature{
		{"⌕", "DNS Lookup", theme.ColorPrimaryBlue, false, "Query DNS records for any domain"},
		{"◎", "Propagation", theme.ColorAccentCyan, false, "Check DNS propagation across resolvers"},
		{"≡", "WHOIS", theme.ColorViolet, true, "Domain registration information"},
		{"◇", "Availability", theme.ColorEmerald, true, "Check if a domain is available"},
		{"⚷", "SSL Certificate", theme.ColorAmber, true, "Inspect SSL/TLS certificates"},
		{"⚡", "Benchmark", theme.ColorRose, false, "Compare DNS resolver performance"},
	}

	showDesc := v.width >= 80

	var lines []string
	for _, f := range features {
		iconStyle := lipgloss.NewStyle().Foreground(f.color).Bold(true)
		nameStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite).Width(18)

		// Mode badge
		var badge string
		if f.remote {
			badgeColor := theme.ColorSlate600
			if loggedIn {
				badgeColor = theme.ColorEmerald
			}
			badge = lipgloss.NewStyle().Foreground(badgeColor).Render("remote")
		} else {
			badge = lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Render("local ")
		}

		badgeStyle := lipgloss.NewStyle().Width(10)
		descStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray)

		line := fmt.Sprintf("  %s %s %s",
			iconStyle.Render(f.icon),
			nameStyle.Render(f.name),
			badgeStyle.Render(badge),
		)

		if showDesc {
			line += descStyle.Render(f.desc)
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderSystemInfo renders resolver and system information at the bottom.
func (v *HomeView) renderSystemInfo() string {
	resolver := dns.GetSystemResolver()
	isDefault := dns.IsDefaultResolver()
	resolverCount := len(dns.DefaultResolvers)

	labelStyle := lipgloss.NewStyle().Foreground(theme.ColorSlate600)
	valueStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray)

	host, _, err := net.SplitHostPort(resolver)
	if err != nil {
		host = resolver
	}

	resolverText := host
	if isDefault {
		resolverText += " (fallback)"
	}

	info := fmt.Sprintf("%s %s   %s %s",
		labelStyle.Render("Resolver:"),
		valueStyle.Render(resolverText),
		labelStyle.Render("Public resolvers:"),
		valueStyle.Render(fmt.Sprintf("%d", resolverCount)),
	)

	divider := lipgloss.NewStyle().
		Foreground(theme.ColorSlate800).
		Width(44).
		Align(lipgloss.Center).
		Render(strings.Repeat("─", 36))

	return lipgloss.JoinVertical(lipgloss.Center, divider, info)
}

// renderGradientDivider renders a subtle horizontal rule.
func (v *HomeView) renderGradientDivider() string {
	return lipgloss.NewStyle().Foreground(theme.ColorSlate600).Render(strings.Repeat("─", 30))
}

// renderTip renders a helpful tip line below the system info.
func (v *HomeView) renderTip() string {
	labelStyle := lipgloss.NewStyle().Foreground(theme.ColorSlate600)
	textStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray).Italic(true)
	return labelStyle.Render("Tip: ") + textStyle.Render("Press / to search for a domain, then Enter to look it up")
}

// renderVersion renders the version indicator at the bottom.
func (v *HomeView) renderVersion() string {
	style := lipgloss.NewStyle().Foreground(theme.ColorSlate600)
	return style.Render("vdns-tui v0.1.0")
}
