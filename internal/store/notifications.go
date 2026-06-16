package store

import (
	"database/sql"
	"strings"
	"time"
)

const notifCols = `id, task_id, title, body, urgency, mode, due_date, interval_minutes, trigger_status, active, last_sent, created_at, updated_at`

func scanNotification(r scanner) (Notification, error) {
	var n Notification
	var active int
	err := r.Scan(&n.ID, &n.TaskID, &n.Title, &n.Body, &n.Urgency, &n.Mode,
		&n.DueDate, &n.IntervalMinutes, &n.TriggerStatus, &active,
		&n.LastSent, &n.CreatedAt, &n.UpdatedAt)
	n.Active = active == 1
	return n, err
}

func (s *Store) CreateNotification(taskID int64, title, body, urgency, mode, dueDate string, intervalMinutes int, triggerStatus string, active bool) (int64, error) {
	ts := now()
	a := 0
	if active {
		a = 1
	}
	res, err := s.db.Exec(`INSERT INTO notifications (task_id, title, body, urgency, mode, due_date, interval_minutes, trigger_status, active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		taskID, title, body, urgency, mode, dueDate, intervalMinutes, triggerStatus, a, ts, ts)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateNotification(id int64, title, body, urgency, mode, dueDate string, intervalMinutes int, triggerStatus string, active bool) error {
	a := 0
	if active {
		a = 1
	}
	_, err := s.db.Exec(`UPDATE notifications SET title=?, body=?, urgency=?, mode=?, due_date=?, interval_minutes=?, trigger_status=?, active=?, updated_at=? WHERE id=?`,
		title, body, urgency, mode, dueDate, intervalMinutes, triggerStatus, a, now(), id)
	return err
}

func (s *Store) SetNotificationActive(id int64, active bool) error {
	a := 0
	if active {
		a = 1
	}
	_, err := s.db.Exec(`UPDATE notifications SET active=?, updated_at=? WHERE id=?`, a, now(), id)
	return err
}

func (s *Store) DeleteNotification(id int64) error {
	res, err := s.db.Exec(`DELETE FROM notifications WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ListNotifications(taskID int64) ([]Notification, error) {
	rows, err := s.db.Query(`SELECT `+notifCols+` FROM notifications WHERE task_id=? ORDER BY created_at`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) GetNotification(id int64) (Notification, error) {
	row := s.db.QueryRow(`SELECT `+notifCols+` FROM notifications WHERE id=?`, id)
	return scanNotification(row)
}

// DueNotifications returns active notifications whose conditions are met.
func (s *Store) DueNotifications() ([]Notification, error) {
	nowTime := time.Now()
	nowStr := nowTime.Format("2006-01-02 15:04")
	todayStr := nowTime.Format("2006-01-02")
	timeStr := nowTime.Format("15:04")

	rows, err := s.db.Query(`SELECT ` + notifCols + ` FROM notifications WHERE active=1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all []Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		all = append(all, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var due []Notification
	for _, n := range all {
		if n.LastSent != "" && n.Mode == NotifOnce {
			continue
		}
		task, err := s.GetTask(n.TaskID)
		if err != nil {
			continue
		}
		if !statusMatches(n.TriggerStatus, task.Status) {
			continue
		}
		switch n.Mode {
		case NotifOnce:
			if n.DueDate <= nowStr && n.LastSent == "" {
				due = append(due, n)
			}
		case NotifRecurring:
			if n.DueDate != "" && n.DueDate <= timeStr {
				sentToday := false
				if n.LastSent != "" {
					sentDay, _ := time.Parse(time.RFC3339, n.LastSent)
					if sentDay.Format("2006-01-02") == todayStr {
						sentToday = true
					}
				}
				if !sentToday {
					due = append(due, n)
				}
			}
		case NotifInterval:
			if n.IntervalMinutes <= 0 {
				continue
			}
			if n.LastSent == "" {
				due = append(due, n)
			} else {
				last, err := time.Parse(time.RFC3339, n.LastSent)
				if err != nil {
					continue
				}
				if nowTime.Sub(last) >= time.Duration(n.IntervalMinutes)*time.Minute {
					due = append(due, n)
				}
			}
		}
	}
	return due, nil
}

func (s *Store) MarkNotificationSent(id int64) error {
	_, err := s.db.Exec(`UPDATE notifications SET last_sent=?, updated_at=? WHERE id=?`,
		now(), now(), id)
	return err
}

func (s *Store) UpsertNotification(id int64, taskID int64, title, body, urgency, mode, dueDate string, intervalMinutes int, triggerStatus string, active bool) (int64, error) {
	if id == 0 {
		return s.CreateNotification(taskID, title, body, urgency, mode, dueDate, intervalMinutes, triggerStatus, active)
	}
	err := s.UpdateNotification(id, title, body, urgency, mode, dueDate, intervalMinutes, triggerStatus, active)
	return id, err
}

func statusMatches(triggerStatus string, taskStatus TaskStatus) bool {
	for _, s := range strings.Split(triggerStatus, ",") {
		if strings.TrimSpace(s) == string(taskStatus) {
			return true
		}
	}
	return false
}
