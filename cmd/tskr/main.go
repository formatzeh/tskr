package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"tskr/internal/config"
	"tskr/internal/store"
	"tskr/internal/ui"
	"tskr/internal/ui/styles"
)

func main() {
	cfgPath := config.Path()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tskr: config:", err)
		os.Exit(1)
	}

	if len(os.Args) > 1 && os.Args[1] == "notify" {
		runNotify(cfg)
		return
	}

	styles.ApplyColors(cfg.Colors)
	styles.ApplyMarker(cfg.Marker)

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tskr: database:", err)
		os.Exit(1)
	}
	defer st.Close()

	today := time.Now().Format("2006-01-02")
	backupDir := filepath.Join(filepath.Dir(cfg.DBPath), "backups")
	if _, err := st.BackupIfDue(backupDir, 7, today); err != nil {
		fmt.Fprintln(os.Stderr, "tskr: backup failed:", err) // non-fatal
	}

	p := tea.NewProgram(ui.New(st, &cfg, cfgPath), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tskr:", err)
		os.Exit(1)
	}
}

func runNotify(cfg config.Config) {
	if !cfg.NotifyCron {
		return
	}
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tskr notify:", err)
		os.Exit(1)
	}
	defer st.Close()

	notifs, err := st.DueNotifications()
	if err != nil {
		fmt.Fprintln(os.Stderr, "tskr notify:", err)
		os.Exit(1)
	}
	for _, n := range notifs {
		u := "--urgency=" + n.Urgency
		title := n.Title
		if title == "" {
			title = "tskr"
		}
		exec.Command("notify-send", u, title, n.Body).Run()
		st.MarkNotificationSent(n.ID)
	}
}
