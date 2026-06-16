// Package notification is the notification configuration modal for a task.
package notification

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/store"
	"tskr/internal/ui/keys"
	"tskr/internal/ui/msgs"
	"tskr/internal/ui/styles"
)

type fieldKind int

const (
	fText         fieldKind = iota
	fSelect
	fMultiSelect
	fBool
)

type notifField struct {
	Label   string
	kind    fieldKind
	Input   textinput.Model
	options []string
	values  []string
	selIdx  int
	selMap  map[int]bool // for fMultiSelect
	checked bool         // for fBool
}

func (f *notifField) visible(modeIdx int) bool {
	switch f.Label {
	case "Due":
		return modeIdx != 2
	case "Minutes":
		return modeIdx == 2
	default:
		return true
	}
}

func (f *notifField) fieldValue() string {
	if f.kind == fMultiSelect {
		var selected []string
		for i, v := range f.values {
			if f.selMap[i] {
				selected = append(selected, v)
			}
		}
		return strings.Join(selected, ",")
	}
	if f.kind == fSelect {
		if f.values != nil {
			return f.values[f.selIdx]
		}
		return f.options[f.selIdx]
	}
	return f.Input.Value()
}

type Model struct {
	task      store.Task
	notif     *store.Notification
	fields    []notifField
	focus     int
	fieldList []int
	onSubmit  func(title, body, urgency, mode, dueDate string, intervalMin int, triggerStatus string, active bool) tea.Msg
	onDelete  func(id int64) tea.Msg
}

func New(task store.Task, notif *store.Notification,
	submit func(title, body, urgency, mode, dueDate string, intervalMin int, triggerStatus string, active bool) tea.Msg,
	onDelete func(id int64) tea.Msg) Model {

	m := Model{task: task, notif: notif, onSubmit: submit, onDelete: onDelete}

	urgencyOpts := []string{"normal", "critical"}
	modeOpts := []string{"Once", "Recurring (daily)", "Every X minutes"}
	modeVals := []string{store.NotifOnce, store.NotifRecurring, store.NotifInterval}
	statusOpts := []string{"Pending", "In Progress", "Done"}
	statusVals := []string{"pending", "in_progress", "done"}

	urgencyIdx, modeIdx := 0, 0
	dueVal, titleVal, bodyVal, intervalVal := "", "", "", ""
	active := true
	statusSel := map[int]bool{0: true}

	if notif != nil {
		titleVal = notif.Title
		bodyVal = notif.Body
		dueVal = notif.DueDate
		active = notif.Active
		for i, v := range urgencyOpts {
			if v == notif.Urgency {
				urgencyIdx = i
			}
		}
		for i, v := range modeVals {
			if v == notif.Mode {
				modeIdx = i
			}
		}
		statusSel = map[int]bool{}
		for _, p := range strings.Split(notif.TriggerStatus, ",") {
			p = strings.TrimSpace(p)
			for i, v := range statusVals {
				if v == p {
					statusSel[i] = true
				}
			}
		}
		if notif.IntervalMinutes > 0 {
			intervalVal = fmt.Sprintf("%d", notif.IntervalMinutes)
		}
	}

	dueInput := textinput.New()
	dueInput.Width = 30
	dueInput.SetValue(dueVal)
	if modeIdx == 1 {
		dueInput.Placeholder = "HH:MM (daily)"
	} else {
		dueInput.Placeholder = "YYYY-MM-DD HH:MM"
	}

	intervalInput := textinput.New()
	intervalInput.Width = 30
	intervalInput.Placeholder = "minutes"
	intervalInput.SetValue(intervalVal)

	titleInput := textinput.New()
	titleInput.Width = 30
	titleInput.Placeholder = "notification title"
	titleInput.SetValue(titleVal)

	bodyInput := textinput.New()
	bodyInput.Width = 30
	bodyInput.Placeholder = "notification body"
	bodyInput.SetValue(bodyVal)

	m.fields = []notifField{
		{Label: "Active", kind: fBool, checked: active},
		{Label: "Urgency", kind: fSelect, options: urgencyOpts, values: urgencyOpts, selIdx: urgencyIdx},
		{Label: "Mode", kind: fSelect, options: modeOpts, values: modeVals, selIdx: modeIdx},
		{Label: "Due", kind: fText, Input: dueInput},
		{Label: "Minutes", kind: fText, Input: intervalInput},
		{Label: "Title", kind: fText, Input: titleInput},
		{Label: "Body", kind: fText, Input: bodyInput},
		{Label: "Send when status", kind: fMultiSelect, options: statusOpts, values: statusVals, selMap: statusSel},
	}

	m.rebuildFieldList()
	if len(m.fieldList) > 0 {
		fi := m.fieldList[0]
		if m.fields[fi].kind == fText {
			m.fields[fi].Input.Focus()
		}
	}
	return m
}

func (m *Model) rebuildFieldList() {
	modeIdx := m.fields[2].selIdx
	m.fieldList = m.fieldList[:0]
	for i := range m.fields {
		if m.fields[i].visible(modeIdx) {
			m.fieldList = append(m.fieldList, i)
		}
	}
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	// Global keys — work regardless of focused field type.
	switch k.String() {
	case "esc":
		return m, msgs.Cmd(msgs.CloseModal{})
	case "tab", "down":
		m.moveFocus(1)
		return m, nil
	case "shift+tab", "up":
		m.moveFocus(-1)
		return m, nil
	case "enter":
		notif := m.collect()
		return m, tea.Batch(
			msgs.Cmd(m.onSubmit(notif.Title, notif.Body, notif.Urgency, notif.Mode, notif.DueDate, notif.IntervalMinutes, notif.TriggerStatus, notif.Active)),
			msgs.Cmd(msgs.CloseModal{}),
		)
	case "ctrl+t":
		notif := m.collect()
		go func() {
			u := "--urgency=" + notif.Urgency
			t := notif.Title
			if t == "" {
				t = "tskr"
			}
			exec.Command("notify-send", u, t, notif.Body).Run()
		}()
		return m, msgs.Info("test notification sent")
	case "ctrl+d":
		if m.notif != nil {
			return m, msgs.Cmd(m.onDelete(m.notif.ID))
		}
		return m, nil
	}

	// When a text field is focused, delegate every remaining key to it.
	if len(m.fieldList) > 0 {
		fi := m.fieldList[m.focus]
		if m.fields[fi].kind == fText {
			var cmd tea.Cmd
			m.fields[fi].Input, cmd = m.fields[fi].Input.Update(msg)
			return m, cmd
		}
	}

	// Navigation on select / multiselect / bool fields.
	switch k.String() {
	case " ", "space":
		if len(m.fieldList) == 0 {
			return m, nil
		}
		fi := m.fieldList[m.focus]
		switch m.fields[fi].kind {
		case fBool:
			m.fields[fi].checked = !m.fields[fi].checked
		case fMultiSelect:
			m.fields[fi].selMap[m.fields[fi].selIdx] = !m.fields[fi].selMap[m.fields[fi].selIdx]
		}
		return m, nil
	case "left", "h":
		if len(m.fieldList) == 0 {
			return m, nil
		}
		fi := m.fieldList[m.focus]
		if m.fields[fi].kind == fSelect || m.fields[fi].kind == fMultiSelect {
			if m.fields[fi].selIdx > 0 {
				m.fields[fi].selIdx--
			}
			if fi == 2 {
				m.rebuildFieldList()
				m.clampFocus()
				m.updateDuePlaceholder()
			}
		}
		return m, nil
	case "right", "l":
		if len(m.fieldList) == 0 {
			return m, nil
		}
		fi := m.fieldList[m.focus]
		if m.fields[fi].kind == fSelect || m.fields[fi].kind == fMultiSelect {
			if m.fields[fi].selIdx < len(m.fields[fi].options)-1 {
				m.fields[fi].selIdx++
			}
			if fi == 2 {
				m.rebuildFieldList()
				m.clampFocus()
				m.updateDuePlaceholder()
			}
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) moveFocus(delta int) {
	if len(m.fieldList) == 0 {
		return
	}
	oldFi := m.fieldList[m.focus]
	if m.fields[oldFi].kind == fText {
		m.fields[oldFi].Input.Blur()
	}
	m.focus = (m.focus + delta + len(m.fieldList)) % len(m.fieldList)
	newFi := m.fieldList[m.focus]
	if m.fields[newFi].kind == fText {
		m.fields[newFi].Input.Focus()
	}
}

func (m *Model) clampFocus() {
	if m.focus >= len(m.fieldList) {
		m.focus = len(m.fieldList) - 1
	}
	if m.focus < 0 {
		m.focus = 0
	}
}

func (m *Model) updateDuePlaceholder() {
	if m.fields[2].selIdx == 1 {
		m.fields[3].Input.Placeholder = "HH:MM (daily)"
	} else {
		m.fields[3].Input.Placeholder = "YYYY-MM-DD HH:MM"
	}
}

func (m Model) collect() store.Notification {
	n := store.Notification{
		TaskID:  m.task.ID,
		Title:   m.fields[5].Input.Value(),
		Body:    m.fields[6].Input.Value(),
		Active:  m.fields[0].checked,
	}
	n.Urgency = m.fields[1].values[m.fields[1].selIdx]
	n.Mode = m.fields[2].values[m.fields[2].selIdx]
	n.TriggerStatus = m.fields[7].fieldValue()

	if n.Mode == store.NotifInterval {
		fmt.Sscanf(m.fields[4].Input.Value(), "%d", &n.IntervalMinutes)
	} else {
		n.DueDate = m.fields[3].Input.Value()
	}

	if m.notif != nil {
		n.ID = m.notif.ID
	}
	return n
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Notifications — "+m.task.Title) + "\n\n")
	modeIdx := m.fields[2].selIdx

	for i := range m.fields {
		if !m.fields[i].visible(modeIdx) {
			continue
		}
		isFocused := false
		for fi := range m.fieldList {
			if m.fieldList[fi] == i && fi == m.focus {
				isFocused = true
				break
			}
		}
		b.WriteString(styles.FormLabel.Render(m.fields[i].Label) + "\n")
		b.WriteString(m.renderField(i, isFocused) + "\n")
	}

	b.WriteString("\n")
	hints := []keys.Hint{
		{Key: "enter", Desc: "save"},
		{Key: "ctrl+t", Desc: "test"},
		{Key: "tab", Desc: "next"},
		{Key: "space", Desc: "toggle"},
		{Key: "\u2190/\u2192", Desc: "option"},
	}
	if m.notif != nil {
		hints = append(hints, keys.Hint{Key: "ctrl+d", Desc: "delete"})
	}
	hints = append(hints, keys.Hint{Key: "esc", Desc: "cancel"})
	b.WriteString(keys.Render(hints))

	return styles.ModalBox.Render(b.String())
}

func (m Model) renderField(i int, focused bool) string {
	f := m.fields[i]
	switch f.kind {
	case fBool:
		box := "[ ]"
		label := styles.Label.Render("off")
		if f.checked {
			box = styles.Green.Render("[x]")
			label = styles.Green.Render("on")
		}
		marker := "  "
		if focused {
			marker = styles.Cyan.Render("▸")
			box = styles.Cyan.Render("[" + styles.Light.Render("x") + "]")
			if f.checked {
				box = styles.Cyan.Render("[x]")
			}
		}
		if f.checked && focused {
			box = styles.Cyan.Render("[x]")
		} else if f.checked {
			box = styles.Green.Render("[x]")
		}
		return marker + " " + box + " " + label

	case fMultiSelect:
		parts := make([]string, len(f.options))
		for j, opt := range f.options {
			box := "[ ]"
			style := styles.Label
			if f.selMap[j] {
				box, style = "[x]", styles.Green
			}
			marker := "  "
			if j == f.selIdx && focused {
				marker = styles.Cyan.Render("▸")
				style = styles.Cyan
			}
			label := opt
			if j == f.selIdx && focused {
				label = styles.Light.Render(opt)
			} else {
				label = styles.Label.Render(opt)
			}
			parts[j] = marker + style.Render(box) + " " + label
		}
		return strings.Join(parts, "  ")

	case fSelect:
		parts := make([]string, len(f.options))
		for j, opt := range f.options {
			switch {
			case j == f.selIdx && focused:
				parts[j] = styles.Cyan.Render("▸ " + opt)
			case j == f.selIdx:
				parts[j] = styles.Light.Render(opt)
			default:
				parts[j] = styles.Label.Render(opt)
			}
		}
		return strings.Join(parts, "  ")

	default:
		return f.Input.View()
	}
}
