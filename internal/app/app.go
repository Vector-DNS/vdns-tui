package app

import (
	"github.com/Vector-DNS/vdns-tui/internal/client"
	"github.com/Vector-DNS/vdns-tui/internal/config"
	"github.com/Vector-DNS/vdns-tui/internal/views"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// View represents the active view in the TUI.
type View int

const (
	ViewHome View = iota
	ViewLookup
	ViewPropagation
	ViewWhois
	ViewAvailability
	ViewSSL
	ViewReport
	ViewHistory
	ViewBenchmark
	ViewSettings
)

var viewNames = []string{
	"Home",
	"Lookup",
	"Propagation",
	"Whois",
	"Availability",
	"SSL",
	"Full Report",
	"History",
	"Benchmark",
	"Settings",
}

var viewIcons = []string{
	"◆",
	"⌕",
	"◎",
	"≡",
	"◇",
	"⚷",
	"▣",
	"☰",
	"⚡",
	"⚙",
}

// viewCount is the total number of views.
const viewCount = 10

// Model is the top-level Bubble Tea model for the TUI.
type Model struct {
	width  int
	height int

	activeView   View
	sidebarFocus int
	sidebarOpen  bool
	contentFocus bool // true when arrow keys go to the content view instead of sidebar

	domainInput  textinput.Model
	domain       string
	inputFocused bool

	spinner    spinner.Model
	loading    bool
	statusText string
	errorText  string

	help     help.Model
	showHelp bool

	config    *config.Config
	apiClient *client.Client

	// Sub-views - each implements views.Component.
	homeView         views.Component
	lookupView       views.Component
	propagationView  views.Component
	whoisView        views.Component
	availabilityView views.Component
	sslView          views.Component
	reportView       views.Component
	historyView      views.Component
	benchmarkView    views.Component
	settingsView     views.Component
}

// New creates a new Model with default state, loading config and
// creating an API client if the user is logged in.
func New() Model {
	// Load config (fall back to defaults on error).
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Create API client when logged in.
	var apiClient *client.Client
	if cfg.IsLoggedIn() {
		apiClient = client.New(cfg.ActiveServer(), cfg.ActiveAPIKey())
	}

	// Domain input field.
	ti := textinput.New()
	ti.Placeholder = "press / to search..."
	ti.CharLimit = 253
	ti.Width = 30

	// Spinner with dots style and cyan color.
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = GlowStyle

	// Help model.
	h := help.New()
	h.ShowAll = true

	return Model{
		activeView:       ViewHome,
		sidebarFocus:     0,
		sidebarOpen:      true,
		domainInput:      ti,
		inputFocused:     false,
		spinner:          sp,
		help:             h,
		config:           cfg,
		apiClient:        apiClient,
		homeView:         views.NewHomeView(cfg),
		lookupView:       views.NewLookupView(cfg, apiClient),
		propagationView:  views.NewPropagationView(cfg, apiClient),
		whoisView:        views.NewWhoisView(cfg, apiClient),
		availabilityView: views.NewAvailabilityView(cfg, apiClient),
		sslView:          views.NewSSLView(cfg, apiClient),
		reportView:       views.NewReportView(cfg, apiClient),
		historyView:      views.NewHistoryView(cfg),
		benchmarkView:    views.NewBenchmarkView(cfg),
		settingsView:     views.NewSettingsView(cfg),
	}
}

// Init returns initial commands - text input blink and spinner tick.
func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

// activeComponent returns the views.Component for the current active view.
func (m *Model) activeComponent() views.Component {
	switch m.activeView {
	case ViewHome:
		return m.homeView
	case ViewLookup:
		return m.lookupView
	case ViewPropagation:
		return m.propagationView
	case ViewWhois:
		return m.whoisView
	case ViewAvailability:
		return m.availabilityView
	case ViewSSL:
		return m.sslView
	case ViewReport:
		return m.reportView
	case ViewHistory:
		return m.historyView
	case ViewBenchmark:
		return m.benchmarkView
	case ViewSettings:
		return m.settingsView
	default:
		return nil
	}
}

// setActiveComponent sets the views.Component for the current active view.
func (m *Model) setActiveComponent(c views.Component) {
	switch m.activeView {
	case ViewHome:
		m.homeView = c
	case ViewLookup:
		m.lookupView = c
	case ViewPropagation:
		m.propagationView = c
	case ViewWhois:
		m.whoisView = c
	case ViewAvailability:
		m.availabilityView = c
	case ViewSSL:
		m.sslView = c
	case ViewReport:
		m.reportView = c
	case ViewHistory:
		m.historyView = c
	case ViewBenchmark:
		m.benchmarkView = c
	case ViewSettings:
		m.settingsView = c
	}
}
