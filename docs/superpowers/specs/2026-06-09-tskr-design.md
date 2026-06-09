# tskr — TUI Task Manager: Design

**Date:** 2026-06-09
**Status:** Approved design, pending implementation plan

## Overview

`tskr` is a single-user, local-first task manager with a terminal UI, written in
Go with the Bubble Tea framework. Data lives in one SQLite database. The top
level of organization is **projects**; projects contain **tasks**; tasks
contain **subtasks** (simple checklist items), **notes**, **time entries**, and
**dependencies** on other tasks in the same project.

The app is **TUI-only**: running `tskr` opens the interface; there are no
scriptable subcommands. Everything (including dependencies) is managed through
forms and hotkeys.

## Decisions made during brainstorming

| Topic | Decision |
|---|---|
| Interface surface | TUI-only, no CLI subcommands |
| Project lifecycle | `active` / `archived` (archived hidden from default picker, kept for reference) |
| Subtasks | Simple checklist items: title + done flag only. No priority, due date, tags, notes, or dependencies |
| Dependencies | Within a single project only; cycles rejected |
| Blocking semantics | Visual indicator only (⛔ + blocker list); status changes are never prevented |
| Startup | Project picker modal first (Option A); config setting switches to "resume last project" (Option B) |
| Project picker | Modal with fuzzy search bar |
| Architecture | Layered: repository over `database/sql` + composed Bubble Tea child models |
| SQLite driver | `modernc.org/sqlite` (pure Go, no CGo) |

## Tech stack

- Go 1.24+
- `github.com/charmbracelet/bubbletea` (+ `bubbles`, `lipgloss`)
- `modernc.org/sqlite` via `database/sql`
- `github.com/BurntSushi/toml` (config)
- No other runtime dependencies; fuzzy matching is implemented in-repo as a
  simple subsequence matcher

## Project layout

```
tskr/
├── cmd/tskr/main.go          # entry point: load config, open store, run TUI
├── internal/
│   ├── config/config.go      # TOML config load/save
│   ├── store/                # storage layer — no UI imports
│   │   ├── db.go             # open, migrate (embedded schema), backup
│   │   ├── models.go         # Project, Task, Subtask, Note, TimeEntry, Dependency
│   │   ├── projects.go       # project CRUD + archive
│   │   ├── tasks.go          # task CRUD, filters, sorts, delete guard
│   │   ├── subtasks.go       # subtask CRUD + toggle + reorder
│   │   ├── notes.go          # note CRUD
│   │   ├── timelog.go        # time entries
│   │   └── deps.go           # dependency add/remove, blocker queries, cycle check
│   └── ui/
│       ├── app.go            # root model: routing, focus, layout, modal stack
│       ├── keys.go           # all keybindings (help overlay reads this)
│       ├── styles.go         # lipgloss theme — single source for all colors
│       ├── fuzzy/            # subsequence matcher (pure functions)
│       ├── tasklist/         # left panel: tabs, list, fuzzy filter, sort
│       ├── detail/           # right panel: scrollable task detail viewport
│       ├── picker/           # project picker modal with fuzzy search bar
│       ├── forms/            # task/project/subtask/note/timelog forms
│       ├── confirm/          # confirmation dialog (incl. delete-guard variant)
│       └── help/             # full-screen help overlay
└── docs/superpowers/specs/
```

The store layer is plain Go + SQL, fully unit-testable without the TUI. The UI
layer only talks to the store through repository methods.

## Data paths (XDG, overridable via config)

- Database: `~/.local/share/tskr/tskr.db`
- Backups: `~/.local/share/tskr/backups/tskr-YYYY-MM-DD.db` (keep newest 7)
- Config: `~/.config/tskr/config.toml`

`XDG_DATA_HOME` / `XDG_CONFIG_HOME` are respected when set.

## Data model (SQLite)

All timestamps are UTC ISO-8601 strings. IDs are
`INTEGER PRIMARY KEY AUTOINCREMENT`. `PRAGMA foreign_keys = ON`.

```sql
projects     id, name TEXT NOT NULL UNIQUE, description TEXT,
             tags TEXT,                          -- comma-separated, normalized
             status TEXT CHECK(status IN ('active','archived')) DEFAULT 'active',
             created_at, updated_at

tasks        id, project_id REFERENCES projects ON DELETE CASCADE,
             title TEXT NOT NULL, description TEXT,
             status TEXT CHECK(status IN ('pending','in_progress','done'))
                    DEFAULT 'pending',
             priority TEXT CHECK(priority IN ('low','medium','high','urgent')),
                                                 -- NULL = no priority (default)
             due_date TEXT,                      -- NULL = no due date (default)
             tags TEXT, created_at, updated_at, completed_at

subtasks     id, task_id REFERENCES tasks ON DELETE CASCADE,
             title TEXT NOT NULL, done INTEGER DEFAULT 0,
             position INTEGER NOT NULL, created_at

task_deps    blocker_id REFERENCES tasks ON DELETE CASCADE,
             blocked_id REFERENCES tasks ON DELETE CASCADE,
             PRIMARY KEY (blocker_id, blocked_id),
             CHECK (blocker_id != blocked_id)
             -- same-project constraint enforced in store layer

notes        id, task_id REFERENCES tasks ON DELETE CASCADE,
             body TEXT NOT NULL, created_at

time_entries id, task_id REFERENCES tasks ON DELETE CASCADE,
             minutes INTEGER NOT NULL CHECK (minutes > 0),
             note TEXT, created_at

meta         key TEXT PRIMARY KEY, value TEXT
             -- schema_version, last_project_id, last_backup_date
```

### Data model rules

- **Tags** are stored comma-separated and normalized on save: trimmed,
  lowercased, deduplicated. Filtering uses substring matching — adequate for
  single-user local data, keeps editing trivial.
- **Overdue** = `due_date < today AND status != 'done'`.
- **Priority sort order**: urgent > high > medium > low > NULL (NULL last).
  Due-date sort puts NULL last.
- **Dependency cycles** are rejected at insert: the store walks the existing
  graph (DFS) before adding an edge and returns an error if the new edge would
  create a cycle.
- **Delete guard**: `DeleteTask` fails with a typed error listing blocked tasks
  if the task is a blocker for others. `DeleteTaskCascade` removes the
  dependency links (not the dependent tasks) and deletes.
- **completed_at** is set when status changes to `done`, cleared when it
  changes away from `done`.
- Schema migrations: embedded SQL files applied in order, tracked by
  `schema_version` in `meta`.

## UI architecture

### Root model

Owns: current project, focused panel (1: Tasks, 2: Details), split ratio, and a
**modal stack**. Modals (picker, forms, confirm dialogs, help) render centered
over a dimmed background; only the top modal receives key events. `Esc` closes
the top modal. When the stack is empty, keys route to the focused panel.

### Screens & flow

1. **Startup**: project picker modal over an empty background (default), or
   straight into the last-used project when `startup = "last-project"` is set
   in config (falls back to picker on first run or when the last project is
   gone).
2. **Main view**: tab bar (top), task list (left), detail panel (right),
   context-sensitive status bar (bottom).
3. **Project picker** (`p` anytime): fuzzy search bar + project list showing
   name and task count. `n` new project, `e` edit, `d` delete (confirm),
   `s` archive/unarchive, toggle to show archived, `enter` open.

### Main view details

- **Tab bar**: `Pending`, `In Progress`, `Done`, `All` — switched with `1`–`4`
  or `tab`/`shift+tab`. Project name displayed on the right (magenta).
- **Task list rows** (two lines): priority badge (color-coded, `———` when
  none) + title + ⛔ if blocked; dim second line with `[done/total]` subtasks
  (blue), `[N notes]` (dim), `[2h 30m]` logged time (green), due date (red +
  `(overdue!)` when overdue).
- **Detail panel** (scrollable viewport): Title, Status, Priority, Created,
  Due, Tags (magenta), Time logged (green), Blocked by / Blocks lists with live
  status of each referenced task, Description, Subtasks header `done/total` +
  progress bar (blue) + checklist, Notes header + timestamped notes, recent
  time entries. When the detail panel is focused, an inner cursor moves
  through actionable items (subtasks, notes, time entries) for
  toggle/edit/delete.
- **Resizable split**: `<` / `>` move the divider; ratio persisted to config.
- **Status bar**: shows the most relevant keybindings for the focused
  panel/modal. Keys in cyan, descriptions in light gray, separators dim.
  Transient messages (errors red, confirmations green) replace hints briefly.

### Keybindings (defaults)

| Key | Context | Action |
|---|---|---|
| `j/k`, `↓/↑` | lists | move selection |
| `enter` | task list | focus details panel |
| `1`–`4`, `tab` | task list | switch status tab |
| `a` / `e` / `d` | task list | add / edit / delete task |
| `a` | details | add subtask |
| `e` / `d` | details (cursor on subtask/note/time entry) | edit / delete that item |
| `J` / `K` | details, cursor on subtask | move subtask down / up |
| `s` | task list | cycle status; `S` opens status select |
| `space` | details, cursor on subtask | toggle done |
| `n` | details | add note |
| `t` | details | log time |
| `b` | details | manage dependencies |
| `/` | task list | fuzzy search (live; `esc` clears) |
| `o` | task list | cycle sort: created → due → priority |
| `p` | global | project picker |
| `<` / `>` | global | resize split |
| `?` | global | help overlay |
| `q` | global | quit (`esc` first backs out of modal/panel focus) |

The help overlay (`?`) renders the full reference grouped by context, generated
from the same key definitions in `keys.go`.

### Color theme (defined once in `styles.go`)

| Element | Color |
|---|---|
| Tags, project name | magenta `#c678dd` |
| Subtask counts, progress bar, notes | blue `#61afef` |
| Time logged, done status, LOW priority | green `#98c379` |
| In-progress status, MED priority | yellow `#e5c07b` |
| HIGH priority | orange `#d19a66` |
| URG priority, overdue, blocked ⛔ | red `#e06c75` |
| Focused panel border, active tab, status-bar keys | cyan `#56d6e0` |
| Labels, hints separators, no-priority badge | gray `#5c6370` |
| Status-bar key descriptions | light gray `#9da5b4` |

## Behaviors

- **Tabs & filters compose**: active tab (status filter) + fuzzy search apply
  together. Sort applies within the result.
- **Fuzzy search**: matches title, description, and tags via case-insensitive
  subsequence matching; ranking prefers earlier/denser matches. Tag filtering
  happens through this search (tags are part of the match corpus) — there is
  no separate tag-filter UI. Implemented in-repo as pure functions.
- **Forms**: modal forms composed of `bubbles/textinput` fields; tab/shift-tab
  between fields. Validation inline: required title, due date `YYYY-MM-DD`,
  priority chosen from list (or none), duration parse for time logging.
  Invalid fields show red hints; submit is blocked until valid. `esc` cancels.
- **Task form fields**: title*, description, priority (none/low/medium/high/
  urgent), due date (optional), tags. Project form: name*, description, tags.
- **Dependencies modal** (`b`): fuzzy-searchable list of same-project tasks;
  `space` toggles "this task blocks X". Shows current edges both directions.
  Cycle attempts show an error message and are not saved.
- **Delete confirmations**: all destructive actions (task, project, subtask,
  note, time entry deletion) require a confirm dialog. Task delete when the
  task blocks others: dialog lists the blocked tasks, plain confirm refused,
  `c` confirms cascade (removes dependency links, then deletes).
- **Project delete** cascades all contained data after an explicit
  confirmation that states the task count.
- **Time logging**: duration input accepts `90m`, `1h30m`, `1h 30m`, `2h`.
  Stored as minutes. Detail panel shows total and the most recent entries.
- **Status cycling** (`s`): pending → in_progress → done → pending.
- **Backups**: on startup, if `last_backup_date` (meta) is before today,
  `VACUUM INTO` a dated file in the backups dir, then prune to the newest 7.
- **Config** (TOML): `startup = "picker" | "last-project"` (default
  `"picker"`), `split_ratio` (auto-saved on resize), `db_path` override.
  Missing file = defaults, written on first run.

## Error handling

- Store errors surface as transient red status-bar messages; the UI never
  crashes on a failed operation.
- Startup failures (unreadable DB, corrupt config) print a clear message to
  stderr and exit nonzero.
- All writes are synchronous; an action is only reflected in the UI after the
  DB write succeeds.

## Testing

- **Store layer**: unit tests against a temp SQLite file — CRUD for all
  entities, tab filters, all three sorts (NULL ordering), tag normalization,
  overdue detection, delete guard + cascade, cycle rejection, backup rotation,
  migrations.
- **UI logic**: unit tests for pure functions — fuzzy matcher, duration
  parser, form validation, key routing tables.
- **Integration**: verified by running the real TUI; no snapshot tests.

## Out of scope

- CLI subcommands / JSON output (storage layer kept UI-free so this can be
  added later)
- Cross-project dependencies
- Recurring tasks, reminders/notifications, sync/multi-device, attachments
- Custom themes/keybinding remapping (colors centralized in `styles.go` for
  easy future work)
