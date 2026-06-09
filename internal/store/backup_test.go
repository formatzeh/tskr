package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupIfDue(t *testing.T) {
	s := testStore(t)
	s.CreateProject("P", "", "")
	dir := filepath.Join(t.TempDir(), "backups")

	ran, err := s.BackupIfDue(dir, 7, "2026-06-09")
	if err != nil || !ran {
		t.Fatalf("first backup: ran=%v err=%v", ran, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "tskr-2026-06-09.db")); err != nil {
		t.Fatal("backup file missing:", err)
	}
	// the backup is a valid database containing the data
	b, err := Open(filepath.Join(dir, "tskr-2026-06-09.db"))
	if err != nil {
		t.Fatal(err)
	}
	ps, _ := b.ListProjects(true)
	b.Close()
	if len(ps) != 1 {
		t.Fatalf("backup content: %+v", ps)
	}

	ran, err = s.BackupIfDue(dir, 7, "2026-06-09")
	if err != nil || ran {
		t.Fatalf("same-day backup must be skipped: ran=%v err=%v", ran, err)
	}
}

func TestBackupRotation(t *testing.T) {
	s := testStore(t)
	dir := t.TempDir()
	for _, d := range []string{"01", "02", "03", "04", "05", "06", "07"} {
		os.WriteFile(filepath.Join(dir, "tskr-2026-06-"+d+".db"), []byte("x"), 0o644)
	}
	if _, err := s.BackupIfDue(dir, 7, "2026-06-09"); err != nil {
		t.Fatal(err)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "tskr-*.db"))
	if len(matches) != 7 {
		t.Fatalf("want 7 files after rotation, got %d", len(matches))
	}
	if _, err := os.Stat(filepath.Join(dir, "tskr-2026-06-01.db")); !os.IsNotExist(err) {
		t.Fatal("oldest backup should be pruned")
	}
}
