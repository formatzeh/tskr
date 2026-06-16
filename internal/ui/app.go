// Package ui wires the panels and modals into the root Bubble Tea model.
package ui

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"tskr/internal/config"
	"tskr/internal/store"
	"tskr/internal/ui/confirm"
	"tskr/internal/ui/depsel"
	"tskr/internal/ui/detail"
	"tskr/internal/ui/forms"
	"tskr/internal/ui/help"
	"tskr/internal/ui/kanban"
	"tskr/internal/ui/notification"
	"tskr/internal/ui/keys"
	"tskr/internal/ui/msgs"
	"tskr/internal/ui/picker"
	"tskr/internal/ui/selectmenu"
	"tskr/internal/ui/styles"
	"tskr/internal/ui/tasklist"
)

type panel int

const (
	panelTasks panel = iota
	panelDetail
)

type viewMode int

const (
	viewList viewMode = iota
	viewKanban
)

type clearStatusMsg struct{}

type Model struct {
	st      *store.Store
	cfg     *config.Config
	cfgPath string

	w, h    int
	project *store.Project
	focus   panel
	view    viewMode

	tl     tasklist.Model
	dt     detail.Model
	kb     kanban.Model
	modals []tea.Model

	status    string
	statusErr bool
}

func New(st *store.Store, cfg *config.Config, cfgPath string) Model {
	m := Model{st: st, cfg: cfg, cfgPath: cfgPath, tl: tasklist.New(st), dt: detail.New(st), kb: kanban.New(st)}
	if cfg.View == "kanban" {
		m.view = viewKanban
	}
	opened := false
	if cfg.Startup == "last-project" {
		if idStr, _ := st.GetMeta("last_project_id"); idStr != "" {
			if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
				if p, err := st.GetProject(id); err == nil && p.Status == store.ProjectActive {
					m.openProject(p)
					opened = true
				}
			}
		}
	}
	if !opened {
		m.pushModal(m.newPicker(false))
	}
	return m
}

func (m *Model) newPicker(allowClose bool) tea.Model { return picker.New(m.st, allowClose) }

func (m *Model) sendNotifNow(n store.Notification) {
	go func() {
		u := "--urgency=" + n.Urgency
		title := n.Title
		if title == "" {
			title = "tskr"
		}
		exec.Command("notify-send", u, title, n.Body).Run()
	}()
}

func (m *Model) pushModal(mod tea.Model) { m.modals = append(m.modals, mod) }

func (m *Model) openProject(p store.Project) {
	m.project = &p
	m.tl.SetProject(p.ID)
	m.kb.SetProject(p.ID)
	m.focus = panelTasks
	m.tl.Focused = true
	m.dt.Focused = false
	m.syncDetail()
	m.st.SetMeta("last_project_id", strconv.FormatInt(p.ID, 10))
}

func (m *Model) syncDetail() {
	if t := m.tl.Selected(); t != nil {
		m.dt.SetTask(t.ID)
	} else {
		m.dt.Clear()
	}
}

func (m *Model) layout() {
	if m.w == 0 || m.project == nil {
		return
	}
	bodyH := m.h - 2    // tab bar + status bar
	innerH := bodyH - 3 // borders + panel title line
	leftW := int(float64(m.w) * m.cfg.SplitRatio)
	m.tl.SetSize(leftW-2, innerH)
	m.dt.SetSize(m.w-leftW-2, innerH)
	m.kb.SetSize(m.w, bodyH)
}

func (m Model) Init() tea.Cmd { return notifTickCmd() }

func notifTickCmd() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return msgs.NotifTick{}
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.layout()
		return m, nil

	case msgs.Status:
		m.status, m.statusErr = msg.Text, msg.Error
		return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{} })

	case clearStatusMsg:
		m.status = ""
		return m, nil

	case msgs.CloseModal:
		if len(m.modals) > 0 {
			m.modals = m.modals[:len(m.modals)-1]
		}
		// Never strand the user with no project and no modal (e.g. after
		// deleting the current project from the picker).
		if len(m.modals) == 0 && m.project == nil {
			m.pushModal(m.newPicker(false))
		}
		return m, nil

	case msgs.OpenProject:
		p, err := m.st.GetProject(msg.ID)
		if err != nil {
			return m, msgs.Err(err)
		}
		m.modals = nil
		m.openProject(p)
		m.layout()
		return m, nil

	case msgs.Refresh:
		var cmds []tea.Cmd
		if m.project != nil {
			if cmd := m.tl.Update(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
			m.kb.SetSort(m.tl.Sort())
			if cmd := m.kb.Update(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
			if t := m.tl.Selected(); t != nil {
				if m.dt.Task == nil || m.dt.Task.ID != t.ID {
					m.dt.SetTask(t.ID)
				} else {
					m.dt.Reload()
				}
			} else {
				m.dt.Clear()
			}
		}
		for i := range m.modals {
			var cmd tea.Cmd
			m.modals[i], cmd = m.modals[i].Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)

	case confirmDeleteNotifMsg:
		if len(m.modals) > 0 {
			m.modals = m.modals[:len(m.modals)-1]
		}
		m.pushModal(confirm.New("Delete notification?", []string{"This cannot be undone."}, false,
			func(bool) tea.Msg { return deleteNotifMsg{id: msg.id} },
		))
		return m, nil

	case msgs.NotifTick:
		notifs, err := m.st.DueNotifications()
		if err == nil {
			for _, n := range notifs {
				m.st.MarkNotificationSent(n.ID)
				m.sendNotifNow(n)
			}
		}
		return m, notifTickCmd()

	case msgs.NewProjectForm:
		m.pushModal(forms.ProjectForm("New project", nil, func(name, desc, tags string) tea.Msg {
			return saveProjectMsg{name: name, desc: desc, tags: tags}
		}))
		return m, nil

	case msgs.EditProjectForm:
		p := msg.Project
		m.pushModal(forms.ProjectForm("Edit project", &p, func(name, desc, tags string) tea.Msg {
			return saveProjectMsg{id: p.ID, name: name, desc: desc, tags: tags}
		}))
		return m, nil

	case msgs.DeleteProject:
		p := msg.Project
		m.pushModal(confirm.New(
			fmt.Sprintf("Delete project %q?", p.Name),
			[]string{fmt.Sprintf("This deletes the project and its %d task(s).", p.TaskCount)},
			false,
			func(bool) tea.Msg { return deleteProjectMsg{id: p.ID} },
		))
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	if cmd, ok := m.handleAction(msg); ok {
		return m, cmd
	}
	return m, nil
}

func (m Model) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if n := len(m.modals); n > 0 {
		var cmd tea.Cmd
		m.modals[n-1], cmd = m.modals[n-1].Update(key)
		return m, cmd
	}
	s := key.String()

	// While typing in the task list search, every key belongs to it.
	if m.focus == panelTasks && m.tl.Searching() {
		cmd := m.tl.Update(key)
		m.syncDetail()
		return m, cmd
	}

	switch s {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "?":
		m.pushModal(help.New(m.w, m.h))
		return m, nil
	case "p":
		m.pushModal(m.newPicker(m.project != nil))
		return m, nil
	case "v":
		if m.project != nil {
			return m.toggleView()
		}
		return m, nil
	case "<", ">":
		delta := -0.05
		if s == ">" {
			delta = 0.05
		}
		r := m.cfg.SplitRatio + delta
		if r < 0.2 {
			r = 0.2
		}
		if r > 0.8 {
			r = 0.8
		}
		m.cfg.SplitRatio = r
		config.Save(m.cfgPath, *m.cfg)
		m.layout()
		return m, nil
	}

	if m.project == nil {
		return m, nil
	}
	if m.view == viewKanban {
		return m.handleKanbanKey(s, key)
	}
	if m.focus == panelTasks {
		return m.handleTasksKey(s, key)
	}
	return m.handleDetailKey(s, key)
}

// toggleView switches between the list and kanban views, persisting the
// choice and reloading the newly active component.
func (m Model) toggleView() (tea.Model, tea.Cmd) {
	if m.view == viewKanban {
		m.view = viewList
		m.cfg.View = "list"
		m.focus = panelTasks
		m.tl.Focused = true
		m.dt.Focused = false
		m.tl.Reload()
		m.syncDetail()
	} else {
		m.view = viewKanban
		m.cfg.View = "kanban"
		m.kb.SetSort(m.tl.Sort())
		m.kb.Reload()
	}
	config.Save(m.cfgPath, *m.cfg)
	m.layout()
	return m, nil
}

func nextStatus(s store.TaskStatus) store.TaskStatus {
	switch s {
	case store.StatusPending:
		return store.StatusInProgress
	case store.StatusInProgress:
		return store.StatusDone
	default:
		return store.StatusPending
	}
}

func (m Model) handleTasksKey(s string, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	sel := m.tl.Selected()
	switch s {
	case "enter":
		if sel != nil {
			m.focus = panelDetail
			m.tl.Focused = false
			m.dt.Focused = true
			m.dt.Reload()
		}
		return m, nil
	case "a":
		m.addTaskModal()
		return m, nil
	case "e":
		if sel != nil {
			m.editTaskModal(*sel)
		}
		return m, nil
	case "d":
		if sel != nil {
			return m, m.deleteTaskModal(*sel)
		}
		return m, nil
	case "s":
		if sel != nil {
			if cmd, ok := m.handleAction(setStatusMsg{id: sel.ID, status: nextStatus(sel.Status)}); ok {
				return m, cmd
			}
		}
		return m, nil
	case "S":
		if sel != nil {
			m.statusMenuModal(*sel)
		}
		return m, nil
	default:
		cmd := m.tl.Update(key)
		m.syncDetail()
		return m, cmd
	}
}

// Shared task-action modals, used by both the list and kanban handlers.

func (m *Model) addTaskModal() {
	pid := m.project.ID
	m.pushModal(forms.TaskForm("New task", nil, func(title, desc string, prio store.Priority, due, tags string) tea.Msg {
		return saveTaskMsg{projectID: pid, title: title, desc: desc, prio: prio, due: due, tags: tags}
	}))
}

func (m *Model) editTaskModal(t store.Task) {
	m.pushModal(forms.TaskForm("Edit task", &t, func(title, desc string, prio store.Priority, due, tags string) tea.Msg {
		return saveTaskMsg{id: t.ID, title: title, desc: desc, prio: prio, due: due, tags: tags}
	}))
}

func (m *Model) deleteTaskModal(t store.Task) tea.Cmd {
	full, err := m.st.GetTask(t.ID)
	if err != nil {
		return msgs.Err(err)
	}
	lines := []string{"This cannot be undone."}
	cascadeOnly := len(full.Blocks) > 0
	if cascadeOnly {
		lines = []string{"This task blocks:"}
		for _, r := range full.Blocks {
			lines = append(lines, "  ⛔ "+r.Title)
		}
		lines = append(lines, "", "Cascade removes these dependency links (the tasks survive).")
	}
	m.pushModal(confirm.New(fmt.Sprintf("Delete task %q?", t.Title), lines, cascadeOnly,
		func(cascade bool) tea.Msg { return deleteTaskMsg{id: t.ID, cascade: cascade} }))
	return nil
}

func (m *Model) statusMenuModal(t store.Task) {
	id := t.ID
	statuses := []store.TaskStatus{store.StatusPending, store.StatusInProgress, store.StatusDone}
	initial := 0
	for i, st := range statuses {
		if st == t.Status {
			initial = i
		}
	}
	m.pushModal(selectmenu.New("Set status", []string{"Pending", "In Progress", "Done"}, initial,
		func(i int) tea.Msg { return setStatusMsg{id: id, status: statuses[i]} }))
}

func (m Model) handleKanbanKey(s string, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	sel := m.kb.Selected()
	switch s {
	case "enter":
		// Hand off to the list+detail view, landing on this card.
		if sel != nil {
			m.view = viewList
			m.cfg.View = "list"
			config.Save(m.cfgPath, *m.cfg)
			m.tl.SwitchToStatus(sel.Status, sel.ID)
			m.tl.Reload()
			m.focus = panelDetail
			m.tl.Focused = false
			m.dt.Focused = true
			m.dt.SetTask(sel.ID)
			m.layout()
		}
		return m, nil
	case "H", "L":
		if sel != nil {
			delta := -1
			if s == "L" {
				delta = 1
			}
			target := m.kb.Col() + delta
			if target < 0 || target > 2 {
				return m, nil
			}
			m.kb.FocusTask(sel.ID) // keep the cursor on the moved card
			if cmd, ok := m.handleAction(setStatusMsg{id: sel.ID, status: kanban.Statuses[target]}); ok {
				return m, cmd
			}
		}
		return m, nil
	case "a":
		m.addTaskModal()
		return m, nil
	case "e":
		if sel != nil {
			m.editTaskModal(*sel)
		}
		return m, nil
	case "d":
		if sel != nil {
			return m, m.deleteTaskModal(*sel)
		}
		return m, nil
	case "s":
		if sel != nil {
			m.kb.FocusTask(sel.ID)
			if cmd, ok := m.handleAction(setStatusMsg{id: sel.ID, status: nextStatus(sel.Status)}); ok {
				return m, cmd
			}
		}
		return m, nil
	case "S":
		if sel != nil {
			m.statusMenuModal(*sel)
		}
		return m, nil
	default:
		return m, m.kb.Update(key)
	}
}

func notifID(n *store.Notification) int64 {
	if n != nil {
		return n.ID
	}
	return 0
}

func (m Model) handleDetailKey(s string, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	task := m.dt.Task
	if task == nil {
		m.focus = panelTasks
		m.tl.Focused = true
		m.dt.Focused = false
		return m, nil
	}
	switch s {
	case "esc":
		m.focus = panelTasks
		m.tl.Focused = true
		m.dt.Focused = false
		return m, nil
	case "a":
		tid := task.ID
		m.pushModal(forms.SubtaskForm("New subtask", nil, func(title, desc string) tea.Msg {
			return addSubtaskMsg{taskID: tid, title: title, description: desc}
		}))
		return m, nil
	case "n":
		tid := task.ID
		m.pushModal(forms.TextForm("New note", "Note", "", func(v string) tea.Msg {
			return addNoteMsg{taskID: tid, body: v}
		}))
		return m, nil
	case "N":
		if task != nil {
			tid := task.ID
			notif := m.dt.Notification()
			existingID := notifID(notif)
			m.pushModal(notification.New(*task, notif,
				func(title, body, urgency, mode, dueDate string, intervalMin int, triggerStatus string, active bool) tea.Msg {
					return saveNotifMsg{id: existingID, taskID: tid, title: title, body: body, urgency: urgency, mode: mode, dueDate: dueDate, intervalMin: intervalMin, triggerStatus: triggerStatus, active: active}
				},
				func(id int64) tea.Msg {
					return confirmDeleteNotifMsg{id: id}
				},
			))
		}
		return m, nil
	case "A":
		if task != nil {
			if notif := m.dt.Notification(); notif != nil {
				if cmd, ok := m.handleAction(toggleNotifActiveMsg{id: notif.ID}); ok {
					return m, cmd
				}
			}
		}
		return m, nil
	case "b":
		m.pushModal(depsel.New(m.st, *task))
		return m, nil
	case "e":
		if it, ok := m.dt.CurrentItem(); ok {
			switch it.Kind {
			case detail.ItemSubtask:
				st := m.dt.Subtask(it.Idx)
				m.pushModal(forms.SubtaskForm("Edit subtask", &st, func(title, desc string) tea.Msg {
					return editSubtaskMsg{id: st.ID, title: title, description: desc}
				}))
			case detail.ItemNote:
				n := m.dt.Note(it.Idx)
				m.pushModal(forms.TextForm("Edit note", "Note", n.Body, func(v string) tea.Msg {
					return editNoteMsg{id: n.ID, body: v}
				}))
			}
		}
		return m, nil
	case "d":
		if it, ok := m.dt.CurrentItem(); ok {
			var title string
			var action func(bool) tea.Msg
			switch it.Kind {
			case detail.ItemSubtask:
				st := m.dt.Subtask(it.Idx)
				title = fmt.Sprintf("Delete subtask %q?", st.Title)
				action = func(bool) tea.Msg { return deleteSubtaskMsg{id: st.ID} }
			case detail.ItemNote:
				title = "Delete this note?"
				n := m.dt.Note(it.Idx)
				action = func(bool) tea.Msg { return deleteNoteMsg{id: n.ID} }
			}
			m.pushModal(confirm.New(title, []string{"This cannot be undone."}, false, action))
		}
		return m, nil
	default:
		return m, m.dt.Update(key)
	}
}

func (m Model) View() string {
	if m.w == 0 {
		return ""
	}
	if n := len(m.modals); n > 0 {
		// Keep the bottom line for status text so store errors stay
		// visible during modal flows (modals render their own key hints).
		body := lipgloss.Place(m.w, m.h-1, lipgloss.Center, lipgloss.Center, m.modals[n-1].View())
		return lipgloss.JoinVertical(lipgloss.Left, body, m.renderStatusText())
	}
	if m.project == nil {
		return ""
	}
	if m.view == viewKanban {
		return lipgloss.JoinVertical(lipgloss.Left, m.renderKanbanHeader(), m.kb.View(), m.renderStatusBar())
	}
	tabs := m.renderTabs()
	leftW := int(float64(m.w) * m.cfg.SplitRatio)
	bodyH := m.h - 2
	left := renderPanel("1: Tasks "+m.sortSuffix(), m.tl.View(), leftW, bodyH, m.focus == panelTasks)
	right := renderPanel("2: Details", m.dt.View(), m.w-leftW, bodyH, m.focus == panelDetail)
	panels := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	return lipgloss.JoinVertical(lipgloss.Left, tabs, panels, m.renderStatusBar())
}

func (m Model) sortSuffix() string {
	return styles.Label.Render("(" + string(m.tl.Sort()) + ")")
}

func renderPanel(title, content string, w, h int, focused bool) string {
	st := styles.Panel
	titleStyle := styles.Label
	if focused {
		st = styles.PanelFocused
		titleStyle = styles.Cyan
	}
	inner := titleStyle.Render(title) + "\n" + content
	return st.Width(w - 2).Height(h - 2).Render(inner)
}

func (m Model) renderTabs() string {
	var parts []string
	for i := tasklist.TabAll; i <= tasklist.TabDone; i++ {
		if i == m.tl.CurrentTab() {
			parts = append(parts, styles.TabActive.Render(i.String()))
		} else {
			parts = append(parts, styles.TabInactive.Render(i.String()))
		}
	}
	tabs := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	name := styles.Tag.Render(m.project.Name)
	pad := m.w - lipgloss.Width(tabs) - lipgloss.Width(name) - 1
	if pad < 1 {
		pad = 1
	}
	return tabs + strings.Repeat(" ", pad) + name
}

func (m Model) renderKanbanHeader() string {
	label := styles.TabActive.Render("Board")
	name := styles.Tag.Render(m.project.Name)
	pad := m.w - lipgloss.Width(label) - lipgloss.Width(name) - 1
	if pad < 1 {
		pad = 1
	}
	return label + strings.Repeat(" ", pad) + name
}

func (m Model) renderStatusText() string {
	if m.status == "" {
		return ""
	}
	st := styles.Ok
	if m.statusErr {
		st = styles.Err
	}
	return " " + st.Render(m.status)
}

func (m Model) renderStatusBar() string {
	if m.status != "" {
		return m.renderStatusText()
	}
	var hints []keys.Hint
	switch {
	case m.view == viewKanban:
		hints = keys.Kanban
	case m.focus == panelDetail:
		hints = keys.Detail
	case m.tl.Searching():
		hints = keys.Search
	default:
		hints = keys.TaskList
	}
	bar := " " + keys.Render(hints)
	if lipgloss.Width(bar) > m.w {
		bar = " " + keys.Render(hints[:6])
	}
	return bar
}
