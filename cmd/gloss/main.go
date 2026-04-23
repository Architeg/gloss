package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/valeriybagrintsev/gloss/internal/config"
	"github.com/valeriybagrintsev/gloss/internal/model"
	"github.com/valeriybagrintsev/gloss/internal/scan"
	"github.com/valeriybagrintsev/gloss/internal/storage"
	"github.com/valeriybagrintsev/gloss/internal/tui"
)

var (
	Version = "0.1.0"
	Commit  = "dev"
	Date    = "unknown"
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
		case "version", "--version", "-v":
			printVersion()
			return
		case "scan":
			if len(os.Args) > 2 {
				fmt.Fprintln(os.Stderr, "gloss: usage: gloss scan")
				os.Exit(1)
			}
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
			tag, err := parseListArgs(os.Args[2:])
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(1)
			}
			if err := runListCLI(repo, tag); err != nil {
				fmt.Fprintf(os.Stderr, "gloss: list: %v\n", err)
				os.Exit(1)
			}
			return
		case "edit":
			if len(os.Args) != 3 || strings.TrimSpace(os.Args[2]) == "" {
				fmt.Fprintln(os.Stderr, "gloss: usage: gloss edit <command>")
				os.Exit(1)
			}
			if err := runEditCLI(repo, os.Args[2]); err != nil {
				fmt.Fprintf(os.Stderr, "gloss: edit: %v\n", err)
				os.Exit(1)
			}
			return
		case "delete":
			if len(os.Args) != 3 || strings.TrimSpace(os.Args[2]) == "" {
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
				if len(os.Args) > 3 {
					fmt.Fprintln(os.Stderr, "gloss: usage: gloss alias add")
					os.Exit(1)
				}
				if err := runAliasAddCLI(repo); err != nil {
					fmt.Fprintf(os.Stderr, "gloss: alias add: %v\n", err)
					os.Exit(1)
				}
			case "sync":
				if len(os.Args) > 3 {
					fmt.Fprintln(os.Stderr, "gloss: usage: gloss alias sync")
					os.Exit(1)
				}
				if err := runAliasSyncCLI(cfg, repo); err != nil {
					fmt.Fprintf(os.Stderr, "gloss: alias sync: %v\n", err)
					os.Exit(1)
				}
			case "delete":
				if len(os.Args) != 4 || strings.TrimSpace(os.Args[3]) == "" {
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

func printVersion() {
	fmt.Printf("gloss %s\n", Version)
}

func printCLIHelp() {
	fmt.Println(`Gloss — command glossary

Terminal (no TUI):
  gloss version                print version
  gloss --version              print version
  gloss -v                     print version
  gloss add                    add an entry (prompts)
  gloss list [--tag <tag>]     list entries, optionally filter by tag
  gloss scan                   print scan suggestions (no import)
  gloss edit <command>         edit description/tags (prompts)
  gloss delete <command>       remove an entry
  gloss alias add              add managed alias (stored only; sync separately)
  gloss alias sync             write managed block to shell file (backup if needed)
  gloss alias delete <name>    remove a managed alias

  gloss help                   show this help

Launch TUI:
  gloss`)
}

func parseListArgs(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	if len(args) == 2 && (args[0] == "--tag" || args[0] == "-t") {
		t := strings.TrimSpace(args[1])
		if t == "" {
			return "", fmt.Errorf("gloss: usage: gloss list [--tag <tag>]")
		}
		return t, nil
	}
	return "", fmt.Errorf("gloss: usage: gloss list [--tag <tag>]")
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

	fmt.Println("Gloss — scan")
	fmt.Println()
	fmt.Println("Sources")
	for _, p := range res.Sources {
		fmt.Printf("  %s\n", p)
	}
	if len(res.SkippedPaths) > 0 {
		fmt.Println()
		fmt.Println("Unavailable (skipped)")
		for _, p := range res.SkippedPaths {
			fmt.Printf("  %s\n", p)
		}
	}
	fmt.Println()
	fmt.Printf("Summary   %d new   ·   %d already in glossary\n", len(res.Suggestions), res.SkippedExisting)
	fmt.Println()

	if len(res.Suggestions) == 0 {
		fmt.Println("No new suggestions.")
		return nil
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "COMMAND\tTYPE\tDETAIL")
	for _, s := range res.Suggestions {
		detail := strings.TrimSpace(s.Target)
		if detail == "" {
			detail = s.Source
		}
		if utf8.RuneCountInString(detail) > 72 {
			runes := []rune(detail)
			detail = string(runes[:69]) + "..."
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", s.Command, s.Type, detail)
	}
	return tw.Flush()
}
