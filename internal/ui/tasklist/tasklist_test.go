package tasklist

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/store"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func fixture(t *testing.T) (*store.Store, int64) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	pid, _ := s.CreateProject("P", "", "")
	return s, pid
}

func TestTabsFilterByStatus(t *testing.T) {
	s, pid := fixture(t)
	a, _ := s.CreateTask(pid, "pending one", "", store.PriorityNone, "", "")
	b, _ := s.CreateTask(pid, "done one", "", store.PriorityNone, "", "")
	s.SetTaskStatus(b, store.StatusDone)

	m := New(s)
	if err := m.SetProject(pid); err != nil {
		t.Fatal(err)
	}
	if sel := m.Selected(); sel == nil || sel.ID != a {
		t.Fatalf("pending tab should show task a, got %+v", sel)
	}
	m.Update(key("3")) // Done tab
	if sel := m.Selected(); sel == nil || sel.ID != b {
		t.Fatalf("done tab should show task b, got %+v", sel)
	}
	m.Update(key("4")) // All tab
	if len(m.Visible()) != 2 {
		t.Fatalf("all tab should show 2, got %d", len(m.Visible()))
	}
}

func TestMoveAndSearch(t *testing.T) {
	s, pid := fixture(t)
	s.CreateTask(pid, "alpha", "", store.PriorityNone, "", "")
	bid, _ := s.CreateTask(pid, "beta", "", store.PriorityNone, "", "")
	m := New(s)
	m.SetProject(pid)

	m.Update(key("j"))
	if sel := m.Selected(); sel == nil || sel.ID != bid {
		t.Fatalf("j should move to beta, got %+v", sel)
	}

	m.Update(key("/"))
	for _, r := range "bt" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if v := m.Visible(); len(v) != 1 || v[0].ID != bid {
		t.Fatalf("search 'bt' should keep beta only: %+v", v)
	}
	m.Update(key("esc")) // clears search
	if len(m.Visible()) != 2 {
		t.Fatal("esc should clear the filter")
	}
}

func TestSortCycle(t *testing.T) {
	s, pid := fixture(t)
	m := New(s)
	m.SetProject(pid)
	if m.Sort() != store.SortCreated {
		t.Fatal("default sort should be created")
	}
	m.Update(key("o"))
	if m.Sort() != store.SortDue {
		t.Fatal("o should cycle to due")
	}
}
