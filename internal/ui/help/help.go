// Package help renders the full keybinding reference overlay.
package help

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"tskr/internal/ui/keys"
	"tskr/internal/ui/msgs"
	"tskr/internal/ui/styles"
)

type Model struct {
	w, h int
}

func New(w, h int) Model { return Model{w: w, h: h} }

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		return m, msgs.Cmd(msgs.CloseModal{})
	}
	return m, nil
}

func (m Model) View() string {
	// Render each group block once, tracking its height and width so we
	// can pack them into columns that fit the window both ways instead of
	// overflowing off the top/bottom (or sides) of the screen.
	type block struct {
		s string
		h int
	}
	blocks := make([]block, 0, len(keys.HelpGroups()))
	colWidth := 0
	for _, g := range keys.HelpGroups() {
		s := renderGroup(g)
		blocks = append(blocks, block{s: s, h: strings.Count(s, "\n") + 2}) // +blank separator
		if w := lipgloss.Width(s); w > colWidth {
			colWidth = w
		}
	}
	const gap = 3
	colWidth += gap

	// How many columns fit the width, and how tall each may be to fit height.
	innerW := m.w - 6 // modal border + horizontal padding
	maxCols := innerW / colWidth
	if maxCols < 1 {
		maxCols = 1
	}
	maxColH := m.h - 9 // title, footer, blank lines, modal border + padding
	if maxColH < 6 {
		maxColH = 6
	}
	if maxCols > len(blocks) {
		maxCols = len(blocks)
	}
	total := 0
	for _, b := range blocks {
		total += b.h
	}
	// Balance the columns: aim for an even height, but never exceed maxCols
	// (the last allowed column absorbs whatever remains so width stays bounded).
	if perCol := (total + maxCols - 1) / maxCols; perCol > maxColH {
		maxColH = perCol
	}

	var cols []string
	var cur strings.Builder
	curH := 0
	flush := func() {
		if cur.Len() > 0 {
			cols = append(cols, strings.TrimRight(cur.String(), "\n"))
			cur.Reset()
			curH = 0
		}
	}
	for _, b := range blocks {
		// Start a new column when this one is full — unless we're already on
		// the last permitted column, which takes the rest.
		if curH+b.h > maxColH && curH > 0 && len(cols) < maxCols-1 {
			flush()
		}
		cur.WriteString(b.s + "\n\n")
		curH += b.h
	}
	flush()

	colStyle := lipgloss.NewStyle().PaddingRight(gap)
	rendered := make([]string, len(cols))
	for i, c := range cols {
		rendered[i] = colStyle.Render(c)
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)

	content := styles.Title.Render("tskr — keybindings") + "\n\n" +
		body + "\n\n" +
		styles.Label.Render("press any key to close")
	return styles.ModalBox.Render(content)
}

func renderGroup(g keys.Group) string {
	var b strings.Builder
	b.WriteString(styles.Cyan.Render(g.Title) + "\n")
	for i, h := range g.Hints {
		b.WriteString(fmt.Sprintf("  %s %s",
			styles.Cyan.Render(fmt.Sprintf("%-16s", h.Key)),
			styles.Light.Render(h.Desc)))
		if i < len(g.Hints)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}
