// Package styles is the single source of truth for tskr's colors and styles.
package styles

import (
	"github.com/charmbracelet/lipgloss"

	"tskr/internal/config"
	"tskr/internal/store"
)

// Base colors — adjusted by ApplyColors. Defaults are slightly lighter than
// the original palette to improve readability on dark terminals.
var (
	ColMagenta = lipgloss.Color("#ce88e4")
	ColBlue    = lipgloss.Color("#72bbf3")
	ColGreen   = lipgloss.Color("#a5cf88")
	ColYellow  = lipgloss.Color("#e9ca8a")
	ColOrange  = lipgloss.Color("#d9a876")
	ColRed     = lipgloss.Color("#e87c84")
	ColCyan    = lipgloss.Color("#62dde8")
	ColGray    = lipgloss.Color("#7c8799") // was #5c6370 — labels now much more readable
	ColLight   = lipgloss.Color("#adb8c7")
	ColText    = lipgloss.Color("#d4d8e2")
	ColBg      = lipgloss.Color("#11151c")
)

// Derived styles — rebuilt whenever base colors change.
var (
	Text      lipgloss.Style
	Label     lipgloss.Style
	Light     lipgloss.Style
	Tag       lipgloss.Style
	Blue      lipgloss.Style
	Green     lipgloss.Style
	Yellow    lipgloss.Style
	Orange    lipgloss.Style
	Red       lipgloss.Style
	Cyan      lipgloss.Style
	Title     lipgloss.Style
	FormLabel lipgloss.Style
	Err       lipgloss.Style
	Ok        lipgloss.Style

	Panel        lipgloss.Style
	PanelFocused lipgloss.Style
	ModalBox     lipgloss.Style
	ModalBoxRed  lipgloss.Style

	TabActive   lipgloss.Style
	TabInactive lipgloss.Style
)

func init() { rebuildStyles() }

func rebuildStyles() {
	Text      = lipgloss.NewStyle().Foreground(ColText)
	Label     = lipgloss.NewStyle().Foreground(ColGray)
	Light     = lipgloss.NewStyle().Foreground(ColLight)
	Tag       = lipgloss.NewStyle().Foreground(ColMagenta)
	Blue      = lipgloss.NewStyle().Foreground(ColBlue)
	Green     = lipgloss.NewStyle().Foreground(ColGreen)
	Yellow    = lipgloss.NewStyle().Foreground(ColYellow)
	Orange    = lipgloss.NewStyle().Foreground(ColOrange)
	Red       = lipgloss.NewStyle().Foreground(ColRed)
	Cyan      = lipgloss.NewStyle().Foreground(ColCyan)
	Title     = lipgloss.NewStyle().Bold(true)
	FormLabel = lipgloss.NewStyle().Foreground(ColLight)
	Err       = lipgloss.NewStyle().Foreground(ColRed)
	Ok        = lipgloss.NewStyle().Foreground(ColGreen)

	Panel        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColGray)
	PanelFocused = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColCyan)
	ModalBox     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColCyan).Padding(1, 2)
	ModalBoxRed  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColRed).Padding(1, 2)

	TabActive   = lipgloss.NewStyle().Background(ColCyan).Foreground(ColBg).Bold(true).Padding(0, 1)
	TabInactive = lipgloss.NewStyle().Foreground(ColGray).Padding(0, 1)
}

// ApplyColors overrides base colors from cfg and rebuilds all derived styles.
// Call this once after loading config, before starting the UI.
// Empty strings in cfg leave the corresponding color at its default.
func ApplyColors(c config.ColorConfig) {
	if c.Cyan != "" {
		ColCyan = lipgloss.Color(c.Cyan)
	}
	if c.Magenta != "" {
		ColMagenta = lipgloss.Color(c.Magenta)
	}
	if c.Blue != "" {
		ColBlue = lipgloss.Color(c.Blue)
	}
	if c.Green != "" {
		ColGreen = lipgloss.Color(c.Green)
	}
	if c.Yellow != "" {
		ColYellow = lipgloss.Color(c.Yellow)
	}
	if c.Orange != "" {
		ColOrange = lipgloss.Color(c.Orange)
	}
	if c.Red != "" {
		ColRed = lipgloss.Color(c.Red)
	}
	if c.Gray != "" {
		ColGray = lipgloss.Color(c.Gray)
	}
	if c.Light != "" {
		ColLight = lipgloss.Color(c.Light)
	}
	if c.Text != "" {
		ColText = lipgloss.Color(c.Text)
	}
	rebuildStyles()
}

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
