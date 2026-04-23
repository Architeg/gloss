package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/valeriybagrintsev/gloss/internal/config"
	"github.com/valeriybagrintsev/gloss/internal/storage"
	"github.com/valeriybagrintsev/gloss/internal/tui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gloss: config: %v\n", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(cfg.StoragePath, "gloss.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gloss: database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	repo := storage.NewEntryRepo(db)
	p := tea.NewProgram(
		tui.New(tui.Options{Config: cfg, Repo: repo}),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
