package alias

import (
	"fmt"
	"sort"
	"strings"

	"github.com/valeriybagrintsev/gloss/internal/model"
)

// Block markers in ~/.zshrc (or configured shell file).
const (
	StartMarker = "# >>> gloss aliases >>>"
	EndMarker   = "# <<< gloss aliases <<<"
)

// ManagedAliases returns managed alias entries sorted by command (case-insensitive).
func ManagedAliases(entries []model.Entry) []model.Entry {
	var out []model.Entry
	for _, e := range entries {
		if e.ManagedAlias && e.Type == model.EntryTypeAlias {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Command) < strings.ToLower(out[j].Command)
	})
	return out
}

// RenderManagedBlock builds the gloss-managed alias block including marker lines.
func RenderManagedBlock(entries []model.Entry) string {
	managed := ManagedAliases(entries)
	var b strings.Builder
	b.WriteString(StartMarker)
	b.WriteByte('\n')
	for _, e := range managed {
		b.WriteString(fmt.Sprintf("alias %s=%s\n", e.Command, doubleQuoteExpand(e.Target)))
	}
	b.WriteString(EndMarker)
	return b.String()
}

func doubleQuoteExpand(s string) string {
	var out strings.Builder
	out.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\', '"':
			out.WriteByte('\\')
			out.WriteByte(c)
		default:
			out.WriteByte(c)
		}
	}
	out.WriteByte('"')
	return out.String()
}

// MergeShellContent replaces an existing marked block or appends the block to the end.
func MergeShellContent(existing string, block string) string {
	idxStart := strings.Index(existing, StartMarker)
	idxEnd := strings.Index(existing, EndMarker)
	if idxStart >= 0 && idxEnd > idxStart {
		rest := existing[idxEnd:]
		endNL := strings.Index(rest, "\n")
		blockEnd := idxEnd + len(rest)
		if endNL >= 0 {
			blockEnd = idxEnd + endNL + 1
		}
		before := strings.TrimRight(existing[:idxStart], " \t\r\n")
		after := strings.TrimLeft(existing[blockEnd:], " \t\r\n")
		return joinAroundBlock(before, block, after)
	}
	trimmed := strings.TrimRight(existing, " \t\r\n")
	if trimmed == "" {
		return strings.TrimRight(block, "\n") + "\n"
	}
	return trimmed + "\n\n" + strings.TrimRight(block, "\n") + "\n"
}

func joinAroundBlock(before, block, after string) string {
	var b strings.Builder
	if before != "" {
		b.WriteString(before)
		b.WriteString("\n\n")
	}
	b.WriteString(strings.TrimRight(block, "\n"))
	b.WriteByte('\n')
	if after != "" {
		b.WriteByte('\n')
		b.WriteString(after)
	}
	b.WriteByte('\n')
	return b.String()
}
