package forms

import (
	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/store"
	"tskr/internal/ui/timefmt"
)

// TaskForm builds the new/edit task form. Values reach submit in the
// order: title, description, priority, due date, tags.
func TaskForm(title string, t *store.Task, submit func(title, desc string, prio store.Priority, due, tags string) tea.Msg) Model {
	var v store.Task
	if t != nil {
		v = *t
	}
	prioLabels := []string{"None", "Low", "Medium", "High", "Urgent"}
	prioValues := []string{"", "low", "medium", "high", "urgent"}
	prioIdx := 0
	for i, pv := range prioValues {
		if string(v.Priority) == pv {
			prioIdx = i
		}
	}
	fields := []Field{
		NewField("Title", v.Title, "task title", Required),
		NewField("Description", v.Description, "", nil),
		NewSelectField("Priority", prioLabels, prioValues, prioIdx),
		NewField("Due date", v.DueDate, "YYYY-MM-DD — empty = none", OptionalDate),
		NewField("Tags", v.Tags, "comma,separated", nil),
	}
	return New(title, fields, func(vals []string) tea.Msg {
		return submit(vals[0], vals[1], store.Priority(vals[2]), vals[3], vals[4])
	})
}

// ProjectForm builds the new/edit project form: name, description, tags.
func ProjectForm(title string, p *store.Project, submit func(name, desc, tags string) tea.Msg) Model {
	var v store.Project
	if p != nil {
		v = *p
	}
	fields := []Field{
		NewField("Name", v.Name, "project name", Required),
		NewField("Description", v.Description, "", nil),
		NewField("Tags", v.Tags, "comma,separated", nil),
	}
	return New(title, fields, func(vals []string) tea.Msg {
		return submit(vals[0], vals[1], vals[2])
	})
}

// SubtaskForm builds the new/edit subtask form: required title, optional description.
func SubtaskForm(title string, st *store.Subtask, submit func(title, description string) tea.Msg) Model {
	var v store.Subtask
	if st != nil {
		v = *st
	}
	fields := []Field{
		NewField("Title", v.Title, "subtask title", Required),
		NewField("Description", v.Description, "optional", nil),
	}
	return New(title, fields, func(vals []string) tea.Msg {
		return submit(vals[0], vals[1])
	})
}

// TextForm is a single required text field (note body).
func TextForm(title, label, value string, submit func(string) tea.Msg) Model {
	fields := []Field{NewField(label, value, "", Required)}
	return New(title, fields, func(vals []string) tea.Msg { return submit(vals[0]) })
}

// TimeForm asks for a duration and an optional note. Pass minutes=0 for
// a new entry, or the current values when editing.
func TimeForm(title string, minutes int, note string, submit func(minutes int, note string) tea.Msg) Model {
	val := ""
	if minutes > 0 {
		val = timefmt.FormatMinutes(minutes)
	}
	fields := []Field{
		NewField("Duration", val, "e.g. 1h 30m", ValidDuration),
		NewField("Note", note, "optional", nil),
	}
	return New(title, fields, func(vals []string) tea.Msg {
		mins, _ := timefmt.ParseDuration(vals[0]) // validated above
		return submit(mins, vals[1])
	})
}
