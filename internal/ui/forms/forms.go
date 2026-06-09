// Package forms provides validated modal input forms.
package forms

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/ui/keys"
	"tskr/internal/ui/msgs"
	"tskr/internal/ui/styles"
)

type Field struct {
	Label    string
	Input    textinput.Model
	Validate func(string) string // "" = valid, otherwise the error text
}

func NewField(label, value, placeholder string, validate func(string) string) Field {
	ti := textinput.New()
	ti.SetValue(value)
	ti.Placeholder = placeholder
	ti.Width = 40
	return Field{Label: label, Input: ti, Validate: validate}
}

type Model struct {
	Title    string
	fields   []Field
	errs     []string
	focus    int
	OnSubmit func(values []string) tea.Msg
}

func New(title string, fields []Field, onSubmit func([]string) tea.Msg) Model {
	m := Model{Title: title, fields: fields, errs: make([]string, len(fields)), OnSubmit: onSubmit}
	m.fields[0].Input.Focus()
	return m
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			return m, msgs.Cmd(msgs.CloseModal{})
		case "tab", "down":
			m.setFocus((m.focus + 1) % len(m.fields))
			return m, nil
		case "shift+tab", "up":
			m.setFocus((m.focus - 1 + len(m.fields)) % len(m.fields))
			return m, nil
		case "enter":
			valid := true
			for i := range m.fields {
				if m.fields[i].Validate != nil {
					m.errs[i] = m.fields[i].Validate(m.fields[i].Input.Value())
					if m.errs[i] != "" {
						valid = false
					}
				}
			}
			if !valid {
				return m, nil
			}
			values := make([]string, len(m.fields))
			for i := range m.fields {
				values[i] = strings.TrimSpace(m.fields[i].Input.Value())
			}
			return m, tea.Batch(msgs.Cmd(m.OnSubmit(values)), msgs.Cmd(msgs.CloseModal{}))
		}
	}
	var cmd tea.Cmd
	m.fields[m.focus].Input, cmd = m.fields[m.focus].Input.Update(msg)
	return m, cmd
}

func (m *Model) setFocus(i int) {
	m.fields[m.focus].Input.Blur()
	m.focus = i
	m.fields[m.focus].Input.Focus()
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render(m.Title) + "\n\n")
	for i := range m.fields {
		b.WriteString(styles.Label.Render(m.fields[i].Label) + "\n")
		b.WriteString(m.fields[i].Input.View() + "\n")
		if m.errs[i] != "" {
			b.WriteString(styles.Err.Render("✗ "+m.errs[i]) + "\n")
		}
	}
	b.WriteString("\n" + keys.Render(keys.Form))
	return styles.ModalBox.Render(b.String())
}
