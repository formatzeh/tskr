package store

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return err
	}
	if legacy, err := s.isLegacyDB(); err != nil {
		return err
	} else if legacy {
		if err := s.migrateLegacy(); err != nil {
			return fmt.Errorf("legacy db: %w", err)
		}
		return nil
	}
	v, err := s.GetMeta("schema_version")
	if err != nil {
		return err
	}
	if v == "" {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(schemaSQL); err != nil {
			tx.Rollback()
			return err
		}
		if _, err := tx.Exec(`INSERT INTO meta (key, value) VALUES ('schema_version', '2')`); err != nil {
			tx.Rollback()
			return err
		}
		return tx.Commit()
	}
	if v == "1" {
		if _, err := s.db.Exec(`ALTER TABLE subtasks ADD COLUMN description TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
		if err := s.SetMeta("schema_version", "2"); err != nil {
			return err
		}
		v = "2"
	}
	if v == "2" {
		_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS notifications (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id          INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			title            TEXT NOT NULL DEFAULT '',
			body             TEXT NOT NULL DEFAULT '',
			urgency          TEXT NOT NULL DEFAULT 'normal' CHECK (urgency IN ('normal','critical')),
			mode             TEXT NOT NULL DEFAULT 'once' CHECK (mode IN ('once','recurring','interval')),
			due_date         TEXT NOT NULL DEFAULT '',
			interval_minutes INTEGER NOT NULL DEFAULT 0,
			trigger_status   TEXT NOT NULL DEFAULT 'pending',
			last_sent        TEXT NOT NULL DEFAULT '',
			created_at       TEXT NOT NULL,
			updated_at       TEXT NOT NULL
		)`)
		if err != nil {
			return err
		}
		if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_notifications_task ON notifications(task_id)`); err != nil {
			return err
		}
		if err := s.SetMeta("schema_version", "3"); err != nil {
			return err
		}
		v = "3"
	}
	if v == "3" {
		// Check if column already exists (fresh installs have it in schema.sql)
		var cnt int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('notifications') WHERE name='active'`).Scan(&cnt); err != nil {
			return err
		}
		if cnt == 0 {
			if _, err := s.db.Exec(`ALTER TABLE notifications ADD COLUMN active INTEGER NOT NULL DEFAULT 1`); err != nil {
				return err
			}
		}
		return s.SetMeta("schema_version", "4")
	}
	return nil
}

func (s *Store) GetMeta(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

func (s *Store) SetMeta(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }
