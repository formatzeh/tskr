package store

import "testing"

func taskFixture(t *testing.T) (*Store, int64) {
	t.Helper()
	s, pid := projectWithTasks(t)
	tid, err := s.CreateTask(pid, "task", "", PriorityNone, "", "")
	if err != nil {
		t.Fatal(err)
	}
	return s, tid
}

func TestSubtaskLifecycle(t *testing.T) {
	s, tid := taskFixture(t)
	a, _ := s.AddSubtask(tid, "one", "")
	b, _ := s.AddSubtask(tid, "two", "")
	c, _ := s.AddSubtask(tid, "three", "")

	list, err := s.ListSubtasks(tid)
	if err != nil || len(list) != 3 {
		t.Fatalf("list = %+v, err %v", list, err)
	}
	if list[0].ID != a || list[1].ID != b || list[2].ID != c {
		t.Fatalf("order wrong: %+v", list)
	}

	if err := s.ToggleSubtask(b); err != nil {
		t.Fatal(err)
	}
	list, _ = s.ListSubtasks(tid)
	if !list[1].Done {
		t.Fatal("toggle failed")
	}
	task, _ := s.GetTask(tid)
	if task.SubtasksDone != 1 || task.SubtasksTotal != 3 {
		t.Fatalf("aggregates: %+v", task)
	}

	if err := s.MoveSubtask(c, true); err != nil {
		t.Fatal(err)
	}
	list, _ = s.ListSubtasks(tid)
	if list[1].ID != c || list[2].ID != b {
		t.Fatalf("move up failed: %+v", list)
	}
	if err := s.MoveSubtask(a, true); err != nil {
		t.Fatal(err) // moving the first up is a no-op
	}

	if err := s.UpdateSubtask(a, "renamed", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSubtask(a); err != nil {
		t.Fatal(err)
	}
	list, _ = s.ListSubtasks(tid)
	if len(list) != 2 {
		t.Fatalf("delete failed: %+v", list)
	}
}
