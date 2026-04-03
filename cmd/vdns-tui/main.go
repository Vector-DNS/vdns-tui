package main

import (
	"fmt"
	"os"

	"github.com/Vector-DNS/vdns-tui/internal/app"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	m := app.New()
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
