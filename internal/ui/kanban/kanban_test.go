package kanban

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/store"
)

func key(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// board returns a sized board for project pid, reloaded.
func board(t *testing.T) (*store.Store, Model, int64) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	pid, _ := s.CreateProject("P", "", "")
	m := New(s)
	m.SetSize(90, 24)
	if err := m.SetProject(pid); err != nil {
		t.Fatal(err)
	}
	return s, m, pid
}

func mkTask(t *testing.T, s *store.Store, pid int64, title string, st store.TaskStatus) int64 {
	t.Helper()
	id, err := s.CreateTask(pid, title, "", store.PriorityNone, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if st != store.StatusPending {
		if err := s.SetTaskStatus(id, st); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func TestReloadBucketsByStatus(t *testing.T) {
	s, m, pid := board(t)
	mkTask(t, s, pid, "p1", store.StatusPending)
	mkTask(t, s, pid, "p2", store.StatusPending)
	mkTask(t, s, pid, "ip", store.StatusInProgress)
	mkTask(t, s, pid, "d", store.StatusDone)
	if err := m.Reload(); err != nil {
		t.Fatal(err)
	}
	if got := []int{len(m.cols[0]), len(m.cols[1]), len(m.cols[2])}; got[0] != 2 || got[1] != 1 || got[2] != 1 {
		t.Fatalf("column counts = %v, want [2 1 1]", got)
	}
}

func TestNavigationClamps(t *testing.T) {
	s, m, pid := board(t)
	mkTask(t, s, pid, "p1", store.StatusPending)
	mkTask(t, s, pid, "p2", store.StatusPending)
	mkTask(t, s, pid, "ip", store.StatusInProgress)
	m.Reload()

	// k at top stays at 0.
	m.Update(key("k"))
	if m.sel != 0 {
		t.Fatalf("sel after k at top = %d, want 0", m.sel)
	}
	// j moves down, but not past the last card.
	m.Update(key("j"))
	m.Update(key("j"))
	if m.sel != 1 {
		t.Fatalf("sel after two j in 2-card column = %d, want 1", m.sel)
	}
	// h at leftmost column stays.
	m.Update(key("h"))
	if m.col != 0 {
		t.Fatalf("col after h at left = %d, want 0", m.col)
	}
	// l moves right; sel re-clamps into the shorter column (1 card -> sel 0).
	m.Update(key("l"))
	if m.col != 1 || m.sel != 0 {
		t.Fatalf("after l: col=%d sel=%d, want col=1 sel=0", m.col, m.sel)
	}
	// l past the last column stays at 2.
	m.Update(key("l"))
	m.Update(key("l"))
	if m.col != 2 {
		t.Fatalf("col after l past end = %d, want 2", m.col)
	}
}

func TestFocusTaskFollowsAcrossReload(t *testing.T) {
	s, m, pid := board(t)
	mkTask(t, s, pid, "p1", store.StatusPending)
	id := mkTask(t, s, pid, "p2", store.StatusPending)
	m.Reload()

	// Simulate the root model moving the card to in_progress and asking
	// the board to keep the cursor on it.
	m.FocusTask(id)
	if err := s.SetTaskStatus(id, store.StatusInProgress); err != nil {
		t.Fatal(err)
	}
	m.Reload()
	if sel := m.Selected(); sel == nil || sel.ID != id {
		t.Fatalf("cursor did not follow moved card: %+v", sel)
	}
	if m.col != 1 {
		t.Fatalf("cursor column = %d, want 1 (in_progress)", m.col)
	}
}
