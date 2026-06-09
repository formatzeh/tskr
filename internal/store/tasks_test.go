package store

import (
	"errors"
	"testing"
)

func projectWithTasks(t *testing.T) (*Store, int64) {
	t.Helper()
	s := testStore(t)
	pid, err := s.CreateProject("P", "", "")
	if err != nil {
		t.Fatal(err)
	}
	return s, pid
}

func TestTaskCRUDAndStatus(t *testing.T) {
	s, pid := projectWithTasks(t)
	id, err := s.CreateTask(pid, "Fix bug", "desc", PriorityHigh, "2026-07-01", "A, a, b")
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.GetTask(id)
	if err != nil {
		t.Fatal(err)
	}
	if task.Title != "Fix bug" || task.Priority != PriorityHigh || task.DueDate != "2026-07-01" || task.Tags != "a,b" {
		t.Fatalf("unexpected task: %+v", task)
	}
	if task.Status != StatusPending || task.CompletedAt != "" {
		t.Fatalf("new task should be pending: %+v", task)
	}

	if err := s.SetTaskStatus(id, StatusDone); err != nil {
		t.Fatal(err)
	}
	task, err = s.GetTask(id)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != StatusDone || task.CompletedAt == "" {
		t.Fatalf("done should set completed_at: %+v", task)
	}
	if err := s.SetTaskStatus(id, StatusInProgress); err != nil {
		t.Fatal(err)
	}
	task, err = s.GetTask(id)
	if err != nil {
		t.Fatal(err)
	}
	if task.CompletedAt != "" {
		t.Fatalf("leaving done should clear completed_at: %+v", task)
	}

	if err := s.UpdateTask(id, "New", "", PriorityNone, "", ""); err != nil {
		t.Fatal(err)
	}
	task, err = s.GetTask(id)
	if err != nil {
		t.Fatal(err)
	}
	if task.Title != "New" || task.Priority != PriorityNone || task.DueDate != "" {
		t.Fatalf("update failed: %+v", task)
	}

	if err := s.DeleteTask(id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetTask(id); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestListTasksFilterAndSort(t *testing.T) {
	s, pid := projectWithTasks(t)
	a, _ := s.CreateTask(pid, "a", "", PriorityNone, "2026-01-02", "")
	b, _ := s.CreateTask(pid, "b", "", PriorityUrgent, "", "")
	c, _ := s.CreateTask(pid, "c", "", PriorityLow, "2026-01-01", "")
	s.SetTaskStatus(b, StatusDone)

	pending, err := s.ListTasks(pid, StatusPending, SortCreated)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 2 || pending[0].ID != a || pending[1].ID != c {
		t.Fatalf("pending sorted by created: %+v", pending)
	}

	all, _ := s.ListTasks(pid, "", SortDue)
	if len(all) != 3 || all[0].ID != c || all[1].ID != a || all[2].ID != b {
		t.Fatalf("due sort should put NULL last: %v %v %v", all[0].ID, all[1].ID, all[2].ID)
	}

	byPrio, _ := s.ListTasks(pid, "", SortPriority)
	if byPrio[0].ID != b || byPrio[1].ID != c || byPrio[2].ID != a {
		t.Fatalf("priority sort: %v %v %v", byPrio[0].ID, byPrio[1].ID, byPrio[2].ID)
	}
}

func TestDeleteGuard(t *testing.T) {
	s, pid := projectWithTasks(t)
	blocker, _ := s.CreateTask(pid, "blocker", "", PriorityNone, "", "")
	blocked, _ := s.CreateTask(pid, "blocked", "", PriorityNone, "", "")
	if err := s.AddDependency(blocker, blocked); err != nil {
		t.Fatal(err)
	}

	err := s.DeleteTask(blocker)
	var be *BlocksError
	if !errors.As(err, &be) || len(be.Blocked) != 1 || be.Blocked[0].ID != blocked {
		t.Fatalf("expected BlocksError listing blocked task, got %v", err)
	}

	if err := s.DeleteTaskCascade(blocker); err != nil {
		t.Fatal(err)
	}
	task, err := s.GetTask(blocked)
	if err != nil {
		t.Fatal("blocked task must survive cascade:", err)
	}
	if task.Blocked || len(task.BlockedBy) != 0 {
		t.Fatalf("dependency links should be gone: %+v", task)
	}
}
