# tskr

A fast terminal task manager: projects → tasks → subtasks, with
dependencies, notes, time logging, and daily SQLite backups.
Built with Go and Bubble Tea.

## Install

    go build -o tskr ./cmd/tskr
    # or
    go install ./cmd/tskr

## Usage

Run `tskr`. Press `?` inside the app for the full keybinding reference.

- Data: `~/.local/share/tskr/tskr.db` (daily backups alongside, 7 kept)
- Config: `~/.config/tskr/config.toml` — `startup = "picker" | "last-project"`,
  `split_ratio`, `db_path`

Design and implementation docs live in `docs/superpowers/`.
