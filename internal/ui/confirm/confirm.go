// Package confirm renders confirmation dialogs for destructive actions,
// including the delete-guard variant that only allows cascade.
package confirm

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/ui/keys"
	"tskr/internal/ui/msgs"
	"tskr/internal/ui/styles"
)

type Model struct {
	Title       string
	Lines       []string
	CascadeOnly bool
	Action      func(cascade bool) tea.Msg
}

func New(title string, lines []string, cascadeOnly bool, action func(bool) tea.Msg) Model {
	return Model{Title: title, Lines: lines, CascadeOnly: cascadeOnly, Action: action}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "y", "enter":
		if m.CascadeOnly {
			return m, msgs.Info("this task blocks others — press c to delete with cascade, esc to abort")
		}
		return m, tea.Batch(msgs.Cmd(m.Action(false)), msgs.Cmd(msgs.CloseModal{}))
	case "c":
		if m.CascadeOnly {
			return m, tea.Batch(msgs.Cmd(m.Action(true)), msgs.Cmd(msgs.CloseModal{}))
		}
	case "n", "esc", "q":
		return m, msgs.Cmd(msgs.CloseModal{})
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render(m.Title) + "\n\n")
	for _, l := range m.Lines {
		b.WriteString(l + "\n")
	}
	b.WriteString("\n")
	hints := keys.Confirm
	if m.CascadeOnly {
		hints = []keys.Hint{{Key: "c", Desc: "cascade delete"}, {Key: "esc", Desc: "cancel"}}
	}
	b.WriteString(keys.Render(hints))
	return styles.ModalBoxRed.Render(b.String())
}
