package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/store"
	"tskr/internal/ui/confirm"
	"tskr/internal/ui/help"
	"tskr/internal/ui/picker"
)

// sk extends key() with the non-rune keys the smoke test needs.
func sk(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	default:
		return key(s)
	}
}

// drive sends a message and pumps resulting command output back through
// Update until the queue drains, approximating the Bubble Tea runtime.
func drive(m Model, msg tea.Msg) Model {
	queue := []tea.Msg{msg}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		tm, cmd := m.Update(cur)
		m = tm.(Model)
		queue = append(queue, collect(cmd)...)
	}
	return m
}

// collect runs a command and flattens its messages; commands that do not
// return promptly (status-clear ticks, cursor blinks) are dropped.
func collect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	select {
	case msg := <-ch:
		if batch, ok := msg.(tea.BatchMsg); ok {
			var out []tea.Msg
			for _, c := range batch {
				out = append(out, collect(c)...)
			}
			return out
		}
		if msg == nil {
			return nil
		}
		if _, ok := msg.(tea.QuitMsg); ok {
			return nil
		}
		return []tea.Msg{msg}
	case <-time.After(150 * time.Millisecond):
		return nil
	}
}

func press(m Model, keys ...string) Model {
	for _, k := range keys {
		m = drive(m, sk(k))
	}
	return m
}

func typeStr(m Model, s string) Model {
	for _, r := range s {
		m = drive(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

// TestSmokeChecklist walks the manual verification checklist end to end
// against the real root model: project and task creation with validation,
// tabs, status changes, subtasks, notes, time logging, dependencies with
// cycle errors, cascade delete, search, sort, resize persistence, picker
// archive, project deletion, and the help overlay.
func TestSmokeChecklist(t *testing.T) {
	s, cfg, cfgPath := fixture(t)
	m := New(s, cfg, cfgPath)
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	// 1. Startup shows the picker; empty project name is refused.
	if len(m.modals) != 1 {
		t.Fatalf("startup: want picker modal, got %d modals", len(m.modals))
	}
	m = press(m, "n", "enter") // submit empty form
	if len(m.modals) != 2 {
		t.Fatalf("empty project name must be refused, modals = %d", len(m.modals))
	}
	m = typeStr(m, "Demo")
	m = press(m, "enter")
	if len(m.modals) != 1 {
		t.Fatalf("project form should close on valid submit, modals = %d", len(m.modals))
	}
	projects, _ := s.ListProjects(true)
	if len(projects) != 1 || projects[0].Name != "Demo" {
		t.Fatalf("project not created: %+v", projects)
	}
	m = press(m, "enter") // open Demo
	if m.project == nil || m.project.Name != "Demo" || len(m.modals) != 0 {
		t.Fatalf("open project failed: project=%+v modals=%d", m.project, len(m.modals))
	}

	// 2. New task: bad due date is refused inline, then fixed and saved.
	m = press(m, "a")
	m = typeStr(m, "My task")
	m = press(m, "tab", "tab", "tab") // -> due date field
	m = typeStr(m, "junk")
	m = press(m, "enter")
	if len(m.modals) != 1 {
		t.Fatal("bad due date must keep the form open")
	}
	m = press(m, "backspace", "backspace", "backspace", "backspace")
	m = typeStr(m, "2026-07-01")
	m = press(m, "tab")
	m = typeStr(m, "work")
	m = press(m, "enter")
	if len(m.modals) != 0 {
		t.Fatal("task form should close on valid submit")
	}
	tasks, _ := s.ListTasks(m.project.ID, "", store.SortCreated)
	if len(tasks) != 1 || tasks[0].Title != "My task" || tasks[0].DueDate != "2026-07-01" || tasks[0].Tags != "work" {
		t.Fatalf("task not created correctly: %+v", tasks)
	}
	if len(m.tl.Visible()) != 1 {
		t.Fatal("task should appear in the Pending tab")
	}

	// 3. Status menu and cycling move the task across tabs.
	m = press(m, "S", "j", "enter") // pending -> in_progress via menu
	if got, _ := s.GetTask(tasks[0].ID); got.Status != store.StatusInProgress {
		t.Fatalf("status menu failed: %+v", got.Status)
	}
	if len(m.tl.Visible()) != 0 {
		t.Fatal("in-progress task must leave the Pending tab")
	}
	m = press(m, "2") // In Progress tab
	if len(m.tl.Visible()) != 1 {
		t.Fatal("task should appear in the In Progress tab")
	}
	m = press(m, "s") // -> done
	if got, _ := s.GetTask(tasks[0].ID); got.Status != store.StatusDone || got.CompletedAt == "" {
		t.Fatalf("s should cycle to done: %+v", got)
	}
	m = press(m, "3")
	if len(m.tl.Visible()) != 1 {
		t.Fatal("done task should appear in the Done tab")
	}
	m = press(m, "s", "1") // done -> pending, back to first tab
	if len(m.tl.Visible()) != 1 {
		t.Fatal("pending task should be back in the Pending tab")
	}

	// 4. Detail panel: subtasks toggle and reorder.
	m = press(m, "enter")
	if m.focus != panelDetail {
		t.Fatal("enter should focus the detail panel")
	}
	m = press(m, "a")
	m = typeStr(m, "Sub one")
	m = press(m, "enter")
	m = press(m, "a")
	m = typeStr(m, "Sub two")
	m = press(m, "enter")
	subs, _ := s.ListSubtasks(tasks[0].ID)
	if len(subs) != 2 {
		t.Fatalf("want 2 subtasks: %+v", subs)
	}
	m = press(m, "space") // toggle subtask under cursor
	subs, _ = s.ListSubtasks(tasks[0].ID)
	if !subs[0].Done && !subs[1].Done {
		t.Fatalf("space should toggle a subtask: %+v", subs)
	}
	m = press(m, "J") // move first subtask down
	subs, _ = s.ListSubtasks(tasks[0].ID)
	if subs[0].Title != "Sub two" || subs[1].Title != "Sub one" {
		t.Fatalf("J should reorder subtasks: %+v", subs)
	}

	// 5. Notes and time logging.
	m = press(m, "n")
	m = typeStr(m, "A note")
	m = press(m, "enter")
	notes, _ := s.ListNotes(tasks[0].ID)
	if len(notes) != 1 || notes[0].Body != "A note" {
		t.Fatalf("note not added: %+v", notes)
	}
	m = press(m, "t")
	m = typeStr(m, "1h 30m")
	m = press(m, "enter")
	if got, _ := s.GetTask(tasks[0].ID); got.Minutes != 90 {
		t.Fatalf("time log failed, minutes = %d", got.Minutes)
	}

	// 6. Dependencies: toggle blocks, cycle attempt surfaces an error.
	m = press(m, "esc", "a") // back to tasks, add a second task
	m = typeStr(m, "Other")
	m = press(m, "enter")
	m = press(m, "k", "k", "enter") // select "My task", focus detail
	m = press(m, "b", "space")      // My task blocks Other
	tasks, _ = s.ListTasks(m.project.ID, "", store.SortCreated)
	other, _ := s.GetTask(tasks[1].ID)
	if !other.Blocked || len(other.BlockedBy) != 1 {
		t.Fatalf("dependency not added: %+v", other)
	}
	m = press(m, "esc")               // close depsel
	m = press(m, "esc", "j", "enter") // select "Other", focus detail
	m = press(m, "b", "space")        // Other blocks My task -> cycle
	if !m.statusErr || !strings.Contains(m.status, "cycle") {
		t.Fatalf("cycle must surface an error status, got %q (err=%v)", m.status, m.statusErr)
	}
	m = press(m, "esc", "esc") // close depsel, back to tasks panel

	// 7. Delete guard: y refused on a blocker, c cascades, dependent survives.
	m = press(m, "k", "d") // select "My task" (the blocker), delete
	if len(m.modals) != 1 {
		t.Fatal("delete should open a confirm dialog")
	}
	if c, ok := m.modals[0].(confirm.Model); !ok || !c.CascadeOnly {
		t.Fatalf("blocker delete must be cascade-only, got %T", m.modals[0])
	}
	m = press(m, "y")
	if len(m.modals) != 1 {
		t.Fatal("y must be refused on a cascade-only confirm")
	}
	m = press(m, "c")
	if len(m.modals) != 0 {
		t.Fatal("cascade delete should close the confirm")
	}
	if _, err := s.GetTask(tasks[0].ID); err == nil {
		t.Fatal("blocker should be deleted")
	}
	if got, err := s.GetTask(tasks[1].ID); err != nil || got.Blocked {
		t.Fatalf("dependent task must survive unblocked: %+v err=%v", got, err)
	}

	// 8. Fuzzy search filters live; esc clears; o cycles sort.
	m = press(m, "/")
	m = typeStr(m, "zz")
	if len(m.tl.Visible()) != 0 {
		t.Fatal("search 'zz' should match nothing")
	}
	m = press(m, "esc")
	if len(m.tl.Visible()) != 1 {
		t.Fatal("esc should clear the search filter")
	}
	m = press(m, "o")
	if m.tl.Sort() != store.SortDue {
		t.Fatalf("o should cycle sort to due, got %v", m.tl.Sort())
	}

	// 9. Split resize persists to the config file.
	m = press(m, ">")
	if m.cfg.SplitRatio <= 0.42 {
		t.Fatalf("> should widen the left panel, ratio = %v", m.cfg.SplitRatio)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil || !strings.Contains(string(data), "split_ratio = 0.47") {
		t.Fatalf("ratio must persist, config: %s (err %v)", data, err)
	}

	// 10. Help overlay opens and any key closes it.
	m = press(m, "?")
	if len(m.modals) != 1 {
		t.Fatal("? should open the help overlay")
	}
	if _, ok := m.modals[0].(help.Model); !ok {
		t.Fatalf("top modal should be help, got %T", m.modals[0])
	}
	if v := m.modals[0].View(); !strings.Contains(v, "keybindings") {
		t.Fatal("help overlay should render the keybinding reference")
	}
	m = press(m, "x")
	if len(m.modals) != 0 {
		t.Fatal("any key should close the help overlay")
	}

	// 11. Picker: archive a project, toggle archived view, delete the
	// current project and land back in a fresh picker.
	m = press(m, "p", "s") // archive "Demo"
	if p, _ := s.GetProject(m.project.ID); p.Status != store.ProjectArchived {
		t.Fatalf("s in picker should archive: %+v", p)
	}
	m = press(m, "A")      // show archived
	m = press(m, "d", "y") // delete it, confirm
	if m.project != nil {
		t.Fatal("deleting the current project should clear it")
	}
	if len(m.modals) != 1 {
		t.Fatalf("should land in exactly one picker, got %d modals", len(m.modals))
	}
	if _, ok := m.modals[0].(picker.Model); !ok {
		t.Fatalf("top modal should be the picker, got %T", m.modals[0])
	}
	if projects, _ := s.ListProjects(true); len(projects) != 0 {
		t.Fatalf("project should be gone: %+v", projects)
	}

	// The main view still renders.
	if v := m.View(); v == "" {
		t.Fatal("view should render the picker")
	}
}
