package selectmenu

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

type picked struct{ i int }

func key(s string) tea.KeyMsg {
	if s == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestPick(t *testing.T) {
	m := New("Status", []string{"Pending", "In Progress", "Done"}, 0,
		func(i int) tea.Msg { return picked{i} })
	mm, _ := m.Update(key("j"))
	mm, cmd := mm.(Model).Update(key("enter"))
	_ = mm
	if cmd == nil {
		t.Fatal("enter should produce a command")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected batch, got %T", msg)
	}
	found := false
	for _, c := range batch {
		if p, ok := c().(picked); ok && p.i == 1 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected picked{1}")
	}
}
