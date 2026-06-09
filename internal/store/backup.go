package store

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// BackupIfDue writes a dated backup with VACUUM INTO when none was made
// today, then prunes the directory to the newest `keep` backups.
// today is "YYYY-MM-DD". Returns whether a backup ran.
func (s *Store) BackupIfDue(dir string, keep int, today string) (bool, error) {
	last, err := s.GetMeta("last_backup_date")
	if err != nil {
		return false, err
	}
	if last >= today {
		return false, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	dest := filepath.Join(dir, "tskr-"+today+".db")
	os.Remove(dest) // VACUUM INTO refuses to overwrite
	quoted := strings.ReplaceAll(dest, "'", "''")
	if _, err := s.db.Exec("VACUUM INTO '" + quoted + "'"); err != nil {
		return false, err
	}
	if err := s.SetMeta("last_backup_date", today); err != nil {
		return false, err
	}
	matches, err := filepath.Glob(filepath.Join(dir, "tskr-*.db"))
	if err != nil {
		return true, err
	}
	sort.Strings(matches) // dated names sort chronologically
	for len(matches) > keep {
		if err := os.Remove(matches[0]); err != nil {
			return true, err
		}
		matches = matches[1:]
	}
	return true, nil
}
