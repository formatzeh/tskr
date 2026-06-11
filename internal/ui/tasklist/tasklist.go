// Package tasklist is the left panel: status tabs, task rows, fuzzy
// search, and sort cycling. Mutating actions (add/edit/delete/status)
// are handled by the root model.
package tasklist

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/store"
	"tskr/internal/ui/fuzzy"
	"tskr/internal/ui/msgs"
	"tskr/internal/ui/styles"
	"tskr/internal/ui/timefmt"
)

type Tab int

const (
	TabAll Tab = iota
	TabPending
	TabInProgress
	TabDone
)

func (t Tab) String() string {
	return [...]string{"All", "Pending", "In Progress", "Done"}[t]
}

// Status maps the tab to its store filter ("" = all).
func (t Tab) Status() store.TaskStatus {
	switch t {
	case TabPending:
		return store.StatusPending
	case TabInProgress:
		return store.StatusInProgress
	case TabDone:
		return store.StatusDone
	default:
		return ""
	}
}

type Model struct {
	st             *store.Store
	projectID      int64
	tab            Tab
	sort           store.SortMode
	search         textinput.Model
	searching      bool
	tasks          []store.Task
	visible        []store.Task
	sel            int
	pendingFocusID int64 // if >0, position cursor on this task ID after next Reload
	width          int
	height         int
	Focused        bool
}

func New(st *store.Store) Model {
	ti := textinput.New()
	ti.Placeholder = "search…"
	ti.Width = 24
	return Model{st: st, tab: TabPending, sort: store.SortCreated, search: ti, Focused: true}
}

func (m *Model) SetSize(w, h int) { m.width, m.height = w, h }

func (m *Model) SetProject(id int64) error {
	m.projectID = id
	m.sel = 0
	m.search.SetValue("")
	return m.Reload()
}

func (m *Model) Reload() error {
	tasks, err := m.st.ListTasks(m.projectID, m.tab.Status(), m.sort)
	if err != nil {
		return err
	}
	m.tasks = tasks
	m.applyFilter()
	if m.pendingFocusID > 0 {
		for i, t := range m.visible {
			if t.ID == m.pendingFocusID {
				m.sel = i
				break
			}
		}
		m.pendingFocusID = 0
	}
	return nil
}

// SwitchToStatus switches to the tab that matches status and marks focusID
// to be selected after the next Reload (which happens via msgs.Refresh).
func (m *Model) SwitchToStatus(status store.TaskStatus, focusID int64) {
	switch status {
	case store.StatusPending:
		m.tab = TabPending
	case store.StatusInProgress:
		m.tab = TabInProgress
	case store.StatusDone:
		m.tab = TabDone
	default:
		m.tab = TabAll
	}
	m.sel = 0
	m.pendingFocusID = focusID
}

func (m *Model) applyFilter() {
	q := m.search.Value()
	m.visible = m.visible[:0]
	for _, t := range m.tasks {
		if fuzzy.Match(q, t.Title+" "+t.Description+" "+t.Tags) {
			m.visible = append(m.visible, t)
		}
	}
	if m.sel >= len(m.visible) {
		m.sel = len(m.visible) - 1
	}
	if m.sel < 0 {
		m.sel = 0
	}
}

func (m Model) Selected() *store.Task {
	if len(m.visible) == 0 {
		return nil
	}
	return &m.visible[m.sel]
}

func (m Model) Visible() []store.Task { return m.visible }
func (m Model) CurrentTab() Tab       { return m.tab }
func (m Model) Sort() store.SortMode  { return m.sort }
func (m Model) Searching() bool       { return m.searching }

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case msgs.Refresh:
		if err := m.Reload(); err != nil {
			return msgs.Err(err)
		}
		return nil
	case tea.KeyMsg:
		if m.searching {
			switch msg.String() {
			case "enter":
				m.searching = false
				m.search.Blur()
			case "esc":
				m.searching = false
				m.search.Blur()
				m.search.SetValue("")
				m.applyFilter()
			default:
				var cmd tea.Cmd
				m.search, cmd = m.search.Update(msg)
				m.applyFilter()
				return cmd
			}
			return nil
		}
		switch msg.String() {
		case "j", "down":
			if m.sel < len(m.visible)-1 {
				m.sel++
			}
		case "k", "up":
			if m.sel > 0 {
				m.sel--
			}
		case "1", "2", "3", "4":
			m.tab = Tab(int(msg.String()[0] - '1'))
			m.sel = 0
			return m.reloadCmd()
		case "tab":
			m.tab = (m.tab + 1) % 4
			m.sel = 0
			return m.reloadCmd()
		case "shift+tab":
			m.tab = (m.tab + 3) % 4
			m.sel = 0
			return m.reloadCmd()
		case "o":
			m.sort = store.NextSort(m.sort)
			if err := m.Reload(); err != nil {
				return msgs.Err(err)
			}
			return msgs.Info("sort: " + string(m.sort))
		case "/":
			m.searching = true
			return m.search.Focus()
		case "esc":
			if m.search.Value() != "" {
				m.search.SetValue("")
				m.applyFilter()
			}
		}
	}
	return nil
}

func (m *Model) reloadCmd() tea.Cmd {
	if err := m.Reload(); err != nil {
		return msgs.Err(err)
	}
	return nil
}

// View renders the panel's inner content (the root model wraps it in
// the border and title).
func (m Model) View() string {
	var b strings.Builder
	if m.searching || m.search.Value() != "" {
		b.WriteString(styles.Cyan.Render("/ ") + m.search.View() + "\n")
	}
	if len(m.visible) == 0 {
		b.WriteString(styles.Label.Render("no tasks — press a to add one"))
		return b.String()
	}
	today := time.Now().Format("2006-01-02")
	rows := m.height / 2
	if rows < 1 {
		rows = 1
	}
	start := 0
	if m.sel >= rows {
		start = m.sel - rows + 1
	}
	end := start + rows
	if end > len(m.visible) {
		end = len(m.visible)
	}
	for i := start; i < end; i++ {
		t := m.visible[i]
		marker := "  "
		title := t.Title
		if i == m.sel {
			marker = styles.Cyan.Render(styles.Marker)
			title = styles.Title.Render(title)
		}
		line1 := marker + styles.PriorityBadge(t.Priority) + " " + title
		if t.Blocked {
			line1 += " " + styles.Red.Render("⛔")
		}
		var meta []string
		if t.SubtasksTotal > 0 {
			meta = append(meta, styles.Blue.Render(fmt.Sprintf("[%d/%d]", t.SubtasksDone, t.SubtasksTotal)))
		}
		if t.NoteCount > 0 {
			meta = append(meta, styles.Label.Render(fmt.Sprintf("[%d notes]", t.NoteCount)))
		}
		if t.Minutes > 0 {
			meta = append(meta, styles.Green.Render("["+timefmt.FormatMinutes(t.Minutes)+"]"))
		}
		if t.DueDate != "" {
			due := "due " + t.DueDate
			if t.Overdue(today) {
				meta = append(meta, styles.Red.Render(due+" (overdue!)"))
			} else {
				meta = append(meta, styles.Light.Render(due))
			}
		}
		b.WriteString(line1 + "\n      " + strings.Join(meta, "  ") + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
