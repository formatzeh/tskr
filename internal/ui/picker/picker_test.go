package picker

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/store"
	"tskr/internal/ui/msgs"
)

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

func testStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenSelected(t *testing.T) {
	s := testStore(t)
	a, _ := s.CreateProject("Alpha", "", "")
	s.CreateProject("Beta", "", "")
	m := New(s, false)
	_, cmd := m.Update(key("enter"))
	if cmd == nil {
		t.Fatal("enter should open")
	}
	if op, ok := cmd().(msgs.OpenProject); !ok || op.ID != a {
		t.Fatalf("want OpenProject{%d}, got %v", a, cmd())
	}
}

func TestFuzzyFilter(t *testing.T) {
	s := testStore(t)
	s.CreateProject("Alpha", "", "")
	b, _ := s.CreateProject("Beta", "", "")
	m := New(s, false)
	mm, _ := m.Update(key("/"))
	for _, r := range "bt" {
		mm, _ = mm.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	mm, _ = mm.(Model).Update(key("enter")) // leave search mode, keep filter
	mm, cmd := mm.(Model).Update(key("enter"))
	_ = mm
	if op, ok := cmd().(msgs.OpenProject); !ok || op.ID != b {
		t.Fatalf("filtered open: got %v", cmd())
	}
}

func TestEscRespectsAllowClose(t *testing.T) {
	s := testStore(t)
	s.CreateProject("Alpha", "", "")
	_, cmd := New(s, false).Update(key("esc"))
	if cmd == nil {
		t.Fatal("startup picker: esc should quit the app")
	}
	if msg := cmd(); msg != tea.Quit() {
		t.Fatalf("startup picker esc: want tea.Quit, got %v", msg)
	}
	_, cmd = New(s, true).Update(key("esc"))
	if cmd == nil {
		t.Fatal("on-demand picker should close on esc")
	}
	if _, ok := cmd().(msgs.CloseModal); !ok {
		t.Fatalf("want CloseModal, got %v", cmd())
	}
}

func TestQuitFromStartupPicker(t *testing.T) {
	s := testStore(t)
	for _, k := range []string{"q", "esc"} {
		_, cmd := New(s, false).Update(key(k))
		if cmd == nil {
			t.Fatalf("startup picker: %q should quit", k)
		}
		if msg := cmd(); msg != tea.Quit() {
			t.Fatalf("startup picker %q: want tea.Quit, got %v", k, msg)
		}
	}
	// On-demand picker (allowClose=true) must not quit on q — it closes.
	_, cmd := New(s, true).Update(key("q"))
	if cmd == nil {
		t.Fatal("on-demand picker: q should close")
	}
	if _, ok := cmd().(msgs.CloseModal); !ok {
		t.Fatalf("on-demand picker q: want CloseModal, got %v", cmd())
	}
}
