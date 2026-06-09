package confirm

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/ui/msgs"
)

type acted struct{ cascade bool }

func newTest(cascadeOnly bool) Model {
	return New("Delete task?", []string{"really?"}, cascadeOnly,
		func(cascade bool) tea.Msg { return acted{cascade} })
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// collect runs the (possibly batched) cmd and returns all produced msgs.
func collect(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, collect(t, c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

func has(ms []tea.Msg, pred func(tea.Msg) bool) bool {
	for _, m := range ms {
		if pred(m) {
			return true
		}
	}
	return false
}

func TestConfirmYes(t *testing.T) {
	_, cmd := newTest(false).Update(key("y"))
	ms := collect(t, cmd)
	if !has(ms, func(m tea.Msg) bool { a, ok := m.(acted); return ok && !a.cascade }) {
		t.Fatal("y should run the action without cascade")
	}
	if !has(ms, func(m tea.Msg) bool { _, ok := m.(msgs.CloseModal); return ok }) {
		t.Fatal("y should close the modal")
	}
}

func TestCascadeOnlyRefusesPlainConfirm(t *testing.T) {
	_, cmd := newTest(true).Update(key("y"))
	ms := collect(t, cmd)
	if has(ms, func(m tea.Msg) bool { _, ok := m.(acted); return ok }) {
		t.Fatal("plain confirm must be refused when cascade is required")
	}
	_, cmd = newTest(true).Update(key("c"))
	ms = collect(t, cmd)
	if !has(ms, func(m tea.Msg) bool { a, ok := m.(acted); return ok && a.cascade }) {
		t.Fatal("c should cascade")
	}
}

func TestCancel(t *testing.T) {
	_, cmd := newTest(false).Update(key("esc"))
	ms := collect(t, cmd)
	if has(ms, func(m tea.Msg) bool { _, ok := m.(acted); return ok }) {
		t.Fatal("esc must not act")
	}
	if !has(ms, func(m tea.Msg) bool { _, ok := m.(msgs.CloseModal); return ok }) {
		t.Fatal("esc should close")
	}
}
