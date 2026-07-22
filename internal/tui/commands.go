package tui

import (
	"strings"

	"github.com/Architeg/gloss/internal/model"
)

type commandsPhase int

const (
	commandsBrowse commandsPhase = iota
	commandsDetail
	commandsDeleteConfirm
	commandsEdit
	commandsBulkTags
)

type commandsFocus int

const (
	commandsFocusList commandsFocus = iota
	commandsFocusSearch
	commandsFocusTag
)

type cmdRow struct {
	Group     string
	ShowGroup bool
	Entry     model.Entry
}

func filterEntries(all []model.Entry, search, tag string) []model.Entry {
	search = strings.TrimSpace(search)
	tag = strings.TrimSpace(tag)
	var out []model.Entry
	for _, e := range all {
		if tag != "" && !model.EntryHasTag(e, tag) {
			continue
		}
		if search != "" {
			if !model.ContainsFold(e.Command, search) && !model.ContainsFold(e.Description, search) {
				continue
			}
		}
		out = append(out, e)
	}
	return out
}

func displayGroup(e model.Entry) string {
	primary, ok := model.PrimaryTag(e)
	if !ok {
		return "Untagged"
	}
	return primary
}

func samePrimaryGroup(a, b model.Entry) bool {
	aTag, aTagged := model.PrimaryTag(a)
	bTag, bTagged := model.PrimaryTag(b)
	if aTagged != bTagged {
		return false
	}
	return !aTagged || model.EqualTag(aTag, bTag)
}

func buildCmdRows(entries []model.Entry) []cmdRow {
	sorted := model.SortEntriesByPrimaryTag(entries)
	rows := make([]cmdRow, 0, len(sorted))
	for i, entry := range sorted {
		showGroup := i == 0 || !samePrimaryGroup(sorted[i-1], entry)
		rows = append(rows, cmdRow{
			Group:     displayGroup(entry),
			ShowGroup: showGroup,
			Entry:     entry,
		})
	}
	return rows
}
