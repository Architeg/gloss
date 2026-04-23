package scan

import (
	"bufio"
	"bytes"
	"os"
	"regexp"
	"strings"

	"github.com/valeriybagrintsev/gloss/internal/model"
)

var (
	reFuncParen = regexp.MustCompile(`^\s*function\s+([a-zA-Z_][a-zA-Z0-9_:-]*)\s*\(\s*\)\s*\{`)
	reFuncBrace = regexp.MustCompile(`^\s*function\s+([a-zA-Z_][a-zA-Z0-9_:-]*)\s*\{`)
	reNamedFn   = regexp.MustCompile(`^\s*([a-zA-Z_][a-zA-Z0-9_:-]*)\s*\(\s*\)\s*\{`)
)

// ParseShellFile reads path and extracts alias and simple function suggestions.
func ParseShellFile(path string) ([]model.ScanSuggestion, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseShellBytes(path, data), nil
}

func parseShellBytes(sourcePath string, data []byte) []model.ScanSuggestion {
	var out []model.ScanSuggestion
	sc := bufio.NewScanner(bytes.NewReader(data))
	// Allow long lines in odd configs.
	const max = 1024 * 1024
	buf := make([]byte, max)
	sc.Buffer(buf, max)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if s, ok := parseAliasLine(sourcePath, line); ok {
			out = append(out, s)
			continue
		}
		if s, ok := parseFunctionLine(sourcePath, line); ok {
			out = append(out, s)
		}
	}
	return out
}

func parseAliasLine(sourcePath, line string) (model.ScanSuggestion, bool) {
	if !strings.HasPrefix(line, "alias") {
		return model.ScanSuggestion{}, false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, "alias"))
	// Skip alias -g / alias -s style flag blocks conservatively: consume leading flags.
	for strings.HasPrefix(rest, "-") {
		i := strings.IndexFunc(rest, func(r rune) bool {
			return r == ' ' || r == '\t'
		})
		if i < 0 {
			return model.ScanSuggestion{}, false
		}
		rest = strings.TrimSpace(rest[i:])
	}
	eq := strings.Index(rest, "=")
	if eq <= 0 {
		return model.ScanSuggestion{}, false
	}
	name := strings.TrimSpace(rest[:eq])
	val := strings.TrimSpace(rest[eq+1:])
	if name == "" {
		return model.ScanSuggestion{}, false
	}
	name = stripOuterQuotes(name)
	val = stripShellComment(unquoteShellValue(val))
	return model.ScanSuggestion{
		Command:  name,
		Type:     model.EntryTypeAlias,
		Source:   sourcePath,
		Target:   val,
		Selected: true,
	}, true
}

func parseFunctionLine(sourcePath, line string) (model.ScanSuggestion, bool) {
	var name string
	switch {
	case reFuncParen.MatchString(line):
		name = reFuncParen.FindStringSubmatch(line)[1]
	case reFuncBrace.MatchString(line):
		name = reFuncBrace.FindStringSubmatch(line)[1]
	case reNamedFn.MatchString(line):
		name = reNamedFn.FindStringSubmatch(line)[1]
	default:
		return model.ScanSuggestion{}, false
	}
	return model.ScanSuggestion{
		Command:  name,
		Type:     model.EntryTypeFunction,
		Source:   sourcePath,
		Target:   "shell function",
		Selected: true,
	}, true
}

func stripOuterQuotes(s string) string {
	if len(s) >= 2 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			return s[1 : len(s)-1]
		}
		if s[0] == '\'' && s[len(s)-1] == '\'' {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func unquoteShellValue(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			return unescapeDoubleQuoted(s[1 : len(s)-1])
		}
		if s[0] == '\'' && s[len(s)-1] == '\'' {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func stripShellComment(s string) string {
	inSingle, inDouble := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case c == '"' && !inSingle:
			inDouble = !inDouble
		case c == '#' && !inSingle && !inDouble:
			return strings.TrimSpace(s[:i])
		}
	}
	return s
}

func unescapeDoubleQuoted(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '"', '\\', '$', '`':
				b.WriteByte(s[i+1])
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
