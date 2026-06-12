package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/config"
	"tskr/internal/store"
)

// TestKanbanToggleMoveAndPersist drives the board end to end through the
// real root model: enter the board, move a card to the next column, and
// confirm the status change persisted and the view choice was saved.
func TestKanbanToggleMoveAndPersist(t *testing.T) {
	s, cfg, path := fixture(t)
	pid, _ := s.CreateProject("P", "", "")
	id, _ := s.CreateTask(pid, "card", "", store.PriorityNone, "", "")

	m := New(s, cfg, path)
	m = drive(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m = drive(m, msgsOpenProject(pid))

	// Enter the board.
	m = press(m, "v")
	if m.view != viewKanban {
		t.Fatal("v should switch to the kanban view")
	}
	if saved, _ := config.Load(path); saved.View != "kanban" {
		t.Fatalf("view choice not persisted: got %q", saved.View)
	}

	// The board renders with all three column titles.
	for _, col := range []string{"Pending", "In Progress", "Done"} {
		if v := m.View(); !strings.Contains(v, col) {
			t.Fatalf("board view missing %q column", col)
		}
	}

	// Move the pending card one column to the right -> in_progress.
	if sel := m.kb.Selected(); sel == nil || sel.ID != id {
		t.Fatalf("board should select the only card, got %+v", sel)
	}
	m = press(m, "L")

	task, err := s.GetTask(id)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != store.StatusInProgress {
		t.Fatalf("L should move card to in_progress, got %q", task.Status)
	}
	if sel := m.kb.Selected(); sel == nil || sel.ID != id || m.kb.Col() != 1 {
		t.Fatalf("cursor should follow card into the in_progress column: sel=%+v col=%d", sel, m.kb.Col())
	}

	// enter hands off to the detail view.
	m = press(m, "enter")
	if m.view != viewList || m.focus != panelDetail {
		t.Fatalf("enter should open detail in list view: view=%d focus=%d", m.view, m.focus)
	}

	// Back to the board, then toggle to the list persists "list".
	m = press(m, "v")
	m = press(m, "v")
	if m.view != viewList {
		t.Fatal("second toggle should return to the list view")
	}
	if saved, _ := config.Load(path); saved.View != "list" {
		t.Fatalf("list choice not persisted: got %q", saved.View)
	}
}
