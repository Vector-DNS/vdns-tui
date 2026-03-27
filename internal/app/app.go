package app

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type View int

const (
	ViewHome View = iota
	ViewLookup
	ViewPropagation
	ViewWhois
	ViewAvailability
	ViewSSL
	ViewHistory
	ViewBenchmark
	ViewSettings
)

var sidebarItems = []string{
	"Home",
	"Lookup",
	"Propagation",
	"Whois",
	"Availability",
	"SSL",
	"History",
	"Benchmark",
	"Settings",
}

type Model struct {
	width  int
	height int

	activeView   View
	sidebarFocus int
	domainInput  textinput.Model
	domain       string

	inputFocused bool
}

func New() Model {
	ti := textinput.New()
	ti.Placeholder = "enter a domain..."
	ti.CharLimit = 253
	ti.Width = 30
	ti.Focus()

	return Model{
		activeView:   ViewHome,
		sidebarFocus: 0,
		domainInput:  ti,
		inputFocused: true,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}
