package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Architeg/gloss/internal/buildinfo"
	"github.com/Architeg/gloss/internal/config"
	"github.com/Architeg/gloss/internal/model"
	"github.com/Architeg/gloss/internal/scan"
	"github.com/Architeg/gloss/internal/storage"
	"github.com/Architeg/gloss/internal/tui"
	"github.com/Architeg/gloss/internal/update"
)

var (
	newUpdateClient = func() *update.Client {
		return update.NewClient(&http.Client{Timeout: 30 * time.Second})
	}
	inspectUpdateExecutable = update.InspectRunningExecutable
	installVerifiedUpdate   = update.InstallVerified
)

func main() {
	appVersion := buildinfo.Version()
	invocation, handled, exitCode := earlyDispatch(os.Args[1:], os.Stdout, os.Stderr, appVersion)
	if handled {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return
	}

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

	if invocation.name != "" {
		switch invocation.name {
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
			if err := runListCLI(repo, invocation.tag); err != nil {
				fmt.Fprintf(os.Stderr, "gloss: list: %v\n", err)
				os.Exit(1)
			}
			return
		case "edit":
			if err := runEditCLI(repo, invocation.command); err != nil {
				fmt.Fprintf(os.Stderr, "gloss: edit: %v\n", err)
				os.Exit(1)
			}
			return
		case "delete":
			if err := runDeleteCLI(repo, invocation.command); err != nil {
				fmt.Fprintf(os.Stderr, "gloss: delete: %v\n", err)
				os.Exit(1)
			}
			return
		case "alias":
			switch invocation.aliasAction {
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
				if err := runAliasDeleteCLI(repo, invocation.command); err != nil {
					fmt.Fprintf(os.Stderr, "gloss: alias delete: %v\n", err)
					os.Exit(1)
				}
			}
			return
		}
	}

	p := tea.NewProgram(
		tui.New(tui.Options{
			Config:               cfg,
			Repo:                 repo,
			UpdateChecker:        update.NewClient(&http.Client{Timeout: 10 * time.Second}),
			Version:              appVersion,
			UpdateState:          update.StateStore{Path: filepath.Join(cfg.StoragePath, "update-state.json")},
			InspectUpdateLayout:  inspectUpdateExecutable,
			UpdateTimeout:        10 * time.Second,
			SaveUpdatePreference: config.SaveCheckForUpdates,
		}),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type invocation struct {
	name          string
	tag           string
	command       string
	aliasAction   string
	updateInstall bool
}

func earlyDispatch(args []string, stdout, stderr io.Writer, appVersion string) (invocation, bool, int) {
	inv, err := parseInvocation(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return invocation{}, true, 1
	}
	switch inv.name {
	case "version":
		printVersion(stdout, appVersion)
		return inv, true, 0
	case "help":
		printCLIHelp(stdout)
		return inv, true, 0
	case "update-help":
		printUpdateHelp(stdout)
		return inv, true, 0
	case "update":
		if err := runUpdateCLI(context.Background(), stdout, inv.updateInstall, appVersion, newUpdateClient(), inspectUpdateExecutable, installVerifiedUpdate); err != nil {
			fmt.Fprintf(stderr, "gloss: update: %v\n", err)
			return inv, true, 1
		}
		return inv, true, 0
	default:
		return inv, false, 0
	}
}

func parseInvocation(args []string) (invocation, error) {
	if len(args) == 0 {
		return invocation{}, nil
	}
	switch args[0] {
	case "version", "--version", "-v":
		return invocation{name: "version"}, nil
	case "help", "-h", "--help":
		return invocation{name: "help"}, nil
	case "update":
		switch {
		case len(args) == 1:
			return invocation{name: "update"}, nil
		case len(args) == 2 && args[1] == "--install":
			return invocation{name: "update", updateInstall: true}, nil
		case len(args) == 2 && (args[1] == "--help" || args[1] == "-h"):
			return invocation{name: "update-help"}, nil
		default:
			return invocation{}, fmt.Errorf("gloss: usage: gloss update [--install]")
		}
	case "scan":
		if len(args) > 1 {
			return invocation{}, fmt.Errorf("gloss: usage: gloss scan")
		}
		return invocation{name: "scan"}, nil
	case "add":
		return invocation{name: "add"}, nil
	case "list":
		tag, err := parseListArgs(args[1:])
		if err != nil {
			return invocation{}, err
		}
		return invocation{name: "list", tag: tag}, nil
	case "edit", "delete":
		if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
			return invocation{}, fmt.Errorf("gloss: usage: gloss %s <command>", args[0])
		}
		return invocation{name: args[0], command: args[1]}, nil
	case "alias":
		if len(args) < 2 {
			return invocation{}, fmt.Errorf("gloss: usage: gloss alias <add|sync|delete>")
		}
		switch args[1] {
		case "add", "sync":
			if len(args) > 2 {
				return invocation{}, fmt.Errorf("gloss: usage: gloss alias %s", args[1])
			}
			return invocation{name: "alias", aliasAction: args[1]}, nil
		case "delete":
			if len(args) != 3 || strings.TrimSpace(args[2]) == "" {
				return invocation{}, fmt.Errorf("gloss: usage: gloss alias delete <name>")
			}
			return invocation{name: "alias", aliasAction: "delete", command: args[2]}, nil
		default:
			return invocation{}, fmt.Errorf("gloss: usage: gloss alias <add|sync|delete>")
		}
	default:
		return invocation{}, fmt.Errorf("gloss: unknown command %q (try gloss help)", args[0])
	}
}

func printVersion(w io.Writer, appVersion string) {
	fmt.Fprintf(w, "gloss %s\n", appVersion)
}

func printCLIHelp(w io.Writer) {
	fmt.Fprintln(w, `Gloss — command glossary

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
  gloss update                 check for a stable update
  gloss update --install       securely install a stable update

  gloss help                   show this help

Launch TUI:
  gloss`)
}

func printUpdateHelp(w io.Writer) {
	fmt.Fprintln(w, `Usage:
  gloss update             check for a stable update
  gloss update --install   verify and install a stable update

Automatic update checks never install updates.`)
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
