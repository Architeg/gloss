package tui

import (
	"sort"
	"strings"

	"github.com/valeriybagrintsev/gloss/internal/model"
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
	search = strings.TrimSpace(strings.ToLower(search))
	tag = strings.TrimSpace(tag)
	var out []model.Entry
	for _, e := range all {
		if tag != "" && !entryHasExactTag(e, tag) {
			continue
		}
		if search != "" {
			c := strings.ToLower(e.Command)
			d := strings.ToLower(e.Description)
			if !strings.Contains(c, search) && !strings.Contains(d, search) {
				continue
			}
		}
		out = append(out, e)
	}
	return out
}

func entryHasExactTag(e model.Entry, tag string) bool {
	for _, t := range e.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

func displayGroup(e model.Entry) string {
	if len(e.Tags) == 0 {
		return "Untagged"
	}
	return e.Tags[0]
}

func buildCmdRows(entries []model.Entry) []cmdRow {
	groups := map[string][]model.Entry{}
	for _, e := range entries {
		g := displayGroup(e)
		groups[g] = append(groups[g], e)
	}
	names := make([]string, 0, len(groups))
	for k := range groups {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool {
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})
	var rows []cmdRow
	for _, g := range names {
		list := groups[g]
		sort.Slice(list, func(i, j int) bool {
			return strings.ToLower(list[i].Command) < strings.ToLower(list[j].Command)
		})
		for i, e := range list {
			rows = append(rows, cmdRow{
				Group:     g,
				ShowGroup: i == 0,
				Entry:     e,
			})
		}
	}
	return rows
}
