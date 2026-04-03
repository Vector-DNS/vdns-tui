package views

import tea "github.com/charmbracelet/bubbletea"

// Component is the interface every view implements.
type Component interface {
	Update(msg tea.Msg) (Component, tea.Cmd)
	View(width, height int) string
}

// DomainSubmitMsg is sent when the user presses enter on the domain input.
type DomainSubmitMsg struct {
	Domain string
}

// StatusMsg carries a status line update.
type StatusMsg string

// ErrorMsg carries an error to display.
type ErrorMsg struct {
	Err error
}

// ExecCommandMsg requests the app to suspend and run an external command.
type ExecCommandMsg struct {
	Name string
	Args []string
}

// ConfigReloadMsg signals that config should be reloaded (after login, settings change, etc).
type ConfigReloadMsg struct{}
