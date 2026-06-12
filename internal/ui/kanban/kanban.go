// Package kanban is the full-screen board view: three status columns
// (Pending, In Progress, Done) with card navigation. Like tasklist, it
// owns navigation only — mutating actions (add/edit/delete/move) are
// handled by the root model.
package kanban

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"tskr/internal/store"
	"tskr/internal/ui/msgs"
	"tskr/internal/ui/styles"
)

// Statuses is the column order, left to right.
var Statuses = [3]store.TaskStatus{store.StatusPending, store.StatusInProgress, store.StatusDone}

var colTitles = [3]string{"Pending", "In Progress", "Done"}

type Model struct {
	st             *store.Store
	projectID      int64
	cols           [3][]store.Task // one bucket per status
	col, sel       int             // selected column / card-in-column
	sort           store.SortMode
	pendingFocusID int64 // if >0, re-select this task ID after next Reload
	width, height  int
	Focused        bool
}

func New(st *store.Store) Model {
	return Model{st: st, sort: store.SortCreated, Focused: true}
}

func (m *Model) SetSize(w, h int) { m.width, m.height = w, h }

func (m *Model) SetProject(id int64) error {
	m.projectID = id
	m.col, m.sel = 0, 0
	return m.Reload()
}

// SetSort keeps the board ordering in step with the list panel. The
// caller is expected to Reload afterwards (e.g. via msgs.Refresh).
func (m *Model) SetSort(s store.SortMode) { m.sort = s }

func (m *Model) Reload() error {
	for i, st := range Statuses {
		tasks, err := m.st.ListTasks(m.projectID, st, m.sort)
		if err != nil {
			return err
		}
		m.cols[i] = tasks
	}
	if m.pendingFocusID > 0 {
		if c, s, ok := m.find(m.pendingFocusID); ok {
			m.col, m.sel = c, s
		}
		m.pendingFocusID = 0
	}
	m.clamp()
	return nil
}

func (m *Model) find(id int64) (col, sel int, ok bool) {
	for c := range m.cols {
		for s, t := range m.cols[c] {
			if t.ID == id {
				return c, s, true
			}
		}
	}
	return 0, 0, false
}

func (m *Model) clamp() {
	if m.col < 0 {
		m.col = 0
	}
	if m.col > 2 {
		m.col = 2
	}
	if m.sel >= len(m.cols[m.col]) {
		m.sel = len(m.cols[m.col]) - 1
	}
	if m.sel < 0 {
		m.sel = 0
	}
}

// FocusTask re-selects the given task ID after the next Reload.
func (m *Model) FocusTask(id int64) { m.pendingFocusID = id }

func (m Model) Selected() *store.Task {
	if len(m.cols[m.col]) == 0 {
		return nil
	}
	return &m.cols[m.col][m.sel]
}

// Col reports the selected column index (0=pending, 1=in_progress, 2=done).
func (m Model) Col() int { return m.col }

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case msgs.Refresh:
		if err := m.Reload(); err != nil {
			return msgs.Err(err)
		}
		return nil
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.sel < len(m.cols[m.col])-1 {
				m.sel++
			}
		case "k", "up":
			if m.sel > 0 {
				m.sel--
			}
		case "h", "left":
			if m.col > 0 {
				m.col--
				m.clamp()
			}
		case "l", "right":
			if m.col < 2 {
				m.col++
				m.clamp()
			}
		}
	}
	return nil
}

// View renders the three columns side by side. The root model places it
// directly in the body (no extra border).
func (m Model) View() string {
	colOuter := m.width / 3
	boxes := make([]string, 3)
	for i := range m.cols {
		boxes[i] = m.renderColumn(i, colOuter)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, boxes...)
}

func (m Model) renderColumn(i, outerW int) string {
	innerW := outerW - 2 // border
	if innerW < 4 {
		innerW = 4
	}
	st := styles.Panel
	titleStyle := styles.Label
	if i == m.col && m.Focused {
		st = styles.PanelFocused
		titleStyle = styles.Cyan
	}

	var b strings.Builder
	title := fmt.Sprintf("%s (%d)", colTitles[i], len(m.cols[i]))
	b.WriteString(titleStyle.Bold(true).Render(title) + "\n")

	if len(m.cols[i]) == 0 {
		b.WriteString(styles.Label.Render("—"))
	} else {
		today := time.Now().Format("2006-01-02")
		rows := (m.height - 1) / 2 // one title line, two lines per card
		if rows < 1 {
			rows = 1
		}
		start := 0
		selected := i == m.col
		if selected && m.sel >= rows {
			start = m.sel - rows + 1
		}
		end := start + rows
		if end > len(m.cols[i]) {
			end = len(m.cols[i])
		}
		for j := start; j < end; j++ {
			b.WriteString(m.renderCard(m.cols[i][j], innerW, today, selected && j == m.sel))
		}
	}
	return st.Width(innerW).Height(m.height - 2).Render(strings.TrimRight(b.String(), "\n"))
}

func (m Model) renderCard(t store.Task, innerW int, today string, sel bool) string {
	marker := "  "
	title := t.Title
	if sel {
		marker = styles.Cyan.Render(styles.Marker)
		title = styles.Title.Render(truncate(t.Title, innerW-len(styles.Marker)-1))
	} else {
		title = truncate(t.Title, innerW-3)
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
		meta = append(meta, styles.Label.Render(fmt.Sprintf("[%dn]", t.NoteCount)))
	}
	if t.DueDate != "" {
		if t.Overdue(today) {
			meta = append(meta, styles.Red.Render(t.DueDate+"!"))
		} else {
			meta = append(meta, styles.Light.Render(t.DueDate))
		}
	}
	return line1 + "\n  " + strings.Join(meta, " ") + "\n"
}

// truncate shortens s to at most max display cells (rune-approximate),
// appending an ellipsis when it cuts.
func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}
