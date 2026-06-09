// Package detail is the right panel: a scrollable viewport over the
// selected task with an inner cursor across subtasks, notes, and time
// entries. Toggle/reorder run directly against the store; modal-opening
// actions are handled by the root model via CurrentItem().
package detail

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/store"
	"tskr/internal/ui/msgs"
	"tskr/internal/ui/styles"
	"tskr/internal/ui/timefmt"
)

type ItemKind int

const (
	ItemSubtask ItemKind = iota
	ItemNote
	ItemEntry
)

// Item is an actionable row under the cursor.
type Item struct {
	Kind ItemKind
	ID   int64
	Idx  int // index within its own list
	line int // line number in the rendered content
}

type Model struct {
	st       *store.Store
	Task     *store.Task
	subtasks []store.Subtask
	notes    []store.Note
	entries  []store.TimeEntry
	items    []Item
	cursor   int
	vp       viewport.Model
	Focused  bool
	width    int
	height   int
}

func New(st *store.Store) Model {
	return Model{st: st, vp: viewport.New(0, 0)}
}

func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	m.vp.Width = w
	m.vp.Height = h
	m.rebuild()
}

func (m *Model) Clear() {
	m.Task = nil
	m.subtasks, m.notes, m.entries, m.items = nil, nil, nil, nil
	m.cursor = 0
	m.rebuild()
}

func (m *Model) SetTask(id int64) error {
	t, err := m.st.GetTask(id)
	if err != nil {
		return err
	}
	m.Task = &t
	if m.subtasks, err = m.st.ListSubtasks(id); err != nil {
		return err
	}
	if m.notes, err = m.st.ListNotes(id); err != nil {
		return err
	}
	if m.entries, err = m.st.ListTimeEntries(id); err != nil {
		return err
	}
	m.rebuild()
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	return nil
}

func (m *Model) Reload() error {
	if m.Task == nil {
		return nil
	}
	return m.SetTask(m.Task.ID)
}

func (m Model) CurrentItem() (Item, bool) {
	if len(m.items) == 0 {
		return Item{}, false
	}
	return m.items[m.cursor], true
}

func (m Model) Subtask(i int) store.Subtask { return m.subtasks[i] }
func (m Model) Note(i int) store.Note       { return m.notes[i] }
func (m Model) Entry(i int) store.TimeEntry { return m.entries[i] }

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
			if m.cursor < len(m.items)-1 {
				m.cursor++
				m.rebuild()
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.rebuild()
			}
		case " ", "space":
			if it, ok := m.CurrentItem(); ok && it.Kind == ItemSubtask {
				if err := m.st.ToggleSubtask(it.ID); err != nil {
					return msgs.Err(err)
				}
				return msgs.Cmd(msgs.Refresh{})
			}
		case "J", "K":
			if it, ok := m.CurrentItem(); ok && it.Kind == ItemSubtask {
				up := msg.String() == "K"
				if err := m.st.MoveSubtask(it.ID, up); err != nil {
					return msgs.Err(err)
				}
				if up && it.Idx > 0 {
					m.cursor--
				}
				if !up && it.Idx < len(m.subtasks)-1 {
					m.cursor++
				}
				return msgs.Cmd(msgs.Refresh{})
			}
		default:
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return cmd
		}
	}
	return nil
}

func (m *Model) rebuild() {
	if m.Task == nil {
		m.vp.SetContent(styles.Label.Render("no task selected"))
		m.items = m.items[:0]
		return
	}
	t := m.Task
	today := time.Now().Format("2006-01-02")
	m.items = m.items[:0]
	var lines []string
	label := func(s string) string { return styles.Label.Render(fmt.Sprintf("%-11s", s)) }

	lines = append(lines, label("Title")+styles.Title.Render(t.Title), "")
	lines = append(lines, label("Status")+styles.StatusLabel(t.Status))
	lines = append(lines, label("Priority")+styles.PriorityName(t.Priority))
	lines = append(lines, label("Created")+fmtDate(t.CreatedAt))
	due := styles.Label.Render("—")
	if t.DueDate != "" {
		if t.Overdue(today) {
			due = styles.Red.Render(t.DueDate + " (overdue!)")
		} else {
			due = t.DueDate
		}
	}
	lines = append(lines, label("Due")+due)
	tags := styles.Label.Render("—")
	if t.Tags != "" {
		tags = styles.Tag.Render(strings.ReplaceAll(t.Tags, ",", ", "))
	}
	lines = append(lines, label("Tags")+tags)
	logged := styles.Label.Render("—")
	if t.Minutes > 0 {
		logged = styles.Green.Render(timefmt.FormatMinutes(t.Minutes) + " logged")
	}
	lines = append(lines, label("Time")+logged)
	for i, r := range t.BlockedBy {
		l := label("")
		if i == 0 {
			l = label("Blocked by")
		}
		lines = append(lines, l+styles.Red.Render("⛔ "+r.Title)+styles.Label.Render(" ("+string(r.Status)+")"))
	}
	for i, r := range t.Blocks {
		l := label("")
		if i == 0 {
			l = label("Blocks")
		}
		lines = append(lines, l+styles.Orange.Render(r.Title)+styles.Label.Render(" ("+string(r.Status)+")"))
	}

	if t.Description != "" {
		lines = append(lines, "", styles.Label.Render("Description"))
		lines = append(lines, strings.Split(t.Description, "\n")...)
	}

	lines = append(lines, "", styles.Label.Render("Subtasks")+"   "+m.subtaskCount())
	if len(m.subtasks) > 0 {
		lines = append(lines, progressBar(m.doneCount(), len(m.subtasks), 24))
		for i, st := range m.subtasks {
			m.items = append(m.items, Item{Kind: ItemSubtask, ID: st.ID, Idx: i, line: len(lines)})
			box, title := "[ ] ", st.Title
			if st.Done {
				box = "[x] "
				title = styles.Green.Render(title)
			}
			lines = append(lines, m.cursorMark(len(m.items)-1)+box+title)
		}
	}

	lines = append(lines, "", styles.Label.Render("Notes")+"      "+fmt.Sprintf("%d", len(m.notes)))
	for i, n := range m.notes {
		m.items = append(m.items, Item{Kind: ItemNote, ID: n.ID, Idx: i, line: len(lines)})
		lines = append(lines, m.cursorMark(len(m.items)-1)+styles.Label.Render(fmtDateTime(n.CreatedAt))+" "+styles.Blue.Render(n.Body))
	}

	if len(m.entries) > 0 {
		lines = append(lines, "", styles.Label.Render("Time log"))
		for i, e := range m.entries {
			m.items = append(m.items, Item{Kind: ItemEntry, ID: e.ID, Idx: i, line: len(lines)})
			row := m.cursorMark(len(m.items)-1) + styles.Label.Render(fmtDateTime(e.CreatedAt)) + " " + styles.Green.Render(timefmt.FormatMinutes(e.Minutes))
			if e.Note != "" {
				row += " " + e.Note
			}
			lines = append(lines, row)
		}
	}

	m.vp.SetContent(strings.Join(lines, "\n"))
	m.scrollToCursor()
}

func (m *Model) scrollToCursor() {
	if len(m.items) == 0 || m.vp.Height <= 0 {
		return
	}
	line := m.items[m.cursor].line
	if line < m.vp.YOffset {
		m.vp.SetYOffset(line)
	} else if line >= m.vp.YOffset+m.vp.Height {
		m.vp.SetYOffset(line - m.vp.Height + 1)
	}
}

func (m Model) cursorMark(itemIdx int) string {
	if m.Focused && itemIdx == m.cursor {
		return styles.Cyan.Render("▸ ")
	}
	return "  "
}

func (m Model) doneCount() int {
	n := 0
	for _, st := range m.subtasks {
		if st.Done {
			n++
		}
	}
	return n
}

func (m Model) subtaskCount() string {
	if len(m.subtasks) == 0 {
		return styles.Label.Render("—")
	}
	return styles.Blue.Render(fmt.Sprintf("%d/%d", m.doneCount(), len(m.subtasks)))
}

func progressBar(done, total, width int) string {
	if total == 0 {
		return ""
	}
	filled := done * width / total
	return styles.Blue.Render(strings.Repeat("█", filled)) +
		styles.Label.Render(strings.Repeat("░", width-filled))
}

// fmtDate renders an RFC3339 timestamp as "May 12, 2026".
func fmtDate(rfc string) string {
	ts, err := time.Parse(time.RFC3339, rfc)
	if err != nil {
		return rfc
	}
	return ts.Local().Format("Jan 2, 2006")
}

// fmtDateTime renders an RFC3339 timestamp as "May 12 17:02".
func fmtDateTime(rfc string) string {
	ts, err := time.Parse(time.RFC3339, rfc)
	if err != nil {
		return rfc
	}
	return ts.Local().Format("Jan 2 15:04")
}

func (m Model) View() string { return m.vp.View() }
