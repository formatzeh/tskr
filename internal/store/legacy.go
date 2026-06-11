package store

import (
	"fmt"
)

// The pre-rewrite tskr used a different schema: projects without
// description/tags columns, capitalized statuses ('Pending'), tags in
// join tables, notes with a content column, and time in timelogs /
// time_logs. The rewrite's schema.sql only ran CREATE TABLE IF NOT
// EXISTS, so opening such a database used to stamp schema_version=1
// while leaving the old tables in place, after which every INSERT
// failed. isLegacyDB spots both states by looking at the actual table
// shape instead of trusting the version stamp.
func (s *Store) isLegacyDB() (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='projects'`).Scan(&n)
	if err != nil || n == 0 {
		return false, err
	}
	hasDesc, err := s.hasColumn("projects", "description")
	return !hasDesc, err
}

func (s *Store) hasColumn(table, column string) (bool, error) {
	rows, err := s.db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

// legacyTS converts the old "YYYY-MM-DD HH:MM:SS" timestamps (from
// SQLite's datetime('now')) to the RFC3339 form the rewrite stores.
func legacyTS(col string) string {
	return fmt.Sprintf("CASE WHEN instr(%[1]s, ' ') > 0 THEN replace(%[1]s, ' ', 'T') || 'Z' ELSE %[1]s END", col)
}

// migrateLegacy rebuilds the database in the current schema and copies
// the old data over, inside a single transaction. Rows that cannot
// survive foreign-key enforcement (tasks of nonexistent projects and
// their children — the old app did not enforce FKs) are dropped.
func (s *Store) migrateLegacy() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Move every legacy table out of the way. task_deps and time_entries
	// may exist in the new format already (the broken first run created
	// them), but their REFERENCES tasks(id) clauses follow the tasks
	// rename, and dropping tasks_legacy would then cascade-delete the
	// copied rows — so they are rebuilt and copied like everything else.
	legacyTables := []string{
		"projects", "tasks", "subtasks", "notes", "timelogs", "time_logs",
		"tags", "task_tags", "project_tags", "task_dependencies",
		"task_deps",
	}
	for _, name := range legacyTables {
		var n int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n); err != nil {
			return err
		}
		if n == 1 {
			if _, err := tx.Exec(fmt.Sprintf(`ALTER TABLE %s RENAME TO %s_legacy`, name, name)); err != nil {
				return fmt.Errorf("rename %s: %w", name, err)
			}
		}
	}

	// Indexes follow their renamed table but keep their name, which
	// would make schema.sql's CREATE INDEX IF NOT EXISTS skip the new
	// table. Drop them; the legacy tables are gone after the copy anyway.
	idxRows, err := tx.Query(`SELECT name FROM sqlite_master
		WHERE type='index' AND tbl_name LIKE '%_legacy' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return err
	}
	var idxNames []string
	for idxRows.Next() {
		var name string
		if err := idxRows.Scan(&name); err != nil {
			idxRows.Close()
			return err
		}
		idxNames = append(idxNames, name)
	}
	idxRows.Close()
	for _, name := range idxNames {
		if _, err := tx.Exec(`DROP INDEX ` + name); err != nil {
			return fmt.Errorf("drop index %s: %w", name, err)
		}
	}

	if _, err := tx.Exec(schemaSQL); err != nil {
		return err
	}

	// Stub out optional legacy tables so the copy statements below can
	// reference them unconditionally.
	for name, cols := range map[string]string{
		"tasks_legacy":             `(id INTEGER, project_id INTEGER, title TEXT, description TEXT, status TEXT, priority TEXT, due_date TEXT, created_at TEXT, updated_at TEXT)`,
		"subtasks_legacy":          `(id INTEGER, task_id INTEGER, title TEXT, description TEXT, done INTEGER, created_at TEXT)`,
		"notes_legacy":             `(id INTEGER, task_id INTEGER, content TEXT, created_at TEXT)`,
		"tags_legacy":              `(id INTEGER, name TEXT)`,
		"task_tags_legacy":         `(task_id INTEGER, tag_id INTEGER)`,
		"project_tags_legacy":      `(project_id INTEGER, tag_id INTEGER)`,
		"task_dependencies_legacy": `(task_id INTEGER, blocks_task_id INTEGER)`,
		"task_deps_legacy":         `(blocker_id INTEGER, blocked_id INTEGER)`,
	} {
		if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS ` + name + ` ` + cols); err != nil {
			return err
		}
	}

	copies := []string{
		`INSERT INTO projects (id, name, description, tags, status, created_at, updated_at)
		SELECT p.id, p.name, '',
			COALESCE((SELECT GROUP_CONCAT(DISTINCT lower(trim(t.name)))
				FROM project_tags_legacy pt JOIN tags_legacy t ON t.id = pt.tag_id
				WHERE pt.project_id = p.id), ''),
			CASE WHEN lower(p.status) = 'archived' THEN 'archived' ELSE 'active' END,
			` + legacyTS("p.created_at") + `, ` + legacyTS("p.updated_at") + `
		FROM projects_legacy p`,

		`INSERT INTO tasks (id, project_id, title, description, status, priority, due_date, tags, created_at, updated_at, completed_at)
		SELECT t.id, t.project_id, t.title, COALESCE(t.description, ''),
			CASE replace(lower(t.status), ' ', '_')
				WHEN 'in_progress' THEN 'in_progress'
				WHEN 'inprogress'  THEN 'in_progress'
				WHEN 'done'        THEN 'done'
				WHEN 'completed'   THEN 'done'
				ELSE 'pending' END,
			CASE WHEN lower(COALESCE(t.priority, '')) IN ('low','medium','high','urgent')
				THEN lower(t.priority) ELSE NULL END,
			NULLIF(t.due_date, ''),
			COALESCE((SELECT GROUP_CONCAT(DISTINCT lower(trim(g.name)))
				FROM task_tags_legacy tt JOIN tags_legacy g ON g.id = tt.tag_id
				WHERE tt.task_id = t.id), ''),
			` + legacyTS("t.created_at") + `, ` + legacyTS("t.updated_at") + `,
			CASE WHEN lower(t.status) IN ('done', 'completed') THEN ` + legacyTS("t.updated_at") + ` ELSE NULL END
		FROM tasks_legacy t
		WHERE EXISTS (SELECT 1 FROM projects p WHERE p.id = t.project_id)`,

		// The old schema had no position or description; ids are creation-ordered.
		// description may not exist in the renamed table, so read it via a
		// generated column expression instead of a direct column reference.
		`INSERT INTO subtasks (id, task_id, title, description, done, position, created_at)
		SELECT s.id, s.task_id, s.title,
			(SELECT COALESCE(value,'') FROM (SELECT NULL AS value) WHERE 0
			 UNION ALL SELECT '' LIMIT 1),
			CASE WHEN s.done THEN 1 ELSE 0 END, s.id, ` + legacyTS("s.created_at") + `
		FROM subtasks_legacy s
		WHERE EXISTS (SELECT 1 FROM tasks t WHERE t.id = s.task_id)`,

		`INSERT INTO notes (id, task_id, body, created_at)
		SELECT n.id, n.task_id, n.content, ` + legacyTS("n.created_at") + `
		FROM notes_legacy n
		WHERE EXISTS (SELECT 1 FROM tasks t WHERE t.id = n.task_id)`,

		`INSERT OR IGNORE INTO task_deps (blocker_id, blocked_id)
		SELECT d.task_id, d.blocks_task_id FROM task_dependencies_legacy d
		WHERE d.task_id != d.blocks_task_id
			AND EXISTS (SELECT 1 FROM tasks t WHERE t.id = d.task_id)
			AND EXISTS (SELECT 1 FROM tasks t WHERE t.id = d.blocks_task_id)`,

		`INSERT OR IGNORE INTO task_deps (blocker_id, blocked_id)
		SELECT d.blocker_id, d.blocked_id FROM task_deps_legacy d
		WHERE d.blocker_id != d.blocked_id
			AND EXISTS (SELECT 1 FROM tasks t WHERE t.id = d.blocker_id)
			AND EXISTS (SELECT 1 FROM tasks t WHERE t.id = d.blocked_id)`,
	}
	for _, q := range copies {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}

	// Children before parents so foreign keys don't block the drops.
	for _, name := range []string{
		"task_tags_legacy", "project_tags_legacy", "task_dependencies_legacy",
		"task_deps_legacy", "notes_legacy", "subtasks_legacy",
		"tags_legacy", "tasks_legacy", "projects_legacy",
	} {
		if _, err := tx.Exec(`DROP TABLE ` + name); err != nil {
			return fmt.Errorf("drop %s: %w", name, err)
		}
	}

	if _, err := tx.Exec(`INSERT INTO meta (key, value) VALUES ('schema_version', '2')
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`); err != nil {
		return err
	}
	return tx.Commit()
}
