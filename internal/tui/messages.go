package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/valeriybagrintsev/gloss/internal/model"
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
