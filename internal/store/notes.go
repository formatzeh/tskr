package store

func (s *Store) AddNote(taskID int64, body string) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO notes (task_id, body, created_at) VALUES (?, ?, ?)`,
		taskID, body, now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateNote(id int64, body string) error {
	_, err := s.db.Exec(`UPDATE notes SET body=? WHERE id=?`, body, id)
	return err
}

func (s *Store) DeleteNote(id int64) error {
	_, err := s.db.Exec(`DELETE FROM notes WHERE id=?`, id)
	return err
}

func (s *Store) ListNotes(taskID int64) ([]Note, error) {
	rows, err := s.db.Query(`SELECT id, task_id, body, created_at FROM notes
		WHERE task_id=? ORDER BY created_at, id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.TaskID, &n.Body, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
