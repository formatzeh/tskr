// Package picker is the project picker modal with a fuzzy search bar.
package picker

import (
	"fmt"
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
	st           *store.Store
	search       textinput.Model
	searching    bool
	showArchived bool
	allowClose   bool // false for the startup picker (no project open yet)
	projects     []store.Project
	filtered     []store.Project
	sel          int
}

func New(st *store.Store, allowClose bool) Model {
	ti := textinput.New()
	ti.Placeholder = "fuzzy search…"
	ti.Width = 30
	m := Model{st: st, search: ti, allowClose: allowClose}
	m.reload()
	return m
}

func (m *Model) reload() {
	m.projects, _ = m.st.ListProjects(m.showArchived)
	m.applyFilter()
}

func (m *Model) applyFilter() {
	q := m.search.Value()
	m.filtered = m.filtered[:0]
	for _, p := range m.projects {
		if fuzzy.Match(q, p.Name+" "+p.Tags) {
			m.filtered = append(m.filtered, p)
		}
	}
	if m.sel >= len(m.filtered) {
		m.sel = len(m.filtered) - 1
	}
	if m.sel < 0 {
		m.sel = 0
	}
}

func (m Model) selected() *store.Project {
	if len(m.filtered) == 0 {
		return nil
	}
	return &m.filtered[m.sel]
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case msgs.Refresh:
		m.reload()
		return m, nil
	case tea.KeyMsg:
		if m.searching {
			switch msg.String() {
			case "enter":
				m.searching = false
				m.search.Blur()
				return m, nil
			case "esc":
				m.searching = false
				m.search.Blur()
				m.search.SetValue("")
				m.applyFilter()
				return m, nil
			default:
				var cmd tea.Cmd
				m.search, cmd = m.search.Update(msg)
				m.applyFilter()
				return m, cmd
			}
		}
		switch msg.String() {
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
		case "n":
			return m, msgs.Cmd(msgs.NewProjectForm{})
		case "e":
			if p := m.selected(); p != nil {
				return m, msgs.Cmd(msgs.EditProjectForm{Project: *p})
			}
		case "d":
			if p := m.selected(); p != nil {
				return m, msgs.Cmd(msgs.DeleteProject{Project: *p})
			}
		case "s":
			if p := m.selected(); p != nil {
				next := store.ProjectArchived
				if p.Status == store.ProjectArchived {
					next = store.ProjectActive
				}
				if err := m.st.SetProjectStatus(p.ID, next); err != nil {
					return m, msgs.Err(err)
				}
				m.reload()
				return m, msgs.Info(fmt.Sprintf("%s → %s", p.Name, next))
			}
		case "A":
			m.showArchived = !m.showArchived
			m.reload()
		case "enter":
			if p := m.selected(); p != nil {
				return m, msgs.Cmd(msgs.OpenProject{ID: p.ID})
			}
		case "esc", "q":
			if m.allowClose {
				return m, msgs.Cmd(msgs.CloseModal{})
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Select Project") + "\n\n")
	b.WriteString(styles.Cyan.Render("/ ") + m.search.View() + "\n\n")
	if len(m.filtered) == 0 {
		b.WriteString(styles.Label.Render("no projects — press n to create one") + "\n")
	}
	for i, p := range m.filtered {
		marker := "  "
		name := p.Name
		if i == m.sel {
			marker = styles.Cyan.Render(styles.Marker)
			name = styles.Title.Render(name)
		}
		count := styles.Label.Render(fmt.Sprintf("  %d tasks", p.TaskCount))
		archived := ""
		if p.Status == store.ProjectArchived {
			archived = styles.Label.Render(" (archived)")
		}
		b.WriteString(marker + name + count + archived + "\n")
	}
	b.WriteString("\n" + keys.Render(keys.Picker))
	return styles.ModalBox.Render(b.String())
}
