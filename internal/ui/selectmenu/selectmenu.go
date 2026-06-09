// Package selectmenu is a small modal list of options (e.g. status select).
package selectmenu

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/ui/msgs"
	"tskr/internal/ui/styles"
)

type Model struct {
	Title  string
	Items  []string
	sel    int
	OnPick func(i int) tea.Msg
}

func New(title string, items []string, initial int, onPick func(int) tea.Msg) Model {
	if initial < 0 || initial >= len(items) {
		initial = 0
	}
	return Model{Title: title, Items: items, sel: initial, OnPick: onPick}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "j", "down":
		if m.sel < len(m.Items)-1 {
			m.sel++
		}
	case "k", "up":
		if m.sel > 0 {
			m.sel--
		}
	case "enter":
		return m, tea.Batch(msgs.Cmd(m.OnPick(m.sel)), msgs.Cmd(msgs.CloseModal{}))
	case "esc", "q":
		return m, msgs.Cmd(msgs.CloseModal{})
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render(m.Title) + "\n\n")
	for i, item := range m.Items {
		if i == m.sel {
			b.WriteString(styles.Cyan.Render("▸ "+item) + "\n")
		} else {
			b.WriteString("  " + item + "\n")
		}
	}
	return styles.ModalBox.Render(b.String())
}
