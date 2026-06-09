package store

func (s *Store) CreateProject(name, description, tags string) (int64, error) {
	ts := now()
	res, err := s.db.Exec(`INSERT INTO projects (name, description, tags, status, created_at, updated_at)
		VALUES (?, ?, ?, 'active', ?, ?)`, name, description, NormalizeTags(tags), ts, ts)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateProject(id int64, name, description, tags string) error {
	_, err := s.db.Exec(`UPDATE projects SET name=?, description=?, tags=?, updated_at=? WHERE id=?`,
		name, description, NormalizeTags(tags), now(), id)
	return err
}

func (s *Store) SetProjectStatus(id int64, status ProjectStatus) error {
	_, err := s.db.Exec(`UPDATE projects SET status=?, updated_at=? WHERE id=?`, string(status), now(), id)
	return err
}

func (s *Store) DeleteProject(id int64) error {
	_, err := s.db.Exec(`DELETE FROM projects WHERE id=?`, id)
	return err
}

func (s *Store) GetProject(id int64) (Project, error) {
	var p Project
	var status string
	err := s.db.QueryRow(`SELECT id, name, description, tags, status, created_at, updated_at,
		(SELECT COUNT(*) FROM tasks t WHERE t.project_id = projects.id)
		FROM projects WHERE id=?`, id).
		Scan(&p.ID, &p.Name, &p.Description, &p.Tags, &status, &p.CreatedAt, &p.UpdatedAt, &p.TaskCount)
	p.Status = ProjectStatus(status)
	return p, err
}

func (s *Store) ListProjects(includeArchived bool) ([]Project, error) {
	q := `SELECT id, name, description, tags, status, created_at, updated_at,
		(SELECT COUNT(*) FROM tasks t WHERE t.project_id = projects.id)
		FROM projects`
	if !includeArchived {
		q += ` WHERE status = 'active'`
	}
	q += ` ORDER BY name COLLATE NOCASE`
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		var status string
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Tags, &status, &p.CreatedAt, &p.UpdatedAt, &p.TaskCount); err != nil {
			return nil, err
		}
		p.Status = ProjectStatus(status)
		out = append(out, p)
	}
	return out, rows.Err()
}
