package store

func (s *Store) AddTimeEntry(taskID int64, minutes int, note string) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO time_entries (task_id, minutes, note, created_at)
		VALUES (?, ?, ?, ?)`, taskID, minutes, note, now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateTimeEntry(id int64, minutes int, note string) error {
	_, err := s.db.Exec(`UPDATE time_entries SET minutes=?, note=? WHERE id=?`, minutes, note, id)
	return err
}

func (s *Store) DeleteTimeEntry(id int64) error {
	_, err := s.db.Exec(`DELETE FROM time_entries WHERE id=?`, id)
	return err
}

func (s *Store) ListTimeEntries(taskID int64) ([]TimeEntry, error) {
	rows, err := s.db.Query(`SELECT id, task_id, minutes, note, created_at FROM time_entries
		WHERE task_id=? ORDER BY created_at DESC, id DESC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TimeEntry
	for rows.Next() {
		var e TimeEntry
		if err := rows.Scan(&e.ID, &e.TaskID, &e.Minutes, &e.Note, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
