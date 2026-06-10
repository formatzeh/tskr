package depsel

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/store"
	"tskr/internal/ui/msgs"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func fixture(t *testing.T) (*store.Store, store.Task, int64) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	pid, _ := s.CreateProject("P", "", "")
	tid, _ := s.CreateTask(pid, "main", "", store.PriorityNone, "", "")
	other, _ := s.CreateTask(pid, "other", "", store.PriorityNone, "", "")
	task, _ := s.GetTask(tid)
	return s, task, other
}

func TestToggleBlocks(t *testing.T) {
	s, task, other := fixture(t)
	m := New(s, task)
	m2, _ := m.Update(key("space")) // toggle: main blocks other
	got, _ := s.GetTask(other)
	if len(got.BlockedBy) != 1 || got.BlockedBy[0].ID != task.ID {
		t.Fatalf("toggle on failed: %+v", got.BlockedBy)
	}
	m2.(Model).Update(key("space")) // toggle off
	got, _ = s.GetTask(other)
	if len(got.BlockedBy) != 0 {
		t.Fatalf("toggle off failed: %+v", got.BlockedBy)
	}
}

func TestCycleRejectionSurfacesError(t *testing.T) {
	s, task, other := fixture(t)
	s.AddDependency(other, task.ID) // other blocks main
	m := New(s, task)
	_, cmd := m.Update(key("space")) // main blocks other would close a cycle
	if cmd == nil {
		t.Fatal("expected an error status command")
	}
	st, ok := cmd().(msgs.Status)
	if !ok || !st.Error {
		t.Fatalf("want error status, got %v", cmd())
	}
}

func TestEscClosesAndRefreshes(t *testing.T) {
	s, task, _ := fixture(t)
	m := New(s, task)
	_, cmd := m.Update(key("esc"))
	if cmd == nil {
		t.Fatal("esc should emit commands")
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("want batch, got %T", msg)
	}
	var refresh, closed bool
	for _, c := range batch {
		switch c().(type) {
		case msgs.Refresh:
			refresh = true
		case msgs.CloseModal:
			closed = true
		}
	}
	if !refresh || !closed {
		t.Fatalf("want Refresh+CloseModal, got refresh=%v closed=%v", refresh, closed)
	}
}
