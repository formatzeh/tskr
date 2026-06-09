package store

import "testing"

func TestNotes(t *testing.T) {
	s, tid := taskFixture(t)
	n1, err := s.AddNote(tid, "first")
	if err != nil {
		t.Fatal(err)
	}
	s.AddNote(tid, "second")
	list, err := s.ListNotes(tid)
	if err != nil || len(list) != 2 || list[0].Body != "first" {
		t.Fatalf("list = %+v, err %v", list, err)
	}
	if err := s.UpdateNote(n1, "edited"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteNote(n1); err != nil {
		t.Fatal(err)
	}
	list, _ = s.ListNotes(tid)
	if len(list) != 1 || list[0].Body != "second" {
		t.Fatalf("after delete: %+v", list)
	}
	task, _ := s.GetTask(tid)
	if task.NoteCount != 1 {
		t.Fatalf("note count: %+v", task)
	}
}

func TestTimeEntries(t *testing.T) {
	s, tid := taskFixture(t)
	e1, err := s.AddTimeEntry(tid, 90, "research")
	if err != nil {
		t.Fatal(err)
	}
	s.AddTimeEntry(tid, 60, "")
	task, _ := s.GetTask(tid)
	if task.Minutes != 150 {
		t.Fatalf("minutes = %d, want 150", task.Minutes)
	}
	if err := s.UpdateTimeEntry(e1, 30, "less"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTimeEntry(e1); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListTimeEntries(tid)
	if err != nil || len(list) != 1 || list[0].Minutes != 60 {
		t.Fatalf("entries = %+v, err %v", list, err)
	}
	if _, err := s.AddTimeEntry(tid, 0, ""); err == nil {
		t.Fatal("zero minutes must be rejected by CHECK constraint")
	}
}
