// Package depsel is the dependency management modal: a fuzzy-searchable
// list of same-project tasks where space toggles "this task blocks X".
package depsel

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/store"
	"tskr/internal/ui/fuzzy"
	"tskr/internal/ui/keys"
	"tskr/internal/ui/msgs"
	"tskr/internal/ui/styles"
)

type Model struct {
	st         *store.Store
	task       store.Task
	candidates []store.Task
	blocks     map[int64]bool // task.ID blocks candidate
	blockedBy  map[int64]bool // candidate blocks task.ID (informational)
	search     textinput.Model
	searching  bool
	filtered   []store.Task
	sel        int
}

func New(st *store.Store, task store.Task) Model {
	ti := textinput.New()
	ti.Placeholder = "fuzzy search…"
	ti.Width = 30
	m := Model{st: st, task: task, search: ti}
	m.reload()
	return m
}

func (m *Model) reload() {
	all, _ := m.st.ListTasks(m.task.ProjectID, "", store.SortCreated)
	m.candidates = m.candidates[:0]
	for _, t := range all {
		if t.ID != m.task.ID {
			m.candidates = append(m.candidates, t)
		}
	}
	full, err := m.st.GetTask(m.task.ID)
	m.blocks = map[int64]bool{}
	m.blockedBy = map[int64]bool{}
	if err == nil {
		for _, r := range full.Blocks {
			m.blocks[r.ID] = true
		}
		for _, r := range full.BlockedBy {
			m.blockedBy[r.ID] = true
		}
	}
	m.applyFilter()
}

func (m *Model) applyFilter() {
	q := m.search.Value()
	m.filtered = m.filtered[:0]
	for _, t := range m.candidates {
		if fuzzy.Match(q, t.Title+" "+t.Tags) {
			m.filtered = append(m.filtered, t)
		}
	}
	if m.sel >= len(m.filtered) {
		m.sel = len(m.filtered) - 1
	}
	if m.sel < 0 {
		m.sel = 0
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.searching {
		switch k.String() {
		case "enter", "esc":
			m.searching = false
			m.search.Blur()
			if k.String() == "esc" {
				m.search.SetValue("")
				m.applyFilter()
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.applyFilter()
			return m, cmd
		}
	}
	switch k.String() {
	case "j", "down":
		if m.sel < len(m.filtered)-1 {
			m.sel++
		}
	case "k", "up":
		if m.sel > 0 {
			m.sel--
		}
	case "/":
		m.searching = true
		return m, m.search.Focus()
	case " ", "space":
		if len(m.filtered) == 0 {
			return m, nil
		}
		target := m.filtered[m.sel]
		var err error
		if m.blocks[target.ID] {
			err = m.st.RemoveDependency(m.task.ID, target.ID)
		} else {
			err = m.st.AddDependency(m.task.ID, target.ID)
		}
		if err != nil {
			return m, msgs.Err(err)
		}
		m.reload()
	case "esc", "q":
		return m, tea.Batch(msgs.Cmd(msgs.Refresh{}), msgs.Cmd(msgs.CloseModal{}))
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Dependencies — "+m.task.Title) + "\n")
	b.WriteString(styles.Label.Render("space marks tasks that this task blocks") + "\n\n")
	b.WriteString(styles.Cyan.Render("/ ") + m.search.View() + "\n\n")
	if len(m.filtered) == 0 {
		b.WriteString(styles.Label.Render("no other tasks in this project") + "\n")
	}
	for i, t := range m.filtered {
		marker := "  "
		title := t.Title
		if i == m.sel {
			marker = styles.Cyan.Render("▸ ")
			title = styles.Title.Render(title)
		}
		box := styles.Label.Render("[ ] ")
		if m.blocks[t.ID] {
			box = styles.Orange.Render("[b] ")
		}
		row := marker + box + title
		if m.blockedBy[t.ID] {
			row += " " + styles.Red.Render("⛔ blocks this task")
		}
		b.WriteString(row + "\n")
	}
	b.WriteString("\n" + keys.Render(keys.Deps))
	return styles.ModalBox.Render(b.String())
}
