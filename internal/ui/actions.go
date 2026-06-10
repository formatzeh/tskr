package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/store"
	"tskr/internal/ui/msgs"
)

// Action messages are emitted by form/confirm closures and handled by
// the root model, which performs the store mutation and refreshes.

type saveTaskMsg struct {
	id        int64 // 0 = create
	projectID int64
	title     string
	desc      string
	prio      store.Priority
	due       string
	tags      string
}

type deleteTaskMsg struct {
	id      int64
	cascade bool
}

type setStatusMsg struct {
	id     int64
	status store.TaskStatus
}

type saveProjectMsg struct {
	id   int64 // 0 = create
	name string
	desc string
	tags string
}

type deleteProjectMsg struct{ id int64 }

type addSubtaskMsg struct {
	taskID int64
	title  string
}
type editSubtaskMsg struct {
	id    int64
	title string
}
type deleteSubtaskMsg struct{ id int64 }

type addNoteMsg struct {
	taskID int64
	body   string
}
type editNoteMsg struct {
	id   int64
	body string
}
type deleteNoteMsg struct{ id int64 }

type addTimeMsg struct {
	taskID  int64
	minutes int
	note    string
}
type editTimeMsg struct {
	id      int64
	minutes int
	note    string
}
type deleteTimeMsg struct{ id int64 }

// handleAction performs the store mutation for an action message.
// It returns ok=false when the message is not an action.
func (m *Model) handleAction(msg tea.Msg) (tea.Cmd, bool) {
	var err error
	info := ""
	switch a := msg.(type) {
	case saveTaskMsg:
		if a.id == 0 {
			_, err = m.st.CreateTask(a.projectID, a.title, a.desc, a.prio, a.due, a.tags)
			info = "task created"
		} else {
			err = m.st.UpdateTask(a.id, a.title, a.desc, a.prio, a.due, a.tags)
			info = "task updated"
		}
	case deleteTaskMsg:
		if a.cascade {
			err = m.st.DeleteTaskCascade(a.id)
		} else {
			err = m.st.DeleteTask(a.id)
		}
		info = "task deleted"
	case setStatusMsg:
		err = m.st.SetTaskStatus(a.id, a.status)
		info = "status: " + string(a.status)
	case saveProjectMsg:
		if a.id == 0 {
			_, err = m.st.CreateProject(a.name, a.desc, a.tags)
			info = "project created"
		} else {
			err = m.st.UpdateProject(a.id, a.name, a.desc, a.tags)
			info = "project updated"
			if m.project != nil && m.project.ID == a.id {
				if p, gerr := m.st.GetProject(a.id); gerr == nil {
					m.project = &p
				}
			}
		}
	case deleteProjectMsg:
		err = m.st.DeleteProject(a.id)
		info = "project deleted"
		if err == nil && m.project != nil && m.project.ID == a.id {
			m.project = nil
			m.pushModal(m.newPicker(false))
		}
	case addSubtaskMsg:
		_, err = m.st.AddSubtask(a.taskID, a.title)
	case editSubtaskMsg:
		err = m.st.UpdateSubtask(a.id, a.title)
	case deleteSubtaskMsg:
		err = m.st.DeleteSubtask(a.id)
	case addNoteMsg:
		_, err = m.st.AddNote(a.taskID, a.body)
	case editNoteMsg:
		err = m.st.UpdateNote(a.id, a.body)
	case deleteNoteMsg:
		err = m.st.DeleteNote(a.id)
	case addTimeMsg:
		_, err = m.st.AddTimeEntry(a.taskID, a.minutes, a.note)
	case editTimeMsg:
		err = m.st.UpdateTimeEntry(a.id, a.minutes, a.note)
	case deleteTimeMsg:
		err = m.st.DeleteTimeEntry(a.id)
	default:
		return nil, false
	}
	if err != nil {
		return msgs.Err(err), true
	}
	cmds := []tea.Cmd{msgs.Cmd(msgs.Refresh{})}
	if info != "" {
		cmds = append(cmds, msgs.Info(info))
	}
	return tea.Batch(cmds...), true
}
