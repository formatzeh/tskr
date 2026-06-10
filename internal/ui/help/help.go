// Package help renders the full keybinding reference overlay.
package help

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/ui/keys"
	"tskr/internal/ui/msgs"
	"tskr/internal/ui/styles"
)

type Model struct{}

func New() Model { return Model{} }

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		return m, msgs.Cmd(msgs.CloseModal{})
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("tskr — keybindings") + "\n")
	for _, g := range keys.HelpGroups() {
		b.WriteString("\n" + styles.Cyan.Render(g.Title) + "\n")
		for _, h := range g.Hints {
			b.WriteString(fmt.Sprintf("  %s %s\n",
				styles.Cyan.Render(fmt.Sprintf("%-16s", h.Key)),
				styles.Light.Render(h.Desc)))
		}
	}
	b.WriteString("\n" + styles.Label.Render("press any key to close"))
	return styles.ModalBox.Render(b.String())
}
