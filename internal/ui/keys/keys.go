// Package keys defines every keybinding hint once; the status bar and
// the help overlay both read from here.
package keys

import (
	"strings"

	"tskr/internal/ui/styles"
)

type Hint struct {
	Key, Desc string
}

var (
	TaskList = []Hint{
		{"j/k", "move"}, {"enter", "details"}, {"1-4", "tab"}, {"a", "add"},
		{"e", "edit"}, {"d", "delete"}, {"s", "status"}, {"/", "search"},
		{"o", "sort"}, {"p", "projects"}, {"?", "help"},
	}
	Detail = []Hint{
		{"j/k", "move"}, {"space", "toggle"}, {"a", "subtask"}, {"n", "note"},
		{"t", "log time"}, {"b", "deps"}, {"e", "edit"}, {"d", "delete"}, {"esc", "back"},
	}
	Search = []Hint{{"enter", "keep filter"}, {"esc", "clear"}}
	Picker = []Hint{
		{"j/k", "move"}, {"enter", "open"}, {"/", "search"}, {"n", "new"},
		{"e", "edit"}, {"d", "delete"}, {"s", "archive"}, {"A", "show archived"},
	}
	Form    = []Hint{{"tab", "next field"}, {"enter", "save"}, {"esc", "cancel"}}
	Confirm = []Hint{{"y", "confirm"}, {"n/esc", "cancel"}}
	Deps    = []Hint{{"j/k", "move"}, {"space", "toggle blocks"}, {"/", "search"}, {"esc", "done"}}
	HelpBar = []Hint{{"any key", "close"}}
)

type Group struct {
	Title string
	Hints []Hint
}

func HelpGroups() []Group {
	return []Group{
		{"Global", []Hint{
			{"p", "project picker"}, {"<  >", "resize split"}, {"?", "help"},
			{"q", "quit"}, {"esc", "back / close"},
		}},
		{"Task list", TaskList},
		{"Task list (extra)", []Hint{
			{"tab / shift+tab", "next / previous tab"}, {"S", "status menu"},
		}},
		{"Details", append(append([]Hint{}, Detail...), Hint{"J/K", "reorder subtask"})},
		{"Project picker", Picker},
		{"Forms", Form},
		{"Dependencies", Deps},
	}
}

// Render draws hints for the status bar: cyan key, light description.
func Render(hints []Hint) string {
	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = styles.Cyan.Render(h.Key) + " " + styles.Light.Render(h.Desc)
	}
	return strings.Join(parts, styles.Label.Render(" · "))
}
