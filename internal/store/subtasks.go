package store

import "database/sql"

func (s *Store) AddSubtask(taskID int64, title string) (int64, error) {
	var max sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(position) FROM subtasks WHERE task_id=?`, taskID).Scan(&max); err != nil {
		return 0, err
	}
	res, err := s.db.Exec(`INSERT INTO subtasks (task_id, title, done, position, created_at)
		VALUES (?, ?, 0, ?, ?)`, taskID, title, max.Int64+1, now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateSubtask(id int64, title string) error {
	_, err := s.db.Exec(`UPDATE subtasks SET title=? WHERE id=?`, title, id)
	return err
}

func (s *Store) ToggleSubtask(id int64) error {
	_, err := s.db.Exec(`UPDATE subtasks SET done = 1 - done WHERE id=?`, id)
	return err
}

func (s *Store) DeleteSubtask(id int64) error {
	_, err := s.db.Exec(`DELETE FROM subtasks WHERE id=?`, id)
	return err
}

func (s *Store) ListSubtasks(taskID int64) ([]Subtask, error) {
	rows, err := s.db.Query(`SELECT id, task_id, title, done, position, created_at
		FROM subtasks WHERE task_id=? ORDER BY position`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Subtask
	for rows.Next() {
		var st Subtask
		var done int
		if err := rows.Scan(&st.ID, &st.TaskID, &st.Title, &done, &st.Position, &st.CreatedAt); err != nil {
			return nil, err
		}
		st.Done = done == 1
		out = append(out, st)
	}
	return out, rows.Err()
}

// MoveSubtask swaps the subtask with its neighbor above (up=true) or
// below. Moving past the edge is a no-op.
func (s *Store) MoveSubtask(id int64, up bool) error {
	var taskID int64
	var pos int
	if err := s.db.QueryRow(`SELECT task_id, position FROM subtasks WHERE id=?`, id).Scan(&taskID, &pos); err != nil {
		return err
	}
	q := `SELECT id, position FROM subtasks WHERE task_id=? AND position > ? ORDER BY position LIMIT 1`
	if up {
		q = `SELECT id, position FROM subtasks WHERE task_id=? AND position < ? ORDER BY position DESC LIMIT 1`
	}
	var nid int64
	var npos int
	err := s.db.QueryRow(q, taskID, pos).Scan(&nid, &npos)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE subtasks SET position=? WHERE id=?`, npos, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE subtasks SET position=? WHERE id=?`, pos, nid); err != nil {
		return err
	}
	return tx.Commit()
}
