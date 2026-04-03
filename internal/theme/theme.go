package theme

import "github.com/charmbracelet/lipgloss"

// VectorDNS brand colors.
var (
	ColorPrimaryBlue = lipgloss.Color("#2563EB")
	ColorAccentCyan  = lipgloss.Color("#06B6D4")
	ColorMidnight    = lipgloss.Color("#0F172A")
	ColorSlate800    = lipgloss.Color("#1E293B")
	ColorSlate600    = lipgloss.Color("#475569")
	ColorCoolGray    = lipgloss.Color("#64748B")
	ColorEmerald     = lipgloss.Color("#10B981")
	ColorAmber       = lipgloss.Color("#F59E0B")
	ColorRose        = lipgloss.Color("#F43F5E")
	ColorViolet      = lipgloss.Color("#8B5CF6")
	ColorWhite       = lipgloss.Color("#E2E8F0")
)

// RecordTypeColor maps DNS record types to brand-appropriate colors.
// These show up in the lookup view tabs and result tables.
var RecordTypeColor = map[string]lipgloss.Color{
	"A":     ColorEmerald,
	"AAAA":  ColorAccentCyan,
	"CNAME": ColorPrimaryBlue,
	"MX":    ColorViolet,
	"NS":    ColorAmber,
	"TXT":   ColorWhite,
	"SOA":   ColorCoolGray,
	"CAA":   ColorRose,
	"SRV":   ColorAccentCyan,
}
