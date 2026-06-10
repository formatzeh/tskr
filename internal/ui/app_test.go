package ui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/config"
	"tskr/internal/store"
	"tskr/internal/ui/msgs"
	"tskr/internal/ui/picker"
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

func fixture(t *testing.T) (*store.Store, *config.Config, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	cfg := config.Default()
	return s, &cfg, filepath.Join(dir, "config.toml")
}

func TestStartsWithPicker(t *testing.T) {
	s, cfg, path := fixture(t)
	m := New(s, cfg, path)
	if len(m.modals) != 1 {
		t.Fatal("picker startup mode must open the picker modal")
	}
	if _, ok := m.modals[0].(picker.Model); !ok {
		t.Fatalf("top modal should be the picker, got %T", m.modals[0])
	}
}

func TestLastProjectStartup(t *testing.T) {
	s, cfg, path := fixture(t)
	pid, _ := s.CreateProject("P", "", "")
	s.SetMeta("last_project_id", "1")
	cfg.Startup = "last-project"
	m := New(s, cfg, path)
	if len(m.modals) != 0 || m.project == nil || m.project.ID != pid {
		t.Fatalf("should resume project %d: modals=%d project=%+v", pid, len(m.modals), m.project)
	}
}

func TestOpenProjectAndFocusFlow(t *testing.T) {
	s, cfg, path := fixture(t)
	pid, _ := s.CreateProject("P", "", "")
	s.CreateTask(pid, "task", "", store.PriorityNone, "", "")
	m := New(s, cfg, path)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	mm, _ = mm.(Model).Update(msgsOpenProject(pid))
	app := mm.(Model)
	if app.project == nil || len(app.modals) != 0 {
		t.Fatal("OpenProject should close the picker and set the project")
	}
	mm, _ = app.Update(key("enter"))
	app = mm.(Model)
	if app.focus != panelDetail {
		t.Fatal("enter should focus the detail panel")
	}
	mm, _ = app.Update(key("esc"))
	app = mm.(Model)
	if app.focus != panelTasks {
		t.Fatal("esc should focus the task list again")
	}
}

func msgsOpenProject(id int64) tea.Msg { return msgs.OpenProject{ID: id} }
