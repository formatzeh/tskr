package store

import (
	"errors"
	"testing"
)

func TestDependencies(t *testing.T) {
	s, pid := projectWithTasks(t)
	a, _ := s.CreateTask(pid, "a", "", PriorityNone, "", "")
	b, _ := s.CreateTask(pid, "b", "", PriorityNone, "", "")
	c, _ := s.CreateTask(pid, "c", "", PriorityNone, "", "")

	if err := s.AddDependency(a, b); err != nil { // a blocks b
		t.Fatal(err)
	}
	if err := s.AddDependency(b, c); err != nil { // b blocks c
		t.Fatal(err)
	}

	task, _ := s.GetTask(b)
	if !task.Blocked || len(task.BlockedBy) != 1 || task.BlockedBy[0].ID != a {
		t.Fatalf("b should be blocked by a: %+v", task)
	}
	if len(task.Blocks) != 1 || task.Blocks[0].ID != c {
		t.Fatalf("b should block c: %+v", task)
	}

	// blocked flag clears when blocker is done
	s.SetTaskStatus(a, StatusDone)
	task, _ = s.GetTask(b)
	if task.Blocked {
		t.Fatal("done blockers should not mark a task blocked")
	}

	if err := s.AddDependency(c, a); !errors.Is(err, ErrCycle) {
		t.Fatalf("c→a closes a cycle, got %v", err)
	}
	if err := s.AddDependency(a, a); !errors.Is(err, ErrSelf) {
		t.Fatalf("self dep, got %v", err)
	}

	other, _ := s.CreateProject("other", "", "")
	x, _ := s.CreateTask(other, "x", "", PriorityNone, "", "")
	if err := s.AddDependency(a, x); !errors.Is(err, ErrCrossProject) {
		t.Fatalf("cross-project dep, got %v", err)
	}

	if err := s.RemoveDependency(a, b); err != nil {
		t.Fatal(err)
	}
	task, _ = s.GetTask(b)
	if len(task.BlockedBy) != 0 {
		t.Fatalf("remove failed: %+v", task)
	}
}
