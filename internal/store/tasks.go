package store

import (
	"database/sql"
	"fmt"
)

type SortMode string

const (
	SortCreated  SortMode = "created"
	SortDue      SortMode = "due"
	SortPriority SortMode = "priority"
)

// NextSort cycles created → due → priority → created.
func NextSort(m SortMode) SortMode {
	switch m {
	case SortCreated:
		return SortDue
	case SortDue:
		return SortPriority
	default:
		return SortCreated
	}
}

// BlocksError is returned by DeleteTask when the task blocks others.
type BlocksError struct {
	Blocked []TaskRef
}

func (e *BlocksError) Error() string {
	return fmt.Sprintf("task blocks %d other task(s)", len(e.Blocked))
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s *Store) CreateTask(projectID int64, title, description string, priority Priority, dueDate, tags string) (int64, error) {
	ts := now()
	res, err := s.db.Exec(`INSERT INTO tasks (project_id, title, description, status, priority, due_date, tags, created_at, updated_at)
		VALUES (?, ?, ?, 'pending', ?, ?, ?, ?, ?)`,
		projectID, title, description, nullStr(string(priority)), nullStr(dueDate), NormalizeTags(tags), ts, ts)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateTask(id int64, title, description string, priority Priority, dueDate, tags string) error {
	_, err := s.db.Exec(`UPDATE tasks SET title=?, description=?, priority=?, due_date=?, tags=?, updated_at=? WHERE id=?`,
		title, description, nullStr(string(priority)), nullStr(dueDate), NormalizeTags(tags), now(), id)
	return err
}

func (s *Store) SetTaskStatus(id int64, status TaskStatus) error {
	var completed any
	if status == StatusDone {
		completed = now()
	}
	_, err := s.db.Exec(`UPDATE tasks SET status=?, completed_at=?, updated_at=? WHERE id=?`,
		string(status), completed, now(), id)
	return err
}

const taskCols = `t.id, t.project_id, t.title, t.description, t.status,
	COALESCE(t.priority,''), COALESCE(t.due_date,''), t.tags, t.created_at, t.updated_at, COALESCE(t.completed_at,''),
	(SELECT COUNT(*) FROM subtasks st WHERE st.task_id=t.id AND st.done=1),
	(SELECT COUNT(*) FROM subtasks st WHERE st.task_id=t.id),
	(SELECT COUNT(*) FROM notes n WHERE n.task_id=t.id),
	COALESCE((SELECT SUM(e.minutes) FROM time_entries e WHERE e.task_id=t.id), 0),
	EXISTS(SELECT 1 FROM task_deps d JOIN tasks b ON b.id=d.blocker_id WHERE d.blocked_id=t.id AND b.status!='done')`

type scanner interface {
	Scan(dest ...any) error
}

func scanTask(r scanner) (Task, error) {
	var t Task
	var status, prio string
	var blocked int
	err := r.Scan(&t.ID, &t.ProjectID, &t.Title, &t.Description, &status,
		&prio, &t.DueDate, &t.Tags, &t.CreatedAt, &t.UpdatedAt, &t.CompletedAt,
		&t.SubtasksDone, &t.SubtasksTotal, &t.NoteCount, &t.Minutes, &blocked)
	t.Status = TaskStatus(status)
	t.Priority = Priority(prio)
	t.Blocked = blocked == 1
	return t, err
}

func (s *Store) GetTask(id int64) (Task, error) {
	row := s.db.QueryRow(`SELECT `+taskCols+` FROM tasks t WHERE t.id=?`, id)
	t, err := scanTask(row)
	if err != nil {
		return t, err
	}
	if t.BlockedBy, err = s.taskRefs(`SELECT b.id, b.title, b.status FROM task_deps d
		JOIN tasks b ON b.id=d.blocker_id WHERE d.blocked_id=? ORDER BY b.title COLLATE NOCASE`, id); err != nil {
		return t, err
	}
	if t.Blocks, err = s.blocksRefs(id); err != nil {
		return t, err
	}
	return t, nil
}

func (s *Store) ListTasks(projectID int64, status TaskStatus, sort SortMode) ([]Task, error) {
	q := `SELECT ` + taskCols + ` FROM tasks t WHERE t.project_id = ?`
	args := []any{projectID}
	if status != "" {
		q += ` AND t.status = ?`
		args = append(args, string(status))
	}
	switch sort {
	case SortDue:
		q += ` ORDER BY (t.due_date IS NULL), t.due_date, t.id`
	case SortPriority:
		q += ` ORDER BY CASE t.priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 ELSE 4 END, t.id`
	default:
		q += ` ORDER BY t.created_at, t.id`
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// blocksRefs returns the tasks that the given task blocks.
func (s *Store) blocksRefs(id int64) ([]TaskRef, error) {
	return s.taskRefs(`SELECT b.id, b.title, b.status FROM task_deps d
		JOIN tasks b ON b.id=d.blocked_id WHERE d.blocker_id=? ORDER BY b.title COLLATE NOCASE`, id)
}

func (s *Store) taskRefs(query string, id int64) ([]TaskRef, error) {
	rows, err := s.db.Query(query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var refs []TaskRef
	for rows.Next() {
		var r TaskRef
		var st string
		if err := rows.Scan(&r.ID, &r.Title, &st); err != nil {
			return nil, err
		}
		r.Status = TaskStatus(st)
		refs = append(refs, r)
	}
	return refs, rows.Err()
}

// DeleteTask refuses (with *BlocksError) if the task blocks others.
func (s *Store) DeleteTask(id int64) error {
	blocked, err := s.blocksRefs(id)
	if err != nil {
		return err
	}
	if len(blocked) > 0 {
		return &BlocksError{Blocked: blocked}
	}
	return s.deleteTaskRow(id)
}

// DeleteTaskCascade removes the task's dependency links (in both
// directions), then deletes the task. Dependent tasks survive.
func (s *Store) DeleteTaskCascade(id int64) error {
	// task_deps rows would also be removed by the FK ON DELETE CASCADE;
	// the explicit delete is kept as a safety net in case FK enforcement
	// is ever off for a connection.
	if _, err := s.db.Exec(`DELETE FROM task_deps WHERE blocker_id=? OR blocked_id=?`, id, id); err != nil {
		return err
	}
	return s.deleteTaskRow(id)
}

func (s *Store) deleteTaskRow(id int64) error {
	res, err := s.db.Exec(`DELETE FROM tasks WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
