package alias

import (
	"fmt"
	"strings"
)

// ValidateAliasName accepts the portable Bash/Zsh identifier grammar
// [A-Za-z_][A-Za-z0-9_]* without trimming or rewriting the name.
func ValidateAliasName(name string) error {
	if name == "" {
		return fmt.Errorf("alias name is required")
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		valid := c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
		if i > 0 {
			valid = valid || c >= '0' && c <= '9'
		}
		if !valid {
			return fmt.Errorf("invalid alias name %q: must match [A-Za-z_][A-Za-z0-9_]*", name)
		}
	}
	return nil
}

// QuoteShellLiteral returns one Bash/Zsh-compatible single-quoted word.
func QuoteShellLiteral(value string) (string, error) {
	if strings.IndexByte(value, 0) >= 0 {
		return "", fmt.Errorf("alias target contains NUL")
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'", nil
}
