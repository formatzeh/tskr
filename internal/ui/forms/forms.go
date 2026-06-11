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

type fieldKind int

const (
	fieldText   fieldKind = iota
	fieldSelect           // inline option selector; ←/→ to move
)

// Field is either a text input (fieldText) or an inline option selector
// (fieldSelect). Only the relevant set of fields is used per kind.
type Field struct {
	Label    string
	kind     fieldKind
	// fieldText
	Input    textinput.Model
	Validate func(string) string
	// fieldSelect — options are display labels; values are submitted strings
	// (nil values means use options directly as values)
	options []string
	values  []string
	selIdx  int
}

func NewField(label, value, placeholder string, validate func(string) string) Field {
	ti := textinput.New()
	ti.SetValue(value)
	ti.Placeholder = placeholder
	ti.Width = 40
	return Field{Label: label, kind: fieldText, Input: ti, Validate: validate}
}

// NewSelectField creates an inline option selector. options are displayed;
// values are the corresponding strings passed to the submit callback (pass
// nil to use options as values). initial is the pre-selected index.
func NewSelectField(label string, options, values []string, initial int) Field {
	if initial < 0 || initial >= len(options) {
		initial = 0
	}
	return Field{Label: label, kind: fieldSelect, options: options, values: values, selIdx: initial}
}

func (f *Field) fieldValue() string {
	if f.kind == fieldSelect {
		if f.values != nil {
			return f.values[f.selIdx]
		}
		return f.options[f.selIdx]
	}
	return strings.TrimSpace(f.Input.Value())
}

func (f *Field) renderInput(focused bool) string {
	if f.kind != fieldSelect {
		return f.Input.View()
	}
	parts := make([]string, len(f.options))
	for i, opt := range f.options {
		switch {
		case i == f.selIdx && focused:
			parts[i] = styles.Cyan.Render("▸ " + opt)
		case i == f.selIdx:
			parts[i] = styles.Light.Render(opt)
		default:
			parts[i] = styles.Label.Render(opt)
		}
	}
	return strings.Join(parts, "  ")
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
	if fields[0].kind == fieldText {
		m.fields[0].Input.Focus()
	}
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
		case "left", "h":
			if m.fields[m.focus].kind == fieldSelect {
				if m.fields[m.focus].selIdx > 0 {
					m.fields[m.focus].selIdx--
				}
				return m, nil
			}
		case "right", "l":
			if m.fields[m.focus].kind == fieldSelect {
				if m.fields[m.focus].selIdx < len(m.fields[m.focus].options)-1 {
					m.fields[m.focus].selIdx++
				}
				return m, nil
			}
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
				values[i] = m.fields[i].fieldValue()
			}
			return m, tea.Batch(msgs.Cmd(m.OnSubmit(values)), msgs.Cmd(msgs.CloseModal{}))
		}
	}
	if m.fields[m.focus].kind == fieldText {
		var cmd tea.Cmd
		m.fields[m.focus].Input, cmd = m.fields[m.focus].Input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) setFocus(i int) {
	if m.fields[m.focus].kind == fieldText {
		m.fields[m.focus].Input.Blur()
	}
	m.focus = i
	if m.fields[m.focus].kind == fieldText {
		m.fields[m.focus].Input.Focus()
	}
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render(m.Title) + "\n\n")
	for i := range m.fields {
		b.WriteString(styles.FormLabel.Render(m.fields[i].Label) + "\n")
		b.WriteString(m.fields[i].renderInput(i == m.focus) + "\n")
		if m.errs[i] != "" {
			b.WriteString(styles.Err.Render("✗ "+m.errs[i]) + "\n")
		}
	}
	b.WriteString("\n" + keys.Render(keys.Form))
	return styles.ModalBox.Render(b.String())
}
