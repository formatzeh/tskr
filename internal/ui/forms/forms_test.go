package forms

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/ui/msgs"
)

type saved struct{ vals []string }

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func collect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, collect(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

func TestSubmitBlockedUntilValid(t *testing.T) {
	m := New("Test", []Field{
		NewField("Title", "", "", Required),
		NewField("Due", "nonsense", "", OptionalDate),
	}, func(vals []string) tea.Msg { return saved{vals} })

	mm, cmd := m.Update(key("enter"))
	if collect(cmd) != nil {
		t.Fatal("invalid form must not emit anything")
	}
	form := mm.(Model)
	if form.errs[0] == "" || form.errs[1] == "" {
		t.Fatalf("expected errors on both fields: %+v", form.errs)
	}
}

func TestSubmitValid(t *testing.T) {
	m := New("Test", []Field{NewField("Title", "hello", "", Required)},
		func(vals []string) tea.Msg { return saved{vals} })
	_, cmd := m.Update(key("enter"))
	ms := collect(cmd)
	var got *saved
	closed := false
	for _, msg := range ms {
		if s, ok := msg.(saved); ok {
			got = &s
		}
		if _, ok := msg.(msgs.CloseModal); ok {
			closed = true
		}
	}
	if got == nil || got.vals[0] != "hello" || !closed {
		t.Fatalf("submit failed: %v closed=%v", ms, closed)
	}
}

func TestValidators(t *testing.T) {
	if Required(" ") == "" {
		t.Error("Required should reject blank")
	}
	if OptionalDate("") != "" || OptionalDate("2026-07-01") != "" {
		t.Error("OptionalDate should accept empty and valid dates")
	}
	if OptionalDate("01.07.2026") == "" {
		t.Error("OptionalDate should reject other formats")
	}
	if OptionalPriority("") != "" || OptionalPriority("Urgent") != "" {
		t.Error("OptionalPriority should accept empty and known levels")
	}
	if OptionalPriority("mega") == "" {
		t.Error("OptionalPriority should reject unknown levels")
	}
	if ValidDuration("1h 30m") != "" || ValidDuration("nope") == "" {
		t.Error("ValidDuration")
	}
}
