package app

import (
	"os/exec"

	"github.com/Vector-DNS/vdns-tui/internal/client"
	"github.com/Vector-DNS/vdns-tui/internal/config"
	"github.com/Vector-DNS/vdns-tui/internal/validate"
	"github.com/Vector-DNS/vdns-tui/internal/views"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Update handles all messages for the main model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		// Delegate spinner ticks to the active sub-view so their spinners animate.
		return m.delegateToActiveView(msg)

	case views.StatusMsg:
		m.statusText = string(msg)
		return m, nil

	case views.ErrorMsg:
		if msg.Err != nil {
			m.errorText = msg.Err.Error()
		} else {
			m.errorText = ""
		}
		return m, nil

	case views.DomainSubmitMsg:
		m.domain = msg.Domain
		m.errorText = ""
		// Forward to active view.
		return m.delegateToActiveView(msg)

	case views.ExecCommandMsg:
		// Suspend the TUI and run an external command.
		c := exec.Command(msg.Name, msg.Args...)
		return m, tea.ExecProcess(c, func(err error) tea.Msg {
			if err != nil {
				return views.ErrorMsg{Err: err}
			}
			return views.ConfigReloadMsg{}
		})

	case views.ConfigReloadMsg:
		// Reload config after external command (e.g., vdns login).
		cfg, err := config.Load()
		if err != nil {
			cfg = config.DefaultConfig()
		}
		m.config = cfg
		if cfg.IsLoggedIn() {
			m.apiClient = client.New(cfg.ActiveServer(), cfg.ActiveAPIKey())
		} else {
			m.apiClient = nil
		}
		m.statusText = "Config reloaded"
		// Reinitialize ALL views with new config.
		m.homeView = views.NewHomeView(cfg)
		m.lookupView = views.NewLookupView(cfg, m.apiClient)
		m.propagationView = views.NewPropagationView(cfg, m.apiClient)
		m.whoisView = views.NewWhoisView(cfg, m.apiClient)
		m.availabilityView = views.NewAvailabilityView(cfg, m.apiClient)
		m.sslView = views.NewSSLView(cfg, m.apiClient)
		m.reportView = views.NewReportView(cfg, m.apiClient)
		m.historyView = views.NewHistoryView(cfg)
		m.benchmarkView = views.NewBenchmarkView(cfg)
		m.settingsView = views.NewSettingsView(cfg)
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	// Delegate everything else to the active view.
	return m.delegateToActiveView(msg)
}

// handleKeyMsg processes keyboard input.
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Always allow ctrl+c to quit.
	if key.Matches(msg, keys.Quit) && msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	// Help overlay: any key closes it, ? toggles it.
	if m.showHelp {
		m.showHelp = false
		return m, nil
	}
	if key.Matches(msg, keys.Help) {
		m.showHelp = true
		return m, nil
	}

	// When the domain input is focused, handle navigation keys globally
	// and only send text/editing keys to the input.
	if m.inputFocused {
		switch {
		case key.Matches(msg, keys.Escape):
			m.inputFocused = false
			m.domainInput.Blur()
			return m, nil

		case key.Matches(msg, keys.Enter):
			return m.submitDomain()

		// Let arrow keys, tab, and number keys pass through to global navigation.
		case key.Matches(msg, keys.Up), key.Matches(msg, keys.Down),
			key.Matches(msg, keys.Tab), key.Matches(msg, keys.ShiftTab),
			key.Matches(msg, keys.Number1), key.Matches(msg, keys.Number2),
			key.Matches(msg, keys.Number3), key.Matches(msg, keys.Number4),
			key.Matches(msg, keys.Number5), key.Matches(msg, keys.Number6),
			key.Matches(msg, keys.Number7), key.Matches(msg, keys.Number8),
			key.Matches(msg, keys.Number9):
			// Fall through to global key handling below.

		default:
			var cmd tea.Cmd
			m.domainInput, cmd = m.domainInput.Update(msg)
			return m, cmd
		}
	}

	// Global key bindings when input is not focused.
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.Slash):
		m.inputFocused = true
		m.domainInput.Focus()
		return m, nil

	case key.Matches(msg, keys.Escape):
		if m.contentFocus {
			m.contentFocus = false
			return m, nil
		}
		m.errorText = ""
		return m, nil

	case key.Matches(msg, keys.Number1):
		return m.switchView(ViewHome)
	case key.Matches(msg, keys.Number2):
		return m.switchView(ViewLookup)
	case key.Matches(msg, keys.Number3):
		return m.switchView(ViewPropagation)
	case key.Matches(msg, keys.Number4):
		return m.switchView(ViewWhois)
	case key.Matches(msg, keys.Number5):
		return m.switchView(ViewAvailability)
	case key.Matches(msg, keys.Number6):
		return m.switchView(ViewSSL)
	case key.Matches(msg, keys.Number7):
		return m.switchView(ViewReport)
	case key.Matches(msg, keys.Number8):
		return m.switchView(ViewHistory)
	case key.Matches(msg, keys.Number9):
		return m.switchView(ViewBenchmark)
	}

	// When content is focused, delegate arrow/navigation keys to the active view.
	if m.contentFocus {
		return m.delegateToActiveView(msg)
	}

	// Sidebar navigation (only when content is not focused).
	switch {
	case key.Matches(msg, keys.Tab), key.Matches(msg, keys.Down):
		m.sidebarFocus = (m.sidebarFocus + 1) % viewCount
		m.activeView = View(m.sidebarFocus)
		m.errorText = ""
		m.statusText = ""
		return m, nil

	case key.Matches(msg, keys.ShiftTab), key.Matches(msg, keys.Up):
		m.sidebarFocus = (m.sidebarFocus - 1 + viewCount) % viewCount
		m.activeView = View(m.sidebarFocus)
		m.errorText = ""
		m.statusText = ""
		return m, nil

	case key.Matches(msg, keys.Enter):
		return m.switchView(View(m.sidebarFocus))

	case key.Matches(msg, keys.Left):
		m.sidebarOpen = false
		return m, nil

	case key.Matches(msg, keys.Right):
		m.sidebarOpen = true
		return m, nil
	}

	// Delegate to active view.
	return m.delegateToActiveView(msg)
}

// submitDomain validates the domain input and sends a DomainSubmitMsg.
func (m Model) submitDomain() (tea.Model, tea.Cmd) {
	value := m.domainInput.Value()
	if value == "" {
		return m, nil
	}

	result, err := validate.Domain(value)
	if err != nil {
		m.errorText = "invalid domain: " + err.Error()
		return m, nil
	}

	m.domain = result.ASCII
	m.errorText = ""
	m.inputFocused = false
	m.domainInput.Blur()
	m.contentFocus = true

	return m.delegateToActiveView(views.DomainSubmitMsg{Domain: m.domain})
}

// switchView changes the active view and optionally sends the current
// domain to the new view.
func (m Model) switchView(v View) (tea.Model, tea.Cmd) {
	m.activeView = v
	m.sidebarFocus = int(v)
	m.errorText = ""
	m.contentFocus = true

	// If we have a domain set, send it to the new view.
	if m.domain != "" {
		return m.delegateToActiveView(views.DomainSubmitMsg{Domain: m.domain})
	}
	return m, nil
}

// delegateToActiveView forwards a message to the active sub-view.
func (m Model) delegateToActiveView(msg tea.Msg) (tea.Model, tea.Cmd) {
	comp := m.activeComponent()
	if comp == nil {
		return m, nil
	}

	updated, cmd := comp.Update(msg)
	m.setActiveComponent(updated)
	return m, cmd
}
