# tskr — Specification

> **Purpose of this document.** This is a complete, implementation-ready
> specification for **tskr**, a terminal task manager. It is written to be built
> from scratch by an engineer (human or LLM) who has never seen the original
> code. It describes *what* the app does and *how it must behave* — the data
> model, persistence, configuration, full keyboard-driven TUI, layout, colors,
> and every interaction — while leaving the internal code architecture open.
>
> Treat every "MUST" as a hard requirement and every numeric/keybinding/string
> detail as exact. When two requirements seem to conflict, the more specific one
> wins. A correct implementation should be indistinguishable from the reference
> in observable behavior.

---

## 1. Overview

tskr is a single-user, offline, keyboard-driven **terminal UI (TUI)** task
manager with a three-level hierarchy:

```
Project  →  Task  →  Subtask
                  →  Note
                  →  Time entry
                  →  Dependencies (task blocks/blocked-by other tasks)
```

The user picks a **project**, then manages its **tasks** in a two-panel layout:
a task list on the left and a detail view of the selected task on the right.
Tasks have status, priority, due date, tags, subtasks, notes, logged time, and
inter-task dependencies. All data persists locally in a single SQLite database,
which is backed up automatically once per day.

The entire app is operated by the keyboard. There is no mouse interaction and no
network access.

### 1.1 Reference technology stack (recommended, not mandatory)

The reference implementation uses:

- **Go** (1.26+)
- **Bubble Tea** (`github.com/charmbracelet/bubbletea`) — the Elm-architecture TUI framework
- **Bubbles** (`github.com/charmbracelet/bubbles`) — text input & viewport widgets
- **Lip Gloss** (`github.com/charmbracelet/lipgloss`) — styling/layout
- **modernc.org/sqlite** — pure-Go SQLite driver (no cgo)
- **BurntSushi/toml** — config parsing

You MAY use another language/stack, but the resulting app MUST match the
behavior, layout, keybindings, persistence format semantics, and configuration
described here. The SQLite schema (Section 4) MUST be reproduced exactly so that
databases are interchangeable.

---

## 2. Application Architecture (conceptual)

The app follows the **Model-View-Update (Elm)** pattern, which you are
encouraged to mirror:

- A single **root model** owns global state: the current project, which panel is
  focused, the window size, a transient status message, and a **stack of modals**.
- **Components** (task list, detail panel, project picker, forms, confirm
  dialogs, select menus, dependency selector, help overlay) each have their own
  state and handle their own keys when active.
- A **store** layer wraps SQLite and exposes typed CRUD operations. The UI never
  writes SQL directly; it calls store methods.
- User actions that mutate data are expressed as **action messages** emitted by
  components and handled centrally by the root model, which performs the store
  mutation and then broadcasts a **Refresh** so every visible component reloads.

### 2.1 Modal stack

Modals (picker, forms, confirm dialogs, menus, dependency selector, help) are
kept on a **stack**. Only the **top** modal receives key input and is the only
thing rendered in the body area (centered). Opening a modal pushes; closing pops.
A "close modal" action pops the top entry. Some flows push a modal from within
another modal (e.g. the picker opens the new-project form on top of itself).

### 2.2 Refresh model

After any successful data mutation, the app issues a global **Refresh**. On
Refresh:
- The task list reloads its tasks for the current project/tab/sort/filter.
- The detail panel reloads the selected task (or clears if none).
- Every modal on the stack reloads its own data (e.g. the picker re-lists projects).

---

## 3. Data Model

All timestamps are stored as **RFC 3339 UTC strings** (e.g.
`2026-06-11T09:30:00Z`). Dates (due dates) are plain `YYYY-MM-DD` strings.

### 3.1 Project

| Field        | Type            | Notes |
|--------------|-----------------|-------|
| ID           | int (autoincr)  | Primary key |
| Name         | string          | Required, **unique**, non-empty |
| Description  | string          | Optional, default `""` |
| Tags         | string          | Normalized comma-separated (see §3.6), default `""` |
| Status       | enum            | `active` or `archived`; default `active` |
| CreatedAt    | timestamp       | Set on creation |
| UpdatedAt    | timestamp       | Set on creation and every update |
| TaskCount    | int (derived)   | Count of tasks in the project (computed on read, not stored) |

### 3.2 Task

| Field         | Type           | Notes |
|---------------|----------------|-------|
| ID            | int (autoincr) | Primary key |
| ProjectID     | int            | FK → project, `ON DELETE CASCADE` |
| Title         | string         | Required, non-empty |
| Description   | string         | Optional, default `""`, may contain newlines |
| Status        | enum           | `pending`, `in_progress`, `done`; default `pending` |
| Priority      | enum or none   | `low`, `medium`, `high`, `urgent`, or none (stored NULL); default none |
| DueDate       | date or none   | `YYYY-MM-DD` or none (stored NULL) |
| Tags          | string         | Normalized comma-separated, default `""` |
| CreatedAt     | timestamp      | |
| UpdatedAt     | timestamp      | Updated on every modification incl. status change |
| CompletedAt   | timestamp or none | Set to "now" when status becomes `done`; cleared (NULL) otherwise |

Derived/computed fields (not stored; computed on read):

| Field         | Meaning |
|---------------|---------|
| SubtasksDone  | Number of subtasks with `done = true` |
| SubtasksTotal | Total subtasks |
| NoteCount     | Number of notes |
| Minutes       | Sum of all time-entry minutes |
| Blocked       | True if any **incomplete** task blocks this one (a blocker whose status ≠ `done`) |
| BlockedBy     | List of task refs (id, title, status) that block this task |
| Blocks        | List of task refs that this task blocks |

### 3.3 Subtask

| Field        | Type           | Notes |
|--------------|----------------|-------|
| ID           | int (autoincr) | |
| TaskID       | int            | FK → task, `ON DELETE CASCADE` |
| Title        | string         | Required, non-empty |
| Description  | string         | Optional, default `""` |
| Done         | bool           | Default false |
| Position     | int            | Ordering within the task; lower = higher in list |
| CreatedAt    | timestamp      | |

### 3.4 Note

| Field     | Type           | Notes |
|-----------|----------------|-------|
| ID        | int (autoincr) | |
| TaskID    | int            | FK → task, `ON DELETE CASCADE` |
| Body      | string         | Required, non-empty |
| CreatedAt | timestamp      | |

### 3.5 Time entry

| Field     | Type           | Notes |
|-----------|----------------|-------|
| ID        | int (autoincr) | |
| TaskID    | int            | FK → task, `ON DELETE CASCADE` |
| Minutes   | int            | Must be **> 0** |
| Note      | string         | Optional, default `""` |
| CreatedAt | timestamp      | |

### 3.6 Dependencies

A dependency is a directed edge **blocker → blocked**: the blocker must be done
before the blocked task. Stored as a pair `(blocker_id, blocked_id)`.

Rules enforced when adding a dependency:
- A task **cannot block itself** (reject with a "self" error).
- Both tasks **must belong to the same project** (reject with a cross-project error).
- Adding the edge **must not create a cycle** in the blocker→blocked graph
  (reject with a "cycle" error). Detect by checking whether the proposed blocked
  task can already reach the proposed blocker via blocker→blocked edges.
- Duplicate edges are ignored (insert-or-ignore).

### 3.7 Tag normalization

Tag strings are stored **normalized**: split on commas, trim whitespace,
lowercase each tag, drop empties, **deduplicate** (preserving first-seen order),
and re-join with commas (no spaces). Example: `" Work, urgent ,work "` →
`work,urgent`. Applied on every project/task create and update.

---

## 4. Persistence

### 4.1 Storage engine & connection

- A single **SQLite** database file.
- **Foreign keys MUST be ON** for every connection (`PRAGMA foreign_keys = ON`).
- Use a busy timeout (~5000 ms) to tolerate transient locks.
- The reference limits the pool to a single open connection to keep SQLite
  writes serialized; an equivalent serialization strategy is acceptable.

### 4.2 File locations (XDG conventions)

| What        | Path |
|-------------|------|
| Database    | `$XDG_DATA_HOME/tskr/tskr.db`, else `~/.local/share/tskr/tskr.db` |
| Backups dir | `<db-dir>/backups/` |
| Config      | `$XDG_CONFIG_HOME/tskr/config.toml`, else `~/.config/tskr/config.toml` |

The database directory MUST be created if missing. The DB path is overridable
via config (`db_path`).

### 4.3 Schema (exact)

The database MUST use this schema. A `meta` key/value table tracks the schema
version for migrations.

```sql
CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS projects (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    tags        TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tasks (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id   INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title        TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','in_progress','done')),
    priority     TEXT CHECK (priority IN ('low','medium','high','urgent')), -- NULL = none
    due_date     TEXT,                                                      -- NULL = none
    tags         TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    completed_at TEXT                                                       -- NULL = not done
);

CREATE TABLE IF NOT EXISTS subtasks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id     INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    done        INTEGER NOT NULL DEFAULT 0,
    position    INTEGER NOT NULL,
    created_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS task_deps (
    blocker_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    blocked_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    PRIMARY KEY (blocker_id, blocked_id),
    CHECK (blocker_id != blocked_id)
);

CREATE TABLE IF NOT EXISTS notes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id    INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    body       TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS time_entries (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id    INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    minutes    INTEGER NOT NULL CHECK (minutes > 0),
    note       TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_subtasks_task ON subtasks(task_id);
CREATE INDEX IF NOT EXISTS idx_notes_task    ON notes(task_id);
CREATE INDEX IF NOT EXISTS idx_time_task     ON time_entries(task_id);
CREATE INDEX IF NOT EXISTS idx_deps_blocked  ON task_deps(blocked_id);
```

### 4.4 Migrations

On open:
1. Ensure the `meta` table exists.
2. If the database is **fresh** (no `schema_version` in `meta`), create the full
   schema in a transaction and set `schema_version = 2`.
3. If `schema_version = 1` (an older version that lacked `subtasks.description`),
   run `ALTER TABLE subtasks ADD COLUMN description TEXT NOT NULL DEFAULT ''`
   and set `schema_version = 2`.

The current schema version is **2**. (Implementations starting fresh only need to
support creating version 2; the v1→v2 migration matters only for upgrading
existing reference databases.)

### 4.5 `meta` keys used

| Key                 | Meaning |
|---------------------|---------|
| `schema_version`    | `"2"` |
| `last_project_id`   | ID of the most recently opened project (for "last-project" startup) |
| `last_backup_date`  | `YYYY-MM-DD` of the most recent backup |

`GetMeta` returns `""` for a missing key (not an error). `SetMeta` is an upsert.

### 4.6 Daily backup

At startup, after opening the DB, run a backup **if one has not been made today**:
1. Read `last_backup_date`. If it is `>=` today's date (`YYYY-MM-DD`), do nothing.
2. Otherwise create the backups directory, remove any existing
   `backups/tskr-<today>.db`, and run `VACUUM INTO 'backups/tskr-<today>.db'`.
3. Set `last_backup_date = today`.
4. Prune: keep only the **newest 7** `tskr-*.db` files (dated filenames sort
   chronologically); delete older ones.

A backup failure MUST be **non-fatal** — print a warning to stderr and continue.

---

## 5. Configuration

Config file: TOML at the path in §4.2. On first run (file missing), the app
writes the file populated with defaults. Invalid values fall back to defaults
rather than erroring.

### 5.1 Options

| Key            | Type    | Default          | Meaning |
|----------------|---------|------------------|---------|
| `startup`      | string  | `"last-project"` | `"last-project"` opens the most recently used project directly; `"picker"` opens the project picker on launch. Any other value is sanitized to `"picker"` on load. |
| `split_ratio`  | float   | `0.42`           | Left panel width as a fraction of total width. Clamped to `[0.2, 0.8]`; out-of-range values reset to `0.42`. |
| `db_path`      | string  | (see §4.2)       | Database file path. Empty resets to default. |
| `[colors]`     | table   | (see §7)         | Optional per-element color overrides. |

### 5.2 `[colors]` table

Each key is an optional hex color string `"#rrggbb"`. An empty/missing value
keeps the built-in default. These map to semantic roles (see §7 for where each
color is used):

| Key       | Role |
|-----------|------|
| `cyan`    | Cursor marker, focused borders, accents, **task description** label area, modal borders |
| `magenta` | Tags |
| `blue`    | Dates, note bodies, subtask progress bar/counter |
| `green`   | Done status, low priority, logged-time values |
| `yellow`  | In-progress status, medium priority |
| `orange`  | High priority, "blocks" relations |
| `red`     | Urgent priority, errors, overdue, blocked indicators |
| `gray`    | Key labels (Status, Priority, …), dim/secondary text, inactive tabs, panel borders |
| `light`   | Secondary text, form field labels, status-bar hint descriptions |
| `text`    | Primary body text |

Colors MUST be applied **once at startup** after the config is loaded, before
the UI renders.

### 5.3 Persisted runtime changes

When the user resizes the split with `<` / `>` (Section 9), the new
`split_ratio` MUST be written back to the config file immediately.

### 5.4 Example config

```toml
startup = "last-project"
split_ratio = 0.42
db_path = "/home/user/.local/share/tskr/tskr.db"

[colors]
cyan = "#56d6e0"
magenta = "#c678dd"
# others omitted → use defaults
```

---

## 6. TUI Layout

### 6.1 Overall

The app runs in the **alternate screen** (full-screen TUI; the terminal is
restored on exit). The screen always has total width `W` and height `H` (in
character cells) from the terminal size.

There are two top-level visual states:

1. **Main view** — when a project is open and no modal is on the stack.
2. **Modal view** — when at least one modal is on the stack; the top modal is
   centered over the screen.

If the window size is unknown (W = 0), render nothing yet.

### 6.2 Main view structure

Top to bottom:

```
┌─────────────────────────────────────────────────────────────────┐
│ Tab bar (1 line)                                                  │
├──────────────────────────────┬──────────────────────────────────┤
│ 1: Tasks (sort)              │ 2: Details                        │
│  ┌ left panel ──────────┐    │  ┌ right panel ───────────────┐   │
│  │ task rows            │    │  │ task detail viewport       │   │
│  │ ...                  │    │  │ ...                        │   │
│  └──────────────────────┘    │  └────────────────────────────┘   │
├──────────────────────────────┴──────────────────────────────────┤
│ Status bar (1 line)                                               │
└─────────────────────────────────────────────────────────────────┘
```

- The **tab bar** is the top line; the **status bar** is the bottom line. The
  body (panels) occupies `H − 2` lines.
- The body is split horizontally. Left width = `round(W × split_ratio)`; right
  takes the rest. Each panel is drawn inside a rounded border with a one-line
  title.
- The panel inner content height is `H − 2 − 3` (tab bar + status bar, then
  border top/bottom + title line).

### 6.3 Tab bar

Four tabs, **in this exact left-to-right order**:

```
All | Pending | In Progress | Done
```

- The active tab is highlighted (background-filled, bold). Inactive tabs are
  dim. Tabs are padded with one space on each side.
- Right-aligned on the same line: the **current project name** (styled as a tag).
  Pad with spaces so the project name sits at the right edge (minimum 1 space gap).

Tab key mapping (number keys): `1` → All, `2` → Pending, `3` → In Progress,
`4` → Done. (All is first.)

### 6.4 Left panel — task list

Title line: `1: Tasks (<sort>)` where `<sort>` is the current sort mode name in
parentheses, dimmed (`created`, `due`, or `priority`). The title is cyan when the
panel is focused, dim otherwise.

When a search is active or has a non-empty query, the first content line shows
the search input: a cyan `/ ` prefix followed by the text input.

If there are no visible tasks: show `no tasks — press a to add one` (dim).

Otherwise each task occupies **two lines**:

- **Line 1:** `<cursor> <priority-badge> <title> [⛔]`
  - `<cursor>` is `▸ ` (cyan) for the selected row, two spaces otherwise.
  - `<priority-badge>` is a fixed 3–4 char colored badge: `LOW` (green),
    `MED` (yellow), `HIGH` (orange), `URG` (red), or `———` (dim) for none.
  - `<title>` is bold when selected.
  - A red `⛔` is appended if the task is **blocked** (has an incomplete blocker).
- **Line 2:** indented metadata, space-separated, only including parts that apply:
  - `[<done>/<total>]` subtask counts (blue) when there are subtasks.
  - `[<n> notes]` (dim) when there are notes.
  - `[<duration>]` logged time (green) when minutes > 0, formatted like `1h 30m`.
  - Due date: `due <YYYY-MM-DD>` (light); if overdue, `due <date> (overdue!)` in red.

**Vertical scrolling:** the list shows up to `floor(panelHeight / 2)` task rows
(each row is 2 lines). Keep the selected row visible: when the selection index is
at or beyond the visible window, scroll so the selection stays in view (window
start = `selection − rows + 1`).

### 6.5 Right panel — task detail

Title line: `2: Details` (cyan when focused, dim otherwise). The content is a
**scrollable viewport** of the selected task. If no task is selected, show
`no task selected` (dim).

Detail content, in order (label column is left-padded to 11 characters, dim):

```
Title       <bold title>

Status      <colored status with glyph>
Priority    <colored priority name, or — >
Created     <Jan 2, 2006>
Due         <YYYY-MM-DD> | <date (overdue!) in red> | —
Tags        <tag, tag in magenta> | —
Time        <Xh Ym logged in green> | —
Blocked by  ⛔ <title> (status)        ← one line per blocker (red); label only on first
Blocks      <title> (status)          ← one line per blocked task (orange); label only on first

Description                           ← only if description non-empty
<description text, wrapped across lines>

Subtasks    <done/total in blue, or — >
<progress bar: blue filled blocks █ + dim empty ░, width 24>   ← only if subtasks exist
  [ ] <title>  <description dim>       ← [x] and green title when done
  ...

Notes       <count>
  <Jan 2 15:04 dim> <body in blue>
  ...

Time log                              ← only if entries exist
  <Jan 2 15:04 dim> <Xh Ym in green> <optional note>
  ...
```

Status glyphs/colors: `○ Pending` (dim), `◐ In Progress` (yellow),
`● Done` (green).

**Inner cursor.** The detail panel has its own cursor that moves over an ordered
list of **actionable items**: subtasks (in order), then notes, then time entries.
The item under the cursor is marked with a cyan `▸ ` (only when the panel is
focused). Moving the cursor scrolls the viewport to keep the current item visible.

Date formats:
- "Created" uses **`Jan 2, 2006`** (e.g. `Jun 11, 2026`).
- Notes/time entries use **`Jan 2 15:04`** (e.g. `Jun 11 11:33`), local time.

### 6.6 Status bar (bottom line)

- If there is a **transient status message**, show it: a leading space then the
  message. Error messages are red; informational messages are green. Status
  messages auto-clear after **3 seconds**.
- Otherwise show a **context-sensitive keybinding hint bar**: cyan keys with
  light descriptions, separated by ` · ` (dim). The hint set depends on focus:
  - Detail panel focused → detail hints.
  - Task list searching → search hints.
  - Otherwise → task-list hints.
- If the hint bar is wider than the screen, fall back to showing only the first
  6 hints.

**Important:** When a modal is open, the body shows the centered modal in the top
`H − 1` lines, and the **bottom line still renders the status message** (so that
errors/info raised during modal flows remain visible). Modals provide their own
in-box key hints, so only the status text (not the hint bar) shows on that line
while a modal is open.

### 6.7 Modal view

When a modal is on the stack, render the top modal centered horizontally and
vertically in the top `H − 1` rows, with the status line on the bottom row. Each
modal is drawn in a **rounded-border box with padding** (1 vertical, 2
horizontal). Confirm dialogs use a **red** border; all other modals use a **cyan**
border.

---

## 7. Color Palette

Default base colors (hex). These are intentionally on the lighter side for
readability on dark terminals. All are overridable via `[colors]` (§5.2).

| Name    | Default     | Primary uses |
|---------|-------------|--------------|
| magenta | `#ce88e4`   | tags |
| blue    | `#72bbf3`   | dates, note bodies, subtask progress/counts |
| green   | `#a5cf88`   | done, low priority, logged time, OK status |
| yellow  | `#e9ca8a`   | in-progress, medium priority |
| orange  | `#d9a876`   | high priority, "blocks" relation |
| red     | `#e87c84`   | urgent priority, errors, overdue, blocked |
| cyan    | `#62dde8`   | cursor, focused border, accents, modal border |
| gray    | `#7c8799`   | key labels, dim text, inactive tabs, unfocused borders |
| light   | `#adb8c7`   | secondary text, form labels, hint descriptions |
| text    | `#d4d8e2`   | primary text |
| bg      | `#11151c`   | background (used as foreground on active tab) |

Derived style notes:
- **Title** style is **bold** (no specific color — inherits terminal default).
- **Active tab**: cyan background, bg-colored foreground, bold, padded `0 1`.
- **Inactive tab**: gray foreground, padded `0 1`.
- **Panel border**: gray unfocused, cyan focused.
- **Modal border**: cyan (red for confirm dialogs).

---

## 8. Screens, Modals & Components

### 8.1 Project picker

A modal listing projects with a fuzzy search bar. Used at startup (when
configured or when no last project) and on demand via `p`.

- Title: `Select Project`.
- A search line: cyan `/ ` + text input (placeholder `fuzzy search…`).
- Project rows: `▸ ` (cyan) marker on the selected row (bold name), two spaces
  otherwise; then the name, then `  <n> tasks` (dim), then ` (archived)` (dim)
  for archived projects.
- If the filtered list is empty: `no projects — press n to create one` (dim).
- In-box hints: `j/k move · enter open · / search · n new · e edit · d delete ·
  s archive · A show archived`.

Behavior:
- By default only **active** projects are listed. Pressing `A` toggles inclusion
  of archived projects; the list re-sorts by name (case-insensitive).
- Fuzzy search filters by `name + " " + tags`. `/` enters search mode; typing
  filters live; `enter` accepts the filter and leaves search mode; `esc` clears
  the query and leaves search mode.
- `j/k` (or arrows) move the selection.
- `enter` opens the highlighted project (switches the main view to it).
- `n` opens the **new project form**.
- `e` opens the **edit project form** for the highlighted project.
- `d` opens a **delete confirmation** for the highlighted project.
- `s` toggles archive status of the highlighted project (active↔archived),
  reloads, and shows an info message `<name> → <status>`.
- `esc`/`q` close behavior: see §8.1.1.

**Picker is fuzzy-ordered** but displayed in name order; matching uses
subsequence matching (§11).

#### 8.1.1 Picker close vs. quit (the "allowClose" rule)

The picker has two modes:
- **Startup picker** (opened because no project is open): `esc`/`q` **quits the
  app**. The user must either open/create a project or quit.
- **On-demand picker** (opened with `p` while a project is already open):
  `esc`/`q` simply **closes** the picker, returning to the main view.

### 8.2 Forms (create/edit)

A modal with a vertical list of fields, a title, validation, and per-field error
display. Fields are either **text inputs** or **inline option selectors**.

Layout per field:
```
<Field label, in light color>
<input or selector>
[✗ <error text in red>]      ← only when invalid
```
Field **labels** are rendered in the **light** color (distinct from the dimmer
gray placeholder text inside empty inputs).

Bottom in-box hint bar:
`tab next field · ←/→ select option · enter save · esc cancel`.

Navigation & submission:
- `tab` / `down` → next field (wraps); `shift+tab` / `up` → previous field (wraps).
- For a focused **selector** field, `←`/`→` (or `h`/`l`) change the selected
  option (clamped at ends). The selected option is shown in cyan with a `▸`
  marker when focused, light when not focused but selected, dim otherwise; other
  options dim.
- For a focused **text** field, all other keys edit the text.
- `enter` validates **all** fields. If any validator returns a non-empty error,
  show the errors and keep the form open. If all valid, emit the submit action
  (trimming text values) **and** close the form.
- `esc` cancels (closes without saving).

The first field is focused on open (if it is a text field).

#### 8.2.1 Project form

Title `New project` or `Edit project`. Fields:
1. **Name** — text, required, placeholder `project name`.
2. **Description** — text, optional, no placeholder.
3. **Tags** — text, optional, placeholder `comma,separated`.

On submit: create or update the project. On create, the new project is **not**
auto-opened; the picker reappears (refreshed). On a unique-name collision, the
store returns an error which is surfaced on the status line (form still closes).

#### 8.2.2 Task form

Title `New task` or `Edit task`. Fields:
1. **Title** — text, required, placeholder `task title`.
2. **Description** — text, optional.
3. **Priority** — **inline selector** with options
   `None`, `Low`, `Medium`, `High`, `Urgent` (submitting the values
   ``, `low`, `medium`, `high`, `urgent`). Pre-selected to the task's current
   priority when editing, else `None`.
4. **Due date** — text, optional, placeholder `YYYY-MM-DD — empty = none`,
   validated as a date (see §11).
5. **Tags** — text, optional, placeholder `comma,separated`.

#### 8.2.3 Subtask form

Title `New subtask` or `Edit subtask`. Fields:
1. **Title** — text, required, placeholder `subtask title`.
2. **Description** — text, optional, placeholder `optional`.

#### 8.2.4 Single-text form

Used for **notes** (label `Note`) — a single required text field.

#### 8.2.5 Time form

Title `Log time` or `Edit time entry`. Fields:
1. **Duration** — text, required, placeholder `e.g. 1h 30m`, validated as a
   duration (§11). When editing, pre-filled with the formatted current value.
2. **Note** — text, optional, placeholder `optional`.

On submit, the duration string is parsed to minutes.

### 8.3 Confirm dialog

A red-bordered modal with a title, optional body lines, and key hints. Two
variants:

- **Normal:** `y`/`enter` confirms (emits the action with `cascade=false`) and
  closes; `n`/`esc`/`q` cancels. Hints: `y confirm · n/esc cancel`.
- **Cascade-only** (used when deleting a task that blocks other tasks): `y`/`enter`
  is **refused** with an info message (`this task blocks others — press c to
  delete with cascade, esc to abort`); only `c` confirms (emits the action with
  `cascade=true`) and closes; `esc` aborts. Hints: `c cascade delete · esc cancel`.

### 8.4 Select menu

A simple modal list (used for the **status menu**). Title, then one option per
line; the selected option is cyan with a `▸ ` marker. `j/k` (or arrows) move,
`enter` picks (emits action + closes), `esc`/`q` cancels.

### 8.5 Dependency selector

A modal for managing what the current task **blocks**.

- Title: `Dependencies — <task title>`.
- Subtitle (dim): `space marks tasks that this task blocks`.
- A fuzzy search line (cyan `/ ` + input).
- Candidate rows = all **other** tasks in the same project. Each row:
  `<cursor> <box> <title> [⛔ blocks this task]`
  - `<box>` is dim `[ ] ` normally, or orange `[b] ` if the current task blocks
    this candidate.
  - A red `⛔ blocks this task` suffix marks candidates that **block the current
    task** (informational; reverse direction).
- If no candidates: `no other tasks in this project` (dim).
- Hints: `j/k move · space toggle blocks · / search · esc done`.

Behavior:
- `j/k` move; `/` enters fuzzy search over `title + tags` (enter/esc leave search;
  esc clears).
- `space` toggles "current task blocks the highlighted candidate": if already
  blocking, remove the dependency; else add it. Dependency rules (§3.6) are
  enforced; on error (cycle, cross-project, self) show the error on the status
  line and do not change the edge. After a successful toggle, reload the modal.
- `esc`/`q` closes the modal **and** triggers a global Refresh (so the detail
  panel reflects new blocked/blocks state).

### 8.6 Help overlay

A modal showing the full keybinding reference, grouped by context. Title:
`tskr — keybindings`. Each group has a cyan heading and a list of
`key  description` rows (key column left-padded to 16 chars, cyan key, light
description). Footer (dim): `press any key to close`. **Any** key closes it.

Help groups and their entries:
- **Global:** `p` project picker, `<  >` resize split, `?` help, `q` quit,
  `esc` back / close.
- **Task list:** the task-list hints (see §9.2).
- **Task list (extra):** `tab / shift+tab` next / previous tab, `S` status menu.
- **Details:** the detail hints plus `J/K` reorder subtask.
- **Project picker:** the picker hints.
- **Forms:** the form hints.
- **Dependencies:** the deps hints.

---

## 9. Keybindings (complete)

Keys are matched on the **top modal** if any modal is open; otherwise on the main
view according to the focused panel. Below, "main view" means no modal is open.

### 9.1 Global (main view, any panel)

| Key        | Action |
|------------|--------|
| `ctrl+c`   | Quit |
| `q`        | Quit |
| `?`        | Open help overlay |
| `p`        | Open project picker (on-demand mode; `esc` closes it) |
| `<`        | Decrease split ratio by 0.05 (min 0.2), persist to config, relayout |
| `>`        | Increase split ratio by 0.05 (max 0.8), persist to config, relayout |

(When the task list search is active, typing keys go to the search box instead —
see §9.2.)

### 9.2 Task list panel (focused, not searching)

Hint bar set: `j/k move · enter details · 1-4 tab · a add · e edit · d delete ·
s status · / search · o sort · p projects · ? help`.

| Key            | Action |
|----------------|--------|
| `j` / `down`   | Move selection down |
| `k` / `up`     | Move selection up |
| `1`            | Switch to **All** tab (reset selection to top) |
| `2`            | Switch to **Pending** tab |
| `3`            | Switch to **In Progress** tab |
| `4`            | Switch to **Done** tab |
| `tab`          | Next tab (wraps All→Pending→In Progress→Done→All) |
| `shift+tab`    | Previous tab (wraps) |
| `o`            | Cycle sort: created → due → priority → created; show info `sort: <mode>` |
| `/`            | Enter search mode (focus the search box) |
| `esc`          | If a search query is set, clear it |
| `enter`        | If a task is selected, focus the **detail** panel |
| `a`            | Open **new task** form (for the current project) |
| `e`            | Open **edit task** form for the selected task |
| `d`            | Delete the selected task (with guard; see §10.4) |
| `s`            | Cycle the selected task's status: pending→in_progress→done→pending |
| `S`            | Open the **status select menu** for the selected task |

**Search mode** (task list): typing filters live; `enter` accepts the filter and
exits search mode; `esc` clears the query, exits search mode. Hint bar set:
`enter keep filter · esc clear`.

### 9.3 Detail panel (focused)

Hint bar set: `j/k move · space toggle · a subtask · n note · t log time · b deps ·
e edit · d delete · esc back`.

| Key           | Action |
|---------------|--------|
| `esc`         | Return focus to the task list |
| `j` / `down`  | Move inner cursor down (over subtasks → notes → time entries) |
| `k` / `up`    | Move inner cursor up |
| `space`       | If the current item is a subtask, toggle its done state |
| `J`           | Move the current subtask **down** (reorder); cursor follows |
| `K`           | Move the current subtask **up** (reorder); cursor follows |
| `a`           | Open **new subtask** form |
| `n`           | Open **new note** form |
| `t`           | Open **log time** form |
| `b`           | Open the **dependency selector** for this task |
| `e`           | Edit the current item (subtask / note / time entry) via the matching form |
| `d`           | Delete the current item (subtask / note / time entry) after confirmation |

Notes:
- If the detail panel is focused but the task becomes unavailable (e.g. nothing
  selected), focus falls back to the task list.
- Keys not listed (e.g. `pgup`/`pgdn`) pass through to the viewport for scrolling.

### 9.4 Modals

Each modal's keys are defined in Section 8. While a modal is open, **only** the
top modal handles keys (global main-view keys like `q`/`p` do not fire).

---

## 10. Feature Behaviors (detailed)

### 10.1 Startup flow

1. Load config (writing defaults if absent). Apply colors.
2. Open the database (creating/migrating as needed). Run the daily backup check.
3. Initialize the root model:
   - If `startup == "last-project"` **and** `meta.last_project_id` resolves to an
     existing **active** project, open that project directly (no picker).
   - Otherwise push the **startup picker** (which quits on `esc`/`q`).
4. Run the TUI on the alternate screen.

When a project is opened (from the picker or at startup), persist its ID to
`meta.last_project_id`.

### 10.2 Opening / switching projects

Opening a project: set it as current, point the task list at its ID, focus the
task list, clear/refresh the detail panel, persist `last_project_id`, and clear
all modals. The task list resets to the **Pending** tab by default for a freshly
constructed list.

### 10.3 Task status changes & automatic tab following

Two ways to change status: `s` (cycle) and `S` (menu). After a **successful**
status change:

- The store sets/clears `completed_at` (set to now when becoming `done`, cleared
  otherwise) and bumps `updated_at`.
- **Tab-follow rule:** If the **current tab is one of Pending / In Progress /
  Done**, the task list switches to the tab matching the task's **new** status
  and keeps the **same task selected** (the cursor lands on that task in the new
  tab after refresh). If the **current tab is All**, the status is changed but
  the tab does **not** switch (the task stays visible in All).

Implementation hint: record the target task ID before the refresh and, after the
list reloads under the new tab, position the cursor on that ID.

### 10.4 Task deletion guard

`d` on a task:
- Fetch the task. If it **blocks** other tasks, open a **cascade-only** confirm:
  title `Delete task "<title>"?`, body listing the blocked tasks
  (`This task blocks:` then `  ⛔ <title>` lines, then a blank line and
  `Cascade removes these dependency links (the tasks survive).`). Only `c`
  confirms (cascade delete: removes the dependency links in both directions, then
  deletes the task; dependent tasks survive). `y` is refused.
- If it blocks nothing, open a **normal** confirm: title `Delete task "<title>"?`,
  body `This cannot be undone.` `y`/`enter` deletes.

Subtasks, notes, and time entries are removed automatically by `ON DELETE
CASCADE`.

### 10.5 Project deletion

`d` in the picker opens a normal confirm: title `Delete project "<name>"?`, body
`This deletes the project and its <n> task(s).` On confirm, delete the project
(cascade removes its tasks and their children).

**Edge case:** If the deleted project is the **currently open** project, the app
must not strand the user. Clear the current project, clear the modal stack, and
push a fresh startup picker. More generally: whenever the modal stack becomes
empty **and** no project is open, push a fresh picker so the user always has
somewhere to go.

### 10.6 Subtasks

- New subtasks get `position = max(position)+1` within their task (appended).
- `space` toggles done (`done = 1 - done`).
- `J`/`K` swap the subtask with its neighbor below/above by swapping positions
  (no-op at the edges). The detail cursor moves with the subtask.
- Editing updates title and description.

### 10.7 Notes

- Added via the single-text note form; listed oldest-first (by `created_at`,
  then id). Editable and deletable from the detail inner cursor.

### 10.8 Time entries

- Added via the time form (duration required > 0, optional note). Listed
  **newest-first**. The task's `Minutes` (sum) appears in the list metadata,
  detail "Time" row, and per-entry rows.
- Duration parsing/formatting per §11.

### 10.9 Dependencies

- Managed via the dependency selector (§8.5). A task's **Blocked** flag is true
  only when at least one blocker is **not done**. The detail panel lists
  `Blocked by` (red ⛔) and `Blocks` (orange) relations with the related task's
  current status.

### 10.10 Sorting

Three sort modes, cycled with `o`:
- **created** (default): by `created_at`, then id (ascending).
- **due**: tasks with a due date first, ordered by due date ascending, then id;
  tasks without a due date last.
- **priority**: urgent → high → medium → low → none, then id.

### 10.11 Search / filtering

- Task list search matches against `title + " " + description + " " + tags` using
  fuzzy subsequence matching (§11). The filter applies on top of the current tab.
- Picker and dependency selector search match against `name/title + " " + tags`.

---

## 11. Validation & Parsing Rules

### 11.1 Required text

A required field is invalid (error text `required`) if it is empty after
trimming whitespace.

### 11.2 Date (optional)

Empty is valid (means "none"). Otherwise it MUST parse as `YYYY-MM-DD`; invalid
input yields error text `use YYYY-MM-DD`.

### 11.3 Priority

Stored value is one of `low`/`medium`/`high`/`urgent` or none. In the task form,
priority is chosen via a selector (no free-text validation needed). (A textual
validator that accepts empty or one of the four levels, case-insensitive, may
exist for completeness but is not user-facing once the selector is used.)

### 11.4 Duration

Accepts (case-insensitive, surrounding whitespace trimmed) forms matching hours
and/or minutes: e.g. `90m`, `2h`, `1h30m`, `1h 30m`. Formally, the pattern is
"optional `<n>h`, optional `<n>m`" — at least one must be present and the total
must be **> 0**. Invalid input yields an error like
`use formats like 90m, 2h, 1h30m`; zero/negative yields `duration must be
positive`.

Formatting minutes back to text: `Xh Ym` when both nonzero, `Xh` when only hours,
`Ym` otherwise; `0m` for non-positive.

### 11.5 Fuzzy matching

Case-insensitive **subsequence** match: query characters must appear in order
within the target (not necessarily contiguous). A scoring function ranks matches
(lower is better): score = sum of gaps between consecutive matched characters
plus the offset of the first match, so earlier and denser matches rank first. An
empty query matches everything. (Ranking is used where lists are fuzzy-ordered;
plain filtering only needs the boolean match.)

---

## 12. Status Messages

Transient one-line messages shown in the status bar, auto-clearing after 3
seconds. Examples that MUST be produced:

| Trigger | Message | Kind |
|---------|---------|------|
| Task created | `task created` | info |
| Task updated | `task updated` | info |
| Task deleted | `task deleted` | info |
| Status changed | `status: <status>` | info |
| Project created | `project created` | info |
| Project updated | `project updated` | info |
| Project deleted | `project deleted` | info |
| Project archived/unarchived | `<name> → <status>` | info |
| Sort changed | `sort: <mode>` | info |
| Any store error | the error text | error |
| Cascade-only confirm `y` | `this task blocks others — press c to delete with cascade, esc to abort` | info |

Errors raised during a modal flow MUST remain visible (status line renders under
the modal — see §6.6).

---

## 13. Non-Functional Requirements

- **Offline & local only.** No network calls.
- **Single binary**, no runtime services. Pure-Go SQLite (no cgo) is preferred so
  the binary is self-contained.
- **Crash-safe persistence.** Every mutation is committed immediately; the daily
  `VACUUM INTO` backup provides a recovery point.
- **Responsive layout.** The UI re-layouts on terminal resize.
- **Graceful degradation.** Backup failures and missing optional data never crash
  the app; invalid config values fall back to defaults.
- **Determinism.** Given the same database and inputs, rendering and ordering are
  deterministic (stable sorts with id tiebreakers).

---

## 14. Acceptance Checklist

A complete implementation should satisfy all of the following observable
behaviors (this doubles as a manual test script):

1. **Startup picker.** With no projects, launch shows the picker; submitting an
   empty project name is refused (form stays open). Creating `Demo` closes the
   form back to the picker; `Demo` exists. Pressing `enter` opens `Demo`.
2. **Last-project startup.** With `startup = "last-project"` and a valid last
   project, launching opens that project directly (no picker).
3. **Task create + validation.** `a` opens the task form; an invalid due date
   keeps the form open with an inline error; fixing it and submitting creates the
   task, which appears on the Pending tab.
4. **Priority selector.** The priority field is a left/right selector (None, Low,
   Medium, High, Urgent), not free text.
5. **Status + tab follow.** `S` then choosing In Progress moves the task and the
   tab auto-switches to In Progress with the task still selected; `s` cycles to
   Done (tab follows) and to Pending (tab follows); on the **All** tab, status
   changes keep you on All.
6. **Tabs order.** Tabs read `All | Pending | In Progress | Done`; keys `1..4` map
   in that order.
7. **Detail items.** `enter` focuses details. `a` adds subtasks (with optional
   description shown dim after the title); `space` toggles; `J/K` reorder; `n`
   adds a note; `t` logs time (`1h 30m` → 90 minutes shown).
8. **Dependencies + cycle.** `b` opens the dependency selector; `space` makes
   task A block task B; attempting to also make B block A surfaces a `cycle`
   error and is rejected.
9. **Delete guard.** Deleting a blocker is cascade-only (`y` refused, `c`
   cascades); the dependent task survives, now unblocked.
10. **Search & sort.** `/` filters live; `esc` clears; `o` cycles sort and shows
    `sort: due` etc.
11. **Split resize persists.** `>` widens the left panel and writes the new
    `split_ratio` to the config file immediately.
12. **Help overlay.** `?` opens the keybinding reference; any key closes it.
13. **Picker archive/delete.** In the picker, `s` archives; `A` reveals archived;
    deleting the current project clears it and lands you back in a fresh picker.
14. **Quit.** `q`/`esc` quits from the startup picker; `q` quits from the main
    view; on-demand picker `q`/`esc` only closes it.
15. **Colors configurable & lighter.** Default text colors are readable on dark
    terminals; `[colors]` overrides change the corresponding elements.
16. **Daily backup.** First launch of the day writes `backups/tskr-<date>.db` and
    keeps at most 7 backups.

---

## 15. Glossary

- **Project** — top-level container of tasks; active or archived.
- **Task** — unit of work with status, priority, due date, tags, and children.
- **Subtask** — ordered checklist item under a task.
- **Note** — timestamped free text under a task.
- **Time entry** — a logged duration (minutes) under a task.
- **Dependency** — a blocker→blocked relation between two tasks in one project.
- **Blocked** — a task with at least one incomplete blocker.
- **Tab** — a status filter over the current project's tasks (All / Pending /
  In Progress / Done).
- **Modal** — a centered overlay (picker, form, confirm, menu, deps, help) that
  captures all key input while on top of the stack.
- **Refresh** — global signal that reloads all visible data after a mutation.
```
