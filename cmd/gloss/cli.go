package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/mattn/go-runewidth"

	"github.com/Architeg/gloss/internal/alias"
	"github.com/Architeg/gloss/internal/model"
	"github.com/Architeg/gloss/internal/storage"
)

func runAddCLI(repo *storage.EntryRepo) error {
	ctx := context.Background()
	r := bufio.NewReader(os.Stdin)

	cmd, err := promptLine(r, "Command: ")
	if err != nil {
		return err
	}
	cmd = model.NormalizeCommand(cmd)
	if cmd == "" {
		return errors.New("command is required")
	}

	desc, err := promptLine(r, "Description: ")
	if err != nil {
		return err
	}
	tagsLine, err := promptLine(r, "Tags (comma-separated, optional): ")
	if err != nil {
		return err
	}

	e := model.Entry{
		Command:      cmd,
		Description:  desc,
		Tags:         model.ParseTagsCSV(tagsLine),
		Type:         model.EntryTypeManual,
		Source:       "manual",
		Target:       "",
		ManagedAlias: false,
	}
	if _, err := repo.CreateEntry(ctx, e); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Saved %q\n", cmd)
	return nil
}

func runListCLI(repo *storage.EntryRepo, tag string) error {
	ctx := context.Background()
	var entries []model.Entry
	var err error
	tag = strings.TrimSpace(tag)
	if tag != "" {
		entries, err = repo.GetEntriesByTag(ctx, tag)
	} else {
		entries, err = repo.GetAllEntries(ctx)
	}
	if err != nil {
		return err
	}
	entries = model.SortEntriesByPrimaryTag(entries)
	if len(entries) == 0 {
		if tag != "" {
			fmt.Println("No entries match that tag.")
			return nil
		}
		fmt.Println("No entries in glossary.")
		return nil
	}

	maxCmd := 12
	for _, e := range entries {
		if w := runewidth.StringWidth(e.Command); w > maxCmd {
			maxCmd = w
		}
	}
	if maxCmd > 36 {
		maxCmd = 36
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, e := range entries {
		desc := strings.ReplaceAll(strings.TrimSpace(e.Description), "\t", " ")
		tagsCol := ""
		if len(e.Tags) > 0 {
			tagsCol = "[" + strings.Join(e.Tags, ", ") + "]"
		}
		cmdCol := padRightVisual(e.Command, maxCmd)
		fmt.Fprintf(tw, "%s\t%s\t%s\n", cmdCol, desc, tagsCol)
	}
	return tw.Flush()
}

func padRightVisual(s string, target int) string {
	w := runewidth.StringWidth(s)
	if w > target {
		return truncateVisualRunes(s, target)
	}
	return s + strings.Repeat(" ", target-w)
}

func truncateVisualRunes(s string, maxW int) string {
	if maxW <= 1 {
		return "…"
	}
	w := 0
	var b strings.Builder
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if w+rw > maxW-1 {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + "…"
}

func runEditCLI(repo *storage.EntryRepo, command string) error {
	ctx := context.Background()
	cmd := model.NormalizeCommand(command)
	if cmd == "" {
		return errors.New("command is required")
	}

	e, err := repo.GetEntryByCommand(ctx, cmd)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no entry for %q", cmd)
		}
		return err
	}

	r := bufio.NewReader(os.Stdin)
	fmt.Fprintf(os.Stderr, "Editing %q (empty line keeps current value)\n", cmd)
	fmt.Fprintf(os.Stderr, "Description [%s]: ", e.Description)
	desc, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	desc = strings.TrimRight(desc, "\r\n")
	if desc != "" {
		e.Description = desc
	}

	fmt.Fprintf(os.Stderr, "Tags [%s]: ", strings.Join(e.Tags, ", "))
	tagsLine, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	tagsLine = strings.TrimRight(tagsLine, "\r\n")
	if tagsLine != "" {
		e.Tags = model.ParseTagsCSV(tagsLine)
	}

	if err := repo.UpdateEntry(ctx, e); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Updated %q\n", cmd)
	return nil
}

func runDeleteCLI(repo *storage.EntryRepo, command string) error {
	ctx := context.Background()
	cmd := model.NormalizeCommand(command)
	if cmd == "" {
		return errors.New("command is required")
	}
	if err := repo.DeleteEntryByCommand(ctx, cmd); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no entry for %q", cmd)
		}
		return err
	}
	fmt.Fprintf(os.Stderr, "Deleted %q\n", cmd)
	return nil
}

func promptLine(r *bufio.Reader, label string) (string, error) {
	fmt.Fprint(os.Stderr, label)
	s, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(s, "\r\n"), nil
}

func runAliasAddCLI(repo *storage.EntryRepo) error {
	ctx := context.Background()
	r := bufio.NewReader(os.Stdin)

	name, err := promptLine(r, "Alias name: ")
	if err != nil {
		return err
	}
	name = model.NormalizeCommand(name)
	if name == "" {
		return errors.New("alias name is required")
	}
	target, err := promptLine(r, "Expands to: ")
	if err != nil {
		return err
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return errors.New("expansion is required")
	}
	desc, err := promptLine(r, "Description (optional): ")
	if err != nil {
		return err
	}
	desc = strings.TrimSpace(desc)
	if desc == "" {
		desc = target
	}
	tagsLine, err := promptLine(r, "Tags (comma-separated, optional): ")
	if err != nil {
		return err
	}

	e := model.Entry{
		Command:      name,
		Description:  desc,
		Tags:         model.ParseTagsCSV(tagsLine),
		Type:         model.EntryTypeAlias,
		Source:       "managed",
		Target:       target,
		ManagedAlias: true,
	}
	if _, err := repo.CreateEntry(ctx, e); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Saved managed alias %q (run gloss alias sync to update shell)\n", name)
	return nil
}

func runAliasSyncCLI(cfg *model.Config, repo *storage.EntryRepo) error {
	ctx := context.Background()
	path, err := alias.ResolveShellPath(cfg)
	if err != nil {
		return err
	}
	entries, err := repo.GetManagedAliases(ctx)
	if err != nil {
		return err
	}
	res, err := alias.Sync(path, entries, 5)
	if err != nil {
		return err
	}
	if res.Noop {
		fmt.Println("Shell file already up to date.")
		return nil
	}
	if res.BackupPath != "" {
		fmt.Printf("Synced managed aliases to %s (backup: %s)\n", res.ShellPath, res.BackupPath)
	} else {
		fmt.Printf("Synced managed aliases to %s\n", res.ShellPath)
	}
	return nil
}

func runAliasDeleteCLI(repo *storage.EntryRepo, name string) error {
	ctx := context.Background()
	cmd := model.NormalizeCommand(name)
	if cmd == "" {
		return errors.New("alias name is required")
	}
	e, err := repo.GetEntryByCommand(ctx, cmd)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no entry for %q", cmd)
		}
		return err
	}
	if !e.ManagedAlias || e.Type != model.EntryTypeAlias {
		return fmt.Errorf("%q is not a managed alias", cmd)
	}
	r := bufio.NewReader(os.Stdin)
	fmt.Fprintf(os.Stderr, "Delete managed alias %q? [y/N]: ", cmd)
	line, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	line = strings.TrimSpace(strings.ToLower(strings.TrimRight(line, "\r\n")))
	if line != "y" && line != "yes" {
		fmt.Fprintln(os.Stderr, "Cancelled.")
		return nil
	}
	if err := repo.DeleteEntryByCommand(ctx, cmd); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Deleted managed alias %q\n", cmd)
	return nil
}
