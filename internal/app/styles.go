package app

import (
	"github.com/Vector-DNS/vdns-tui/internal/theme"
	"github.com/charmbracelet/lipgloss"
)

const sidebarWidth = 22

// Sidebar styles - no backgrounds, just text colors.
var (
	SidebarStyle = lipgloss.NewStyle().
			Width(sidebarWidth).
			Padding(1, 1).
			BorderRight(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(theme.ColorSlate800)

	SidebarItemStyle = lipgloss.NewStyle().
				Foreground(theme.ColorSlate600).
				Padding(0, 1).
				Width(sidebarWidth - 2)

	SidebarActiveStyle = lipgloss.NewStyle().
				Foreground(theme.ColorAccentCyan).
				Bold(true).
				Padding(0, 1).
				Width(sidebarWidth - 2)

	SidebarHoverStyle = lipgloss.NewStyle().
				Foreground(theme.ColorWhite).
				Padding(0, 1).
				Width(sidebarWidth - 2)
)

// Layout styles - minimal backgrounds, clean lines.
var (
	ContentStyle = lipgloss.NewStyle().
			Padding(1, 2)

	HeaderStyle = lipgloss.NewStyle().
			Foreground(theme.ColorWhite).
			Padding(0, 2).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(theme.ColorSlate800)

	InputStyle = lipgloss.NewStyle().
			Foreground(theme.ColorSlate600).
			Padding(0, 1)

	InputActiveStyle = lipgloss.NewStyle().
				Foreground(theme.ColorWhite).
				Padding(0, 1)

	InputBarStyle = lipgloss.NewStyle().
			Foreground(theme.ColorCoolGray).
			Padding(0, 1).
			BorderTop(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(theme.ColorSlate800)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(theme.ColorSlate600).
			Padding(0, 1)
)

// Typography styles.
var (
	TitleStyle = lipgloss.NewStyle().
			Foreground(theme.ColorPrimaryBlue).
			Bold(true)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(theme.ColorCoolGray).
			Italic(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(theme.ColorRose).
			Bold(true)
)

// Decorative styles.
var (
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.ColorSlate600).
			Padding(1, 2)

	GlowStyle = lipgloss.NewStyle().
			Foreground(theme.ColorAccentCyan)

	DividerStyle = lipgloss.NewStyle().
			Foreground(theme.ColorSlate600)

	ModeLocalStyle = lipgloss.NewStyle().
			Foreground(theme.ColorAccentCyan).
			Bold(true)

	ModeRemoteStyle = lipgloss.NewStyle().
			Foreground(theme.ColorEmerald).
			Bold(true)

	RateLimitStyle = lipgloss.NewStyle().
			Foreground(theme.ColorCoolGray)

	LogoStyle = lipgloss.NewStyle().
			Foreground(theme.ColorAccentCyan).
			Bold(true)
)
