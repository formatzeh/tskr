// Package styles is the single source of truth for tskr's colors.
package styles

import (
	"github.com/charmbracelet/lipgloss"

	"tskr/internal/store"
)

var (
	ColMagenta = lipgloss.Color("#c678dd")
	ColBlue    = lipgloss.Color("#61afef")
	ColGreen   = lipgloss.Color("#98c379")
	ColYellow  = lipgloss.Color("#e5c07b")
	ColOrange  = lipgloss.Color("#d19a66")
	ColRed     = lipgloss.Color("#e06c75")
	ColCyan    = lipgloss.Color("#56d6e0")
	ColGray    = lipgloss.Color("#5c6370")
	ColLight   = lipgloss.Color("#9da5b4")
	ColText    = lipgloss.Color("#c8ccd4")
	ColBg      = lipgloss.Color("#11151c")
)

var (
	Text   = lipgloss.NewStyle().Foreground(ColText)
	Label  = lipgloss.NewStyle().Foreground(ColGray)
	Light  = lipgloss.NewStyle().Foreground(ColLight)
	Tag    = lipgloss.NewStyle().Foreground(ColMagenta)
	Blue   = lipgloss.NewStyle().Foreground(ColBlue)
	Green  = lipgloss.NewStyle().Foreground(ColGreen)
	Yellow = lipgloss.NewStyle().Foreground(ColYellow)
	Orange = lipgloss.NewStyle().Foreground(ColOrange)
	Red    = lipgloss.NewStyle().Foreground(ColRed)
	Cyan   = lipgloss.NewStyle().Foreground(ColCyan)
	Title  = lipgloss.NewStyle().Bold(true)
	Err    = Red
	Ok     = Green

	Panel        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColGray)
	PanelFocused = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColCyan)
	ModalBox     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColCyan).Padding(1, 2)
	ModalBoxRed  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColRed).Padding(1, 2)

	TabActive   = lipgloss.NewStyle().Background(ColCyan).Foreground(ColBg).Bold(true).Padding(0, 1)
	TabInactive = lipgloss.NewStyle().Foreground(ColGray).Padding(0, 1)
)

// PriorityBadge renders the color-coded list badge; "———" means none.
func PriorityBadge(p store.Priority) string {
	switch p {
	case store.PriorityLow:
		return Green.Render("LOW")
	case store.PriorityMedium:
		return Yellow.Render("MED")
	case store.PriorityHigh:
		return Orange.Render("HIGH")
	case store.PriorityUrgent:
		return Red.Render("URG")
	default:
		return Label.Render("———")
	}
}

// PriorityName renders the detail-panel value ("Medium", "—").
func PriorityName(p store.Priority) string {
	switch p {
	case store.PriorityLow:
		return Green.Render("Low")
	case store.PriorityMedium:
		return Yellow.Render("Medium")
	case store.PriorityHigh:
		return Orange.Render("High")
	case store.PriorityUrgent:
		return Red.Render("Urgent")
	default:
		return Label.Render("—")
	}
}

// StatusLabel renders a colored status with its glyph.
func StatusLabel(st store.TaskStatus) string {
	switch st {
	case store.StatusInProgress:
		return Yellow.Render("◐ In Progress")
	case store.StatusDone:
		return Green.Render("● Done")
	default:
		return Label.Render("○ Pending")
	}
}
