package app

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Quit     key.Binding
	Help     key.Binding
	Tab      key.Binding
	ShiftTab key.Binding
	Enter    key.Binding
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Escape   key.Binding
	Slash    key.Binding
	Number1  key.Binding
	Number2  key.Binding
	Number3  key.Binding
	Number4  key.Binding
	Number5  key.Binding
	Number6  key.Binding
	Number7  key.Binding
	Number8  key.Binding
	Number9  key.Binding
}

var keys = keyMap{
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c", "q"),
		key.WithHelp("q/ctrl+c", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next view"),
	),
	ShiftTab: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "prev view"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "submit/select"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("up/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("down/j", "down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("left/h", "collapse sidebar"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("right/l", "expand sidebar"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "unfocus/clear"),
	),
	Slash: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "focus input"),
	),
	Number1: key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "Home")),
	Number2: key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "Lookup")),
	Number3: key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "Propagation")),
	Number4: key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "Whois")),
	Number5: key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "Availability")),
	Number6: key.NewBinding(key.WithKeys("6"), key.WithHelp("6", "SSL")),
	Number7: key.NewBinding(key.WithKeys("7"), key.WithHelp("7", "History")),
	Number8: key.NewBinding(key.WithKeys("8"), key.WithHelp("8", "Benchmark")),
	Number9: key.NewBinding(key.WithKeys("9"), key.WithHelp("9", "Settings")),
}

// ShortHelp returns key bindings for the mini help view.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit, k.Help, k.Tab, k.Enter, k.Slash}
}

// FullHelp returns key bindings for the full help view.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Tab, k.ShiftTab, k.Enter, k.Escape},
		{k.Slash, k.Quit, k.Help},
		{k.Number1, k.Number2, k.Number3, k.Number4, k.Number5},
		{k.Number6, k.Number7, k.Number8, k.Number9},
	}
}
