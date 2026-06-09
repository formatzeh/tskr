package store

import "strings"

type ProjectStatus string

const (
	ProjectActive   ProjectStatus = "active"
	ProjectArchived ProjectStatus = "archived"
)

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
)

type Priority string

const (
	PriorityNone   Priority = ""
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

type Project struct {
	ID          int64
	Name        string
	Description string
	Tags        string
	Status      ProjectStatus
	CreatedAt   string
	UpdatedAt   string
	TaskCount   int
}

type Task struct {
	ID          int64
	ProjectID   int64
	Title       string
	Description string
	Status      TaskStatus
	Priority    Priority // "" = none
	DueDate     string   // "YYYY-MM-DD", "" = none
	Tags        string
	CreatedAt   string
	UpdatedAt   string
	CompletedAt string

	SubtasksDone  int
	SubtasksTotal int
	NoteCount     int
	Minutes       int
	Blocked       bool
	BlockedBy     []TaskRef
	Blocks        []TaskRef
}

type TaskRef struct {
	ID     int64
	Title  string
	Status TaskStatus
}

type Subtask struct {
	ID        int64
	TaskID    int64
	Title     string
	Done      bool
	Position  int
	CreatedAt string
}

type Note struct {
	ID        int64
	TaskID    int64
	Body      string
	CreatedAt string
}

type TimeEntry struct {
	ID        int64
	TaskID    int64
	Minutes   int
	Note      string
	CreatedAt string
}

// Overdue reports whether the task is past due. today is "YYYY-MM-DD".
func (t Task) Overdue(today string) bool {
	return t.DueDate != "" && t.DueDate < today && t.Status != StatusDone
}

// NormalizeTags trims, lowercases, and dedupes a comma-separated tag string.
func NormalizeTags(s string) string {
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.Split(s, ",") {
		tag := strings.ToLower(strings.TrimSpace(part))
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	return strings.Join(out, ",")
}
