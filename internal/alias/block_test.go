package alias

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Architeg/gloss/internal/model"
)

func TestValidateAliasName(t *testing.T) {
	for _, name := range []string{"g", "gs", "_private", "build2", "GLOSS_TEST"} {
		t.Run("valid_"+name, func(t *testing.T) {
			if err := ValidateAliasName(name); err != nil {
				t.Fatalf("ValidateAliasName(%q): %v", name, err)
			}
		})
	}
	invalid := []string{
		"", "2build", "has space", "git-status", "git:status", "path/name",
		"name=value", "'quote'", "\"quote\"", "$name", "`name`", "name;echo",
		"name|echo", "name&echo", "name>file", "line\nname", "tab\tname", "café",
	}
	for _, name := range invalid {
		t.Run("invalid_"+name, func(t *testing.T) {
			if err := ValidateAliasName(name); err == nil {
				t.Fatalf("ValidateAliasName(%q) unexpectedly succeeded", name)
			}
		})
	}
}

func TestQuoteShellLiteral(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "empty", value: "", want: "''"},
		{name: "ordinary", value: "git status", want: "'git status'"},
		{name: "spaces and tabs", value: "printf\t spaced", want: "'printf\t spaced'"},
		{name: "single quote", value: "echo '$HOME'", want: "'echo '\\''$HOME'\\'''"},
		{name: "adjacent single quotes", value: "a''b", want: "'a'\\'''\\''b'"},
		{name: "double quotes", value: `printf "%s"`, want: `'printf "%s"'`},
		{name: "parameter", value: `$HOME ${HOME}`, want: `'$HOME ${HOME}'`},
		{name: "substitution", value: `$(touch sentinel)`, want: `'$(touch sentinel)'`},
		{name: "backticks", value: "`touch sentinel`", want: "'`touch sentinel`'"},
		{name: "backslashes", value: `printf \\ path`, want: `'printf \\ path'`},
		{name: "operators", value: `a; b | c && d > out < in`, want: `'a; b | c && d > out < in'`},
		{name: "glob and bang", value: `echo * ? [a] !`, want: `'echo * ? [a] !'`},
		{name: "unicode", value: `echo Привет`, want: `'echo Привет'`},
		{name: "line feed", value: "echo one\necho two", want: "'echo one\necho two'"},
		{name: "carriage return", value: "echo one\recho two", want: "'echo one\recho two'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := QuoteShellLiteral(tt.value)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("QuoteShellLiteral(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
	if got, err := QuoteShellLiteral("bad\x00target"); err == nil || got != "" {
		t.Fatalf("NUL result = %q, %v", got, err)
	}
}

func TestRenderManagedBlockUsesValidatedLiteralSource(t *testing.T) {
	entries := []model.Entry{
		{Command: "ignored", Type: model.EntryTypeManual},
		{Command: "gs", Type: model.EntryTypeAlias, ManagedAlias: true, Target: "echo '$HOME'; $(touch sentinel); `touch other`; echo \\ !\nnext\rline"},
	}
	block, err := RenderManagedBlock(entries)
	if err != nil {
		t.Fatal(err)
	}
	want := StartMarker + "\nalias gs='echo '\\''$HOME'\\''; $(touch sentinel); `touch other`; echo \\ !\nnext\rline'\n" + EndMarker
	if block != want {
		t.Fatalf("block = %q, want %q", block, want)
	}

	for _, entry := range []model.Entry{
		{Command: "bad-name", Type: model.EntryTypeAlias, ManagedAlias: true, Target: "true"},
		{Command: "good", Type: model.EntryTypeAlias, ManagedAlias: true, Target: "bad\x00target"},
	} {
		if got, err := RenderManagedBlock([]model.Entry{entry}); err == nil || got != "" {
			t.Fatalf("unsafe render = %q, %v", got, err)
		}
	}
	if got, err := RenderManagedBlock([]model.Entry{
		{Command: "good", Type: model.EntryTypeAlias, ManagedAlias: true, Target: "true"},
		{Command: "bad-name", Type: model.EntryTypeAlias, ManagedAlias: true, Target: "true"},
	}); err == nil || got != "" {
		t.Fatalf("partially rendered unsafe block = %q, %v", got, err)
	}
}

func TestSyncValidationFailureDoesNotTouchShellFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".zshrc")
	original := []byte("export KEEP=1\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []model.Entry{
		{Command: "bad-name", Type: model.EntryTypeAlias, ManagedAlias: true, Target: "true"},
		{Command: "good", Type: model.EntryTypeAlias, ManagedAlias: true, Target: "bad\x00target"},
	} {
		if _, err := Sync(path, []model.Entry{entry}, 5); err == nil {
			t.Fatalf("Sync(%q) unexpectedly succeeded", entry.Command)
		}
		data, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(data, original) {
			t.Fatalf("shell file changed: %q, %v", data, err)
		}
		backups, err := filepath.Glob(path + ".gloss.bak-*")
		if err != nil || len(backups) != 0 {
			t.Fatalf("validation failure created backups: %v, %v", backups, err)
		}
	}
}

func TestSyncPreservesExternalContentAndShellsSourceLiterally(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			shellPath, err := exec.LookPath(shell)
			if err != nil {
				t.Skipf("%s is unavailable: %v", shell, err)
			}
			dir := t.TempDir()
			file := filepath.Join(dir, ".shellrc")
			sentinelSub := filepath.Join(dir, "substitution-ran")
			sentinelTick := filepath.Join(dir, "backtick-ran")
			external := "export GLOSS_EXTERNAL=kept\n"
			if err := os.WriteFile(file, []byte(external), 0o600); err != nil {
				t.Fatal(err)
			}
			target := "echo $HOME ${HOME}; $(touch " + sentinelSub + "); `touch " + sentinelTick + "`; echo 'quote' \\ !\nnext\rline"
			_, err = Sync(file, []model.Entry{{
				Command: "gloss_safe_test", Type: model.EntryTypeAlias, ManagedAlias: true, Target: target,
			}}, 5)
			if err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(content), external) || !strings.Contains(string(content), StartMarker) {
				t.Fatalf("external content or managed block missing: %q", content)
			}

			cmd := exec.Command(shellPath, "-c", `source "$1"; alias gloss_safe_test`, "gloss-alias-test", file)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("source failed: %v; stdout=%q stderr=%q", err, stdout.String(), stderr.String())
			}
			definition := stdout.String()
			for _, literal := range []string{"$HOME", "${HOME}", "$(touch", "`touch", "quote", "\\", "!", "next", "line"} {
				if !strings.Contains(definition, literal) {
					t.Fatalf("alias definition %q lost literal %q", definition, literal)
				}
			}
			for _, sentinel := range []string{sentinelSub, sentinelTick} {
				if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
					t.Fatalf("sourcing executed alias target and created %s", sentinel)
				}
			}
		})
	}
}
