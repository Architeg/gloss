package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/valeriybagrintsev/gloss/internal/alias"
	"github.com/valeriybagrintsev/gloss/internal/model"
	"github.com/valeriybagrintsev/gloss/internal/openurl"
	"github.com/valeriybagrintsev/gloss/internal/scan"
	"github.com/valeriybagrintsev/gloss/internal/storage"
)

type entriesMsg struct {
	err     error
	entries []model.Entry
}

type saveMsg struct {
	err error
}

type deleteMsg struct {
	err error
}

type scanMsg struct {
	err             error
	sources         []string
	suggestions     []model.ScanSuggestion
	skippedExisting int
	skippedPaths    []string
}

type importScanMsg struct {
	err      error
	imported int
}

type syncAliasesMsg struct {
	err        error
	backupPath string
	shellPath  string
	noop       bool
}

type openURLMsg struct {
	err error
}

func openURLCmd(url string) tea.Cmd {
	return func() tea.Msg {
		return openURLMsg{err: openurl.Open(url)}
	}
}

func loadEntriesCmd(repo *storage.EntryRepo) tea.Cmd {
	if repo == nil {
		return nil
	}
	return func() tea.Msg {
		entries, err := repo.GetAllEntries(context.Background())
		return entriesMsg{err: err, entries: entries}
	}
}

func saveCreateCmd(repo *storage.EntryRepo, e model.Entry) tea.Cmd {
	return func() tea.Msg {
		_, err := repo.CreateEntry(context.Background(), e)
		return saveMsg{err: err}
	}
}

func saveUpdateCmd(repo *storage.EntryRepo, e model.Entry) tea.Cmd {
	return func() tea.Msg {
		err := repo.UpdateEntry(context.Background(), e)
		return saveMsg{err: err}
	}
}

func deleteEntryCmd(repo *storage.EntryRepo, command string) tea.Cmd {
	return func() tea.Msg {
		err := repo.DeleteEntryByCommand(context.Background(), command)
		return deleteMsg{err: err}
	}
}

func runScanCmd(cfg *model.Config, repo *storage.EntryRepo) tea.Cmd {
	if cfg == nil || repo == nil {
		return nil
	}
	return func() tea.Msg {
		ctx := context.Background()
		entries, err := repo.GetAllEntries(ctx)
		if err != nil {
			return scanMsg{err: err}
		}
		existing := make(map[string]struct{}, len(entries))
		for _, e := range entries {
			existing[model.NormalizeCommand(e.Command)] = struct{}{}
		}
		res, err := scan.Run(cfg, existing)
		if err != nil {
			return scanMsg{err: err}
		}
		return scanMsg{
			sources:         res.Sources,
			suggestions:     res.Suggestions,
			skippedExisting: res.SkippedExisting,
			skippedPaths:    res.SkippedPaths,
		}
	}
}

func importScanCmd(repo *storage.EntryRepo, rows []model.ScanSuggestion) tea.Cmd {
	if repo == nil {
		return nil
	}
	return func() tea.Msg {
		ctx := context.Background()
		n := 0
		var firstErr error
		for _, s := range rows {
			if !s.Selected {
				continue
			}
			e := scan.SuggestionToEntry(s)
			_, err := repo.CreateEntry(ctx, e)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			n++
		}
		return importScanMsg{err: firstErr, imported: n}
	}
}

func syncAliasesCmd(cfg *model.Config, repo *storage.EntryRepo) tea.Cmd {
	if cfg == nil || repo == nil {
		return nil
	}
	return func() tea.Msg {
		path, err := alias.ResolveShellPath(cfg)
		if err != nil {
			return syncAliasesMsg{err: err}
		}
		ctx := context.Background()
		entries, err := repo.GetManagedAliases(ctx)
		if err != nil {
			return syncAliasesMsg{err: err, shellPath: path}
		}
		res, err := alias.Sync(path, entries, 5)
		if err != nil {
			return syncAliasesMsg{err: err, shellPath: path}
		}
		return syncAliasesMsg{shellPath: res.ShellPath, backupPath: res.BackupPath, noop: res.Noop}
	}
}
