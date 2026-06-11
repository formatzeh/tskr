package detail

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/store"
)

func key(s string) tea.KeyMsg {
	if s == "space" {
		return tea.KeyMsg{Type: tea.KeySpace}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func fixture(t *testing.T) (*store.Store, int64) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	pid, _ := s.CreateProject("P", "", "")
	tid, _ := s.CreateTask(pid, "Example", "a description", store.PriorityMedium, "", "tag1,tag2")
	return s, tid
}

func TestCursorWalksItems(t *testing.T) {
	s, tid := fixture(t)
	s.AddSubtask(tid, "one", "")
	s.AddSubtask(tid, "two", "")
	s.AddNote(tid, "a note")

	m := New(s)
	m.SetSize(60, 30)
	if err := m.SetTask(tid); err != nil {
		t.Fatal(err)
	}
	m.Focused = true

	it, ok := m.CurrentItem()
	if !ok || it.Kind != ItemSubtask {
		t.Fatalf("cursor should start on first subtask: %+v ok=%v", it, ok)
	}
	m.Update(key("j"))
	m.Update(key("j"))
	it, _ = m.CurrentItem()
	if it.Kind != ItemNote {
		t.Fatalf("cursor should reach the note: %+v", it)
	}
	m.Update(key("j")) // clamp at end
	it, _ = m.CurrentItem()
	if it.Kind != ItemNote {
		t.Fatalf("cursor must clamp: %+v", it)
	}
}

func TestSpaceTogglesSubtask(t *testing.T) {
	s, tid := fixture(t)
	sid, _ := s.AddSubtask(tid, "one", "")
	m := New(s)
	m.SetSize(60, 30)
	m.SetTask(tid)
	m.Focused = true
	m.Update(key("space"))
	subs, _ := s.ListSubtasks(tid)
	if !subs[0].Done {
		t.Fatal("space should toggle the subtask done")
	}
	_ = sid
}

func TestViewShowsTaskFacts(t *testing.T) {
	s, tid := fixture(t)
	m := New(s)
	m.SetSize(60, 30)
	m.SetTask(tid)
	view := m.View()
	for _, want := range []string{"Example", "Medium", "tag1, tag2", "a description"} {
		if !strings.Contains(view, want) {
			t.Errorf("view should contain %q", want)
		}
	}
}
