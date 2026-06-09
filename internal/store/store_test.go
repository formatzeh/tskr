package store

import (
	"path/filepath"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenMigrates(t *testing.T) {
	s := testStore(t)
	v, err := s.GetMeta("schema_version")
	if err != nil {
		t.Fatal(err)
	}
	if v != "1" {
		t.Fatalf("schema_version = %q, want 1", v)
	}
	for _, table := range []string{"projects", "tasks", "subtasks", "task_deps", "notes", "time_entries"} {
		var n int
		err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&n)
		if err != nil || n != 1 {
			t.Errorf("table %s missing (err %v)", table, err)
		}
	}
}

func TestMetaRoundtrip(t *testing.T) {
	s := testStore(t)
	if err := s.SetMeta("k", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMeta("k", "v2"); err != nil {
		t.Fatal(err)
	}
	v, err := s.GetMeta("k")
	if err != nil || v != "v2" {
		t.Fatalf("got %q, %v; want v2", v, err)
	}
	if v, _ := s.GetMeta("absent"); v != "" {
		t.Fatalf("absent key = %q, want empty", v)
	}
}
