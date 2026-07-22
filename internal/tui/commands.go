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

func buildCmdRows(entries []model.Entry) []cmdRow {
	sorted := model.SortEntriesByPrimaryTag(entries)
	rows := make([]cmdRow, 0, len(sorted))
	var previousTag string
	previousTagged := false
	for i, entry := range sorted {
		primary, tagged := model.PrimaryTag(entry)
		showGroup := i == 0 || tagged != previousTagged || (tagged && !model.EqualTag(primary, previousTag))
		rows = append(rows, cmdRow{
			Group:     displayGroup(entry),
			ShowGroup: showGroup,
			Entry:     entry,
		})
		previousTag = primary
		previousTagged = tagged
	}
	return rows
}
