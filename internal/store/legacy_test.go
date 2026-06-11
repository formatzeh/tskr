package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// legacySchema mirrors the tables created by the pre-rewrite tskr.
const legacySchema = `
CREATE TABLE projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'Pending',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE TABLE tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    description TEXT DEFAULT '',
    status TEXT NOT NULL DEFAULT 'Pending',
    priority TEXT DEFAULT 'none',
    due_date DATE,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE TABLE subtasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    done BOOLEAN DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE TABLE notes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);
CREATE TABLE timelogs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    minutes INTEGER NOT NULL,
    note TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);
CREATE TABLE time_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    duration_minutes INTEGER NOT NULL CHECK (duration_minutes > 0),
    note TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);
CREATE TABLE task_tags (
    task_id INTEGER NOT NULL,
    tag_id INTEGER NOT NULL,
    PRIMARY KEY (task_id, tag_id),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);
CREATE TABLE project_tags (
    project_id INTEGER NOT NULL,
    tag_id INTEGER NOT NULL,
    PRIMARY KEY (project_id, tag_id),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);
CREATE TABLE task_dependencies (
    task_id INTEGER NOT NULL,
    blocks_task_id INTEGER NOT NULL,
    PRIMARY KEY (task_id, blocks_task_id),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (blocks_task_id) REFERENCES tasks(id) ON DELETE CASCADE
);
`

const legacyData = `
INSERT INTO projects (id, name, status, created_at, updated_at) VALUES
    (1, 'iuyiy', 'Pending', '2026-06-10 11:11:59', '2026-06-10 11:11:59'),
    (2, 'Old stuff', 'Archived', '2026-01-05 08:00:00', '2026-01-05 08:00:00');
INSERT INTO tasks (id, project_id, title, description, status, priority, due_date, created_at, updated_at) VALUES
    (1, 1, 'regffreg', '', 'Pending', 'none', NULL, '2026-06-10 11:12:02', '2026-06-10 11:12:02'),
    (2, 0, 'orphan', '', 'pending', NULL, NULL, '2026-06-10 11:44:21', '2026-06-10 11:44:21'),
    (3, 1, 'second', 'desc', 'InProgress', 'High', '2026-07-01', '2026-06-10 12:00:00', '2026-06-10 12:00:00'),
    (4, 1, 'third', '', 'Done', 'low', '', '2026-06-10 12:30:00', '2026-06-10 12:30:00');
INSERT INTO subtasks (id, task_id, title, done, created_at) VALUES
    (1, 1, 'sub one', 0, '2026-06-10 11:13:00'),
    (2, 1, 'sub two', 1, '2026-06-10 11:14:00'),
    (3, 2, 'orphan sub', 0, '2026-06-10 11:45:00');
INSERT INTO notes (id, task_id, content, created_at) VALUES
    (1, 1, 'a legacy note', '2026-06-10 11:15:00');
INSERT INTO timelogs (task_id, minutes, note) VALUES (1, 30, 'old log');
INSERT INTO time_logs (task_id, duration_minutes, note) VALUES (1, 45, NULL);
INSERT INTO tags (id, name) VALUES (1, 'Work');
INSERT INTO task_tags (task_id, tag_id) VALUES (1, 1);
INSERT INTO project_tags (project_id, tag_id) VALUES (1, 1);
INSERT INTO task_dependencies (task_id, blocks_task_id) VALUES (3, 4), (2, 3);
`

// seedLegacyDB creates a database as the old tskr left it. stamped also
// replays the broken first run of the rewrite, which created the new-only
// tables and indexes via IF NOT EXISTS and wrote schema_version=1 without
// actually migrating anything.
func seedLegacyDB(t *testing.T, stamped bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{legacySchema, legacyData} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if stamped {
		stampSQL := `
CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO meta (key, value) VALUES ('schema_version', '1');
CREATE TABLE task_deps (
    blocker_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    blocked_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    PRIMARY KEY (blocker_id, blocked_id),
    CHECK (blocker_id != blocked_id)
);
CREATE TABLE time_entries (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id    INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    minutes    INTEGER NOT NULL CHECK (minutes > 0),
    note       TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);
CREATE INDEX idx_tasks_project ON tasks(project_id);
CREATE INDEX idx_subtasks_task ON subtasks(task_id);
CREATE INDEX idx_notes_task ON notes(task_id);
CREATE INDEX idx_time_task ON time_entries(task_id);
CREATE INDEX idx_deps_blocked ON task_deps(blocked_id);
`
		if _, err := db.Exec(stampSQL); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func openLegacy(t *testing.T, stamped bool) *Store {
	t.Helper()
	s, err := Open(seedLegacyDB(t, stamped))
	if err != nil {
		t.Fatalf("Open legacy db: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestLegacyMigration(t *testing.T) {
	// stamped=true mirrors a database the rewrite already ran against
	// once; stamped=false is a database straight from the old app.
	for _, stamped := range []bool{true, false} {
		name := "fresh"
		if stamped {
			name = "stamped"
		}
		t.Run(name, func(t *testing.T) {
			s := openLegacy(t, stamped)

			projects, err := s.ListProjects(true)
			if err != nil {
				t.Fatalf("ListProjects: %v", err)
			}
			if len(projects) != 2 {
				t.Fatalf("want 2 projects, got %+v", projects)
			}
			byName := map[string]Project{}
			for _, p := range projects {
				byName[p.Name] = p
			}
			p := byName["iuyiy"]
			if p.ID != 1 || p.Status != ProjectActive || p.Tags != "work" {
				t.Errorf("iuyiy migrated wrong: %+v", p)
			}
			if p.CreatedAt != "2026-06-10T11:11:59Z" {
				t.Errorf("created_at not converted: %q", p.CreatedAt)
			}
			if old := byName["Old stuff"]; old.Status != ProjectArchived {
				t.Errorf("archived status lost: %+v", old)
			}

			tasks, err := s.ListTasks(1, "", SortCreated)
			if err != nil {
				t.Fatalf("ListTasks: %v", err)
			}
			if len(tasks) != 3 {
				t.Fatalf("want 3 tasks for project 1, got %+v", tasks)
			}
			byTitle := map[string]Task{}
			for _, tk := range tasks {
				byTitle[tk.Title] = tk
			}
			if tk := byTitle["regffreg"]; tk.ID != 1 || tk.Status != StatusPending ||
				tk.Priority != "" || tk.Tags != "work" || tk.Minutes != 75 {
				t.Errorf("regffreg migrated wrong: %+v", tk)
			}
			if tk := byTitle["second"]; tk.Status != StatusInProgress || tk.Priority != PriorityHigh ||
				tk.DueDate != "2026-07-01" || tk.Description != "desc" {
				t.Errorf("second migrated wrong: %+v", tk)
			}
			if tk := byTitle["third"]; tk.Status != StatusDone || tk.Priority != PriorityLow || tk.DueDate != "" {
				t.Errorf("third migrated wrong: %+v", tk)
			}

			// The orphan task pointed at project 0 and cannot survive
			// foreign-key enforcement.
			if _, err := s.GetTask(2); err == nil {
				t.Error("orphan task should not be migrated")
			}

			subs, err := s.ListSubtasks(1)
			if err != nil || len(subs) != 2 {
				t.Fatalf("subtasks: %+v (err %v)", subs, err)
			}
			if subs[0].Title != "sub one" || subs[0].Done || subs[1].Title != "sub two" || !subs[1].Done {
				t.Errorf("subtasks migrated wrong: %+v", subs)
			}

			notes, err := s.ListNotes(1)
			if err != nil || len(notes) != 1 || notes[0].Body != "a legacy note" {
				t.Fatalf("notes migrated wrong: %+v (err %v)", notes, err)
			}

			blocked, err := s.GetTask(4)
			if err != nil {
				t.Fatal(err)
			}
			if !blocked.Blocked || len(blocked.BlockedBy) != 1 || blocked.BlockedBy[0].ID != 3 {
				t.Errorf("dependency migrated wrong: %+v", blocked)
			}

			// The original bug: creating a project must work again.
			id, err := s.CreateProject("after migration", "d", "t")
			if err != nil {
				t.Fatalf("CreateProject after migration: %v", err)
			}
			if p, err := s.GetProject(id); err != nil || p.Name != "after migration" {
				t.Fatalf("created project unreadable: %+v (err %v)", p, err)
			}

			// No legacy leftovers.
			rows, err := s.db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND (
				name LIKE '%_legacy' OR name IN ('timelogs','time_logs','tags','task_tags','project_tags','task_dependencies'))`)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			for rows.Next() {
				var n string
				rows.Scan(&n)
				t.Errorf("leftover legacy table %q", n)
			}

			// The new tasks index must exist on the new table.
			var n int
			if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master
				WHERE type='index' AND name='idx_tasks_project' AND tbl_name='tasks'`).Scan(&n); err != nil || n != 1 {
				t.Errorf("idx_tasks_project missing on new tasks table (n=%d err=%v)", n, err)
			}

			if v, _ := s.GetMeta("schema_version"); v != "2" {
				t.Errorf("schema_version = %q", v)
			}
		})
	}
}

// TestLegacyMigrationReopen makes sure the migration is a one-shot: a
// second Open must not touch the migrated data.
func TestLegacyMigrationReopen(t *testing.T) {
	path := seedLegacyDB(t, true)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProject("kept", "", ""); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	projects, err := s2.ListProjects(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 3 {
		t.Fatalf("reopen lost data: %+v", projects)
	}
}
