package store

import "errors"

var (
	ErrCycle        = errors.New("dependency would create a cycle")
	ErrCrossProject = errors.New("tasks must belong to the same project")
	ErrSelf         = errors.New("a task cannot block itself")
)

// AddDependency records that blocker blocks blocked.
func (s *Store) AddDependency(blockerID, blockedID int64) error {
	if blockerID == blockedID {
		return ErrSelf
	}
	var p1, p2 int64
	if err := s.db.QueryRow(`SELECT project_id FROM tasks WHERE id=?`, blockerID).Scan(&p1); err != nil {
		return err
	}
	if err := s.db.QueryRow(`SELECT project_id FROM tasks WHERE id=?`, blockedID).Scan(&p2); err != nil {
		return err
	}
	if p1 != p2 {
		return ErrCrossProject
	}
	cyclic, err := s.reachable(blockedID, blockerID)
	if err != nil {
		return err
	}
	if cyclic {
		return ErrCycle
	}
	_, err = s.db.Exec(`INSERT OR IGNORE INTO task_deps (blocker_id, blocked_id) VALUES (?, ?)`,
		blockerID, blockedID)
	return err
}

func (s *Store) RemoveDependency(blockerID, blockedID int64) error {
	_, err := s.db.Exec(`DELETE FROM task_deps WHERE blocker_id=? AND blocked_id=?`, blockerID, blockedID)
	return err
}

// reachable reports whether `to` can be reached from `from` by walking
// blocker → blocked edges (i.e. whether `from` transitively blocks `to`).
func (s *Store) reachable(from, to int64) (bool, error) {
	stack := []int64{from}
	seen := map[int64]bool{}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n == to {
			return true, nil
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		rows, err := s.db.Query(`SELECT blocked_id FROM task_deps WHERE blocker_id=?`, n)
		if err != nil {
			return false, err
		}
		for rows.Next() {
			var next int64
			if err := rows.Scan(&next); err != nil {
				rows.Close()
				return false, err
			}
			stack = append(stack, next)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return false, err
		}
		rows.Close()
	}
	return false, nil
}
