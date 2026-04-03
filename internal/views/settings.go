package views

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Vector-DNS/vdns-tui/internal/config"
	"github.com/Vector-DNS/vdns-tui/internal/dns"
	"github.com/Vector-DNS/vdns-tui/internal/theme"
)

// settingsItem represents a selectable row in the settings view.
type settingsItem struct {
	label    string
	value    string
	action   string // "login", "logout", "toggle_mode", "toggle_history", ""
	editable bool
}

// SettingsView displays the current configuration with interactive options.
type SettingsView struct {
	width, height int
	config        *config.Config
	configDir     string
	dataDir       string
	viewport      viewport.Model
	ready         bool
	items         []settingsItem
	selected      int
	actionMode    bool // true when user is in the interactive list
}

// NewSettingsView creates a new settings view.
func NewSettingsView(cfg *config.Config) *SettingsView {
	cfgDir, _ := config.Dir()
	dataDir, _ := config.DataDir()

	v := &SettingsView{
		config:     cfg,
		configDir:  shortenHomePath(cfgDir),
		dataDir:    shortenHomePath(dataDir),
		actionMode: true,
	}
	v.rebuildItems()
	// Start selection on first actionable item (skip section headers).
	for i, item := range v.items {
		if !strings.HasPrefix(item.label, "--") {
			v.selected = i
			break
		}
	}
	return v
}

// rebuildItems constructs the interactive item list.
func (v *SettingsView) rebuildItems() {
	v.items = nil

	// Account section
	v.items = append(v.items, settingsItem{label: "-- Account --"})

	if v.config.IsLoggedIn() {
		v.items = append(v.items, settingsItem{
			label: "API Key", value: maskAPIKey(v.config.ActiveAPIKey()),
		})
		v.items = append(v.items, settingsItem{
			label: "Logout", value: "Run vdns logout", action: "logout", editable: true,
		})
	} else {
		v.items = append(v.items, settingsItem{
			label: "Login", value: "Run vdns login to authenticate", action: "login", editable: true,
		})
	}

	v.items = append(v.items, settingsItem{
		label: "Profile", value: v.config.ActiveProfile,
	})
	v.items = append(v.items, settingsItem{
		label: "Server", value: v.config.ActiveServer(),
	})

	// Mode section
	v.items = append(v.items, settingsItem{label: "-- Mode --"})
	modeVal := "local"
	if v.config.ShouldUseRemote() {
		modeVal = "remote"
	}
	v.items = append(v.items, settingsItem{
		label: "Preferred Mode", value: modeVal, action: "toggle_mode", editable: true,
	})

	// Local settings
	v.items = append(v.items, settingsItem{label: "-- Local Settings --"})
	histVal := "off"
	if v.config.Local.SaveHistory {
		histVal = "on"
	}
	v.items = append(v.items, settingsItem{
		label: "Save History", value: histVal, action: "toggle_history", editable: true,
	})
	v.items = append(v.items, settingsItem{
		label: "Max History", value: fmt.Sprintf("%d entries", v.config.Local.MaxHistoryItems),
	})

	// Paths
	v.items = append(v.items, settingsItem{label: "-- Paths --"})
	v.items = append(v.items, settingsItem{label: "Config Dir", value: v.configDir})
	v.items = append(v.items, settingsItem{label: "Data Dir", value: v.dataDir})

	// Resolver info
	v.items = append(v.items, settingsItem{label: "-- Resolvers --"})
	sysHost, _, err := net.SplitHostPort(dns.GetSystemResolver())
	if err != nil {
		sysHost = dns.GetSystemResolver()
	}
	label := sysHost
	if dns.IsDefaultResolver() {
		label += " (fallback)"
	} else {
		label += " (detected)"
	}
	v.items = append(v.items, settingsItem{label: "System Resolver", value: label})
	v.items = append(v.items, settingsItem{
		label: "Public Resolvers", value: fmt.Sprintf("%d configured", len(dns.DefaultResolvers)),
	})
}

// Update handles messages for the settings view.
func (v *SettingsView) Update(msg tea.Msg) (Component, tea.Cmd) {
	switch msg := msg.(type) {
	case DomainSubmitMsg:
		return v, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			v.selected--
			if v.selected < 0 {
				v.selected = 0
			}
			// Skip section headers
			for v.selected > 0 && strings.HasPrefix(v.items[v.selected].label, "--") {
				v.selected--
			}
		case "down", "j":
			v.selected++
			if v.selected >= len(v.items) {
				v.selected = len(v.items) - 1
			}
			// Skip section headers
			for v.selected < len(v.items)-1 && strings.HasPrefix(v.items[v.selected].label, "--") {
				v.selected++
			}
		case "enter":
			if v.selected >= 0 && v.selected < len(v.items) {
				item := v.items[v.selected]
				switch item.action {
				case "login":
					return v, func() tea.Msg {
						return ExecCommandMsg{Name: "vdns", Args: []string{"login"}}
					}
				case "logout":
					return v, func() tea.Msg {
						return ExecCommandMsg{Name: "vdns", Args: []string{"config", "set", "api_key", ""}}
					}
				case "toggle_mode":
					newMode := config.ModeLocal
					if v.config.PreferredMode == config.ModeLocal {
						newMode = config.ModeRemote
					}
					v.config.PreferredMode = newMode
					_ = config.Save(v.config)
					v.rebuildItems()
					return v, func() tea.Msg { return ConfigReloadMsg{} }
				case "toggle_history":
					v.config.Local.SaveHistory = !v.config.Local.SaveHistory
					_ = config.Save(v.config)
					v.rebuildItems()
				}
			}
		}
	}

	return v, nil
}

// View renders the settings view as an interactive list.
func (v *SettingsView) View(width, height int) string {
	v.width = width
	v.height = height

	labelStyle := lipgloss.NewStyle().Foreground(theme.ColorCoolGray).Width(18)
	valueStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite)
	sectionStyle := lipgloss.NewStyle().Foreground(theme.ColorPrimaryBlue).Bold(true)
	actionStyle := lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true)
	selectedBg := lipgloss.NewStyle().Foreground(theme.ColorAccentCyan).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(theme.ColorSlate600)

	var lines []string
	lines = append(lines, "")

	for i, item := range v.items {
		isSelected := i == v.selected

		// Section header
		if strings.HasPrefix(item.label, "--") {
			name := strings.Trim(item.label, "- ")
			if len(lines) > 1 {
				lines = append(lines, "")
			}
			lines = append(lines, "  "+sectionStyle.Render(name)+" "+dimStyle.Render(strings.Repeat("─", maxInt(0, width-lipgloss.Width(name)-8))))
			continue
		}

		// Build the row
		indicator := "  "
		if isSelected {
			indicator = selectedBg.Render("▸ ")
		}

		label := labelStyle.Render(item.label)
		var val string
		if item.editable {
			val = actionStyle.Render(item.value)
			if isSelected {
				val += dimStyle.Render("  [Enter]")
			}
		} else {
			val = valueStyle.Render(item.value)
		}

		lines = append(lines, indicator+label+val)
	}

	lines = append(lines, "")
	lines = append(lines, "  "+dimStyle.Render("Navigate with Up/Down, press Enter to change a setting"))

	content := strings.Join(lines, "\n")

	return content
}

// maskAPIKey masks an API key for display, showing only a prefix.
func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return key[:4] + "****"
	}
	return key[:8] + "****"
}

// shortenHomePath replaces the home directory prefix with ~.
func shortenHomePath(path string) string {
	if path == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
