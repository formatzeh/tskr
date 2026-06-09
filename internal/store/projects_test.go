package store

import "testing"

func TestProjectCRUD(t *testing.T) {
	s := testStore(t)
	id, err := s.CreateProject("Website", "redesign", "Work, web")
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.GetProject(id)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Website" || p.Tags != "work,web" || p.Status != ProjectActive {
		t.Fatalf("unexpected project: %+v", p)
	}
	if err := s.UpdateProject(id, "Site", "x", ""); err != nil {
		t.Fatal(err)
	}
	p, _ = s.GetProject(id)
	if p.Name != "Site" {
		t.Fatalf("update failed: %+v", p)
	}
	if err := s.DeleteProject(id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetProject(id); err == nil {
		t.Fatal("expected error for deleted project")
	}
}

// TODO(task-4): enable once CreateTask exists.
/*
func TestListProjectsArchiveAndCount(t *testing.T) {
	s := testStore(t)
	a, _ := s.CreateProject("A", "", "")
	b, _ := s.CreateProject("B", "", "")
	if _, err := s.CreateTask(a, "t1", "", PriorityNone, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.SetProjectStatus(b, ProjectArchived); err != nil {
		t.Fatal(err)
	}
	active, err := s.ListProjects(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].ID != a || active[0].TaskCount != 1 {
		t.Fatalf("active = %+v", active)
	}
	all, _ := s.ListProjects(true)
	if len(all) != 2 {
		t.Fatalf("all = %+v", all)
	}
}
*/
