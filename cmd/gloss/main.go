package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/valeriybagrintsev/gloss/internal/config"
	"github.com/valeriybagrintsev/gloss/internal/model"
	"github.com/valeriybagrintsev/gloss/internal/scan"
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

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "scan":
			if err := runScanCLI(cfg, repo); err != nil {
				fmt.Fprintf(os.Stderr, "gloss: scan: %v\n", err)
				os.Exit(1)
			}
			return
		case "add":
			if err := runAddCLI(repo); err != nil {
				fmt.Fprintf(os.Stderr, "gloss: add: %v\n", err)
				os.Exit(1)
			}
			return
		case "list":
			if err := runListCLI(repo); err != nil {
				fmt.Fprintf(os.Stderr, "gloss: list: %v\n", err)
				os.Exit(1)
			}
			return
		case "edit":
			if len(os.Args) < 3 || strings.TrimSpace(os.Args[2]) == "" {
				fmt.Fprintln(os.Stderr, "gloss: usage: gloss edit <command>")
				os.Exit(1)
			}
			if err := runEditCLI(repo, os.Args[2]); err != nil {
				fmt.Fprintf(os.Stderr, "gloss: edit: %v\n", err)
				os.Exit(1)
			}
			return
		case "delete":
			if len(os.Args) < 3 || strings.TrimSpace(os.Args[2]) == "" {
				fmt.Fprintln(os.Stderr, "gloss: usage: gloss delete <command>")
				os.Exit(1)
			}
			if err := runDeleteCLI(repo, os.Args[2]); err != nil {
				fmt.Fprintf(os.Stderr, "gloss: delete: %v\n", err)
				os.Exit(1)
			}
			return
		case "alias":
			if len(os.Args) < 3 {
				fmt.Fprintln(os.Stderr, "gloss: usage: gloss alias <add|sync|delete>")
				os.Exit(1)
			}
			switch os.Args[2] {
			case "add":
				if err := runAliasAddCLI(repo); err != nil {
					fmt.Fprintf(os.Stderr, "gloss: alias add: %v\n", err)
					os.Exit(1)
				}
			case "sync":
				if err := runAliasSyncCLI(cfg, repo); err != nil {
					fmt.Fprintf(os.Stderr, "gloss: alias sync: %v\n", err)
					os.Exit(1)
				}
			case "delete":
				if len(os.Args) < 4 || strings.TrimSpace(os.Args[3]) == "" {
					fmt.Fprintln(os.Stderr, "gloss: usage: gloss alias delete <name>")
					os.Exit(1)
				}
				if err := runAliasDeleteCLI(repo, os.Args[3]); err != nil {
					fmt.Fprintf(os.Stderr, "gloss: alias delete: %v\n", err)
					os.Exit(1)
				}
			default:
				fmt.Fprintln(os.Stderr, "gloss: usage: gloss alias <add|sync|delete>")
				os.Exit(1)
			}
			return
		case "help", "-h", "--help":
			printCLIHelp()
			return
		default:
			fmt.Fprintf(os.Stderr, "gloss: unknown command %q (try gloss help)\n", os.Args[1])
			os.Exit(1)
		}
	}

	p := tea.NewProgram(
		tui.New(tui.Options{Config: cfg, Repo: repo}),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printCLIHelp() {
	fmt.Println(`Gloss — command glossary

Terminal commands (no TUI):
  gloss add              add an entry (interactive prompts)
  gloss list             print all entries
  gloss scan             list scan suggestions (print only)
  gloss edit <command>   edit description/tags (interactive)
  gloss delete <command> remove an entry
  gloss alias add           add a managed alias (does not write ~/.zshrc)
  gloss alias sync          write managed alias block to shell file (with backup)
  gloss alias delete <name> remove a managed alias from Gloss

  gloss, gloss help      show this help

Launch the TUI:
  gloss`)
}

func runScanCLI(cfg *model.Config, repo *storage.EntryRepo) error {
	ctx := context.Background()
	entries, err := repo.GetAllEntries(ctx)
	if err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		existing[model.NormalizeCommand(e.Command)] = struct{}{}
	}
	res, err := scan.Run(cfg, existing)
	if err != nil {
		return err
	}

	fmt.Println("Gloss scan")
	fmt.Println()
	fmt.Println("Sources:")
	for _, p := range res.Sources {
		fmt.Printf("  %s\n", p)
	}
	if len(res.SkippedPaths) > 0 {
		fmt.Println()
		fmt.Println("Unavailable paths:")
		for _, p := range res.SkippedPaths {
			fmt.Printf("  %s\n", p)
		}
	}
	fmt.Println()
	fmt.Printf("Suggestions: %d new (%d already in glossary, skipped)\n\n", len(res.Suggestions), res.SkippedExisting)

	for _, s := range res.Suggestions {
		detail := strings.TrimSpace(s.Target)
		if detail == "" {
			detail = s.Source
		}
		if utf8.RuneCountInString(detail) > 64 {
			runes := []rune(detail)
			detail = string(runes[:61]) + "..."
		}
		fmt.Printf("  %-18s  %-10s  %s\n", s.Command, s.Type, detail)
	}
	return nil
}
