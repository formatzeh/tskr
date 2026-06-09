// Package msgs holds messages exchanged between UI components and the
// root model.
package msgs

import (
	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/store"
)

// Refresh tells every component to reload from the store.
type Refresh struct{}

// CloseModal pops the top modal.
type CloseModal struct{}

// OpenProject switches the main view to the given project.
type OpenProject struct{ ID int64 }

// NewProjectForm / EditProjectForm / DeleteProject ask the root model to
// open the corresponding modal (emitted by the picker).
type NewProjectForm struct{}
type EditProjectForm struct{ Project store.Project }
type DeleteProject struct{ Project store.Project }

// Status shows a transient message in the status bar.
type Status struct {
	Text  string
	Error bool
}

// Cmd wraps a message in a tea.Cmd.
func Cmd(m tea.Msg) tea.Cmd { return func() tea.Msg { return m } }

func Err(err error) tea.Cmd { return Cmd(Status{Text: err.Error(), Error: true}) }

func Info(text string) tea.Cmd { return Cmd(Status{Text: text}) }
