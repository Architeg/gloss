package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallerOutputColorPolicy(t *testing.T) {
	t.Run("simulated terminal", func(t *testing.T) {
		output, err := runInstallerFunction(t, `
stdout_is_terminal() { return 0; }
unset NO_COLOR
TERM=xterm-256color
initialize_output_styles
print_heading "Heading"
print_success "Success"
print_warning "Warning"
print_command "gloss version"
if fail "failure"; then exit 90; fi
`)
		if err != nil {
			t.Fatal(err)
		}
		for _, sequence := range []string{"\x1b[1m", "\x1b[36m", "\x1b[32m", "\x1b[33m", "\x1b[31m"} {
			if !strings.Contains(output, sequence) {
				t.Fatalf("styled output %q does not contain %q", output, sequence)
			}
		}
	})

	for _, tt := range []struct {
		name       string
		expression string
	}{
		{
			name: "NO_COLOR",
			expression: `
stdout_is_terminal() { return 0; }
NO_COLOR=
TERM=xterm-256color
initialize_output_styles
print_success "Success"
`,
		},
		{
			name: "dumb terminal",
			expression: `
stdout_is_terminal() { return 0; }
unset NO_COLOR
TERM=dumb
initialize_output_styles
print_success "Success"
`,
		},
		{
			name:       "redirected stdout",
			expression: `print_success "Success"`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			output, err := runInstallerFunction(t, tt.expression)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(output, "\x1b[") {
				t.Fatalf("plain output contains ANSI styling: %q", output)
			}
			if output != "✓ Success\n" {
				t.Fatalf("plain output = %q", output)
			}
		})
	}
}

func TestInstallerErrorIsReadableWithoutColor(t *testing.T) {
	output, err := runInstallerFunction(t, `
NO_COLOR=1
initialize_output_styles
if fail "example failure"; then exit 90; fi
`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output, "\x1b[") || output != "✗ gloss installer: example failure\n" {
		t.Fatalf("plain error output = %q", output)
	}
}

func TestInstallerPlatformDisplayLabels(t *testing.T) {
	tests := []struct {
		system string
		arch   string
		want   string
	}{
		{system: "darwin", arch: "amd64", want: "macOS (Intel)"},
		{system: "darwin", arch: "arm64", want: "macOS (Apple Silicon)"},
		{system: "linux", arch: "amd64", want: "Linux (x86_64)"},
		{system: "linux", arch: "arm64", want: "Linux (ARM64)"},
	}
	for _, tt := range tests {
		t.Run(tt.system+"/"+tt.arch, func(t *testing.T) {
			output, err := runInstallerFunction(
				t,
				`display_platform "$SYSTEM" "$ARCH"`,
				"SYSTEM="+tt.system,
				"ARCH="+tt.arch,
			)
			if err != nil || strings.TrimSpace(output) != tt.want {
				t.Fatalf("display platform = %q, %v; want %q", output, err, tt.want)
			}
		})
	}
}

func TestSourcingInstallerDoesNotInvokeMain(t *testing.T) {
	output, err := runInstallerFunction(t, `printf '%s\n' sourced`)
	if err != nil || output != "sourced\n" {
		t.Fatalf("sourced installer output = %q, %v", output, err)
	}
}

func TestInstallerRunsWhenPipedToBash(t *testing.T) {
	fixture := validInstallerFixture(t)
	server := newInstallerServer(t, fixture)
	replyDir := canonicalTempDir(t)
	reply := filepath.Join(replyDir, "reply")
	if err := os.WriteFile(reply, []byte("Y\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result := runInstallerIntegrationWithOptions(
		t, server.URL, "v0.1.1", fixture, false, nil,
		[]string{"SHELL=/bin/zsh", "GLOSS_TEST_TTY=" + reply},
		"", "", nil, true,
	)
	if result.err != nil {
		t.Fatalf("stdin installer failed: %v\n%s", result.err, result.output)
	}
	if !strings.Contains(result.output, "? Add this automatically? [Y/n]") ||
		!strings.Contains(result.output, "✓ PATH updated") {
		t.Fatalf("stdin installer did not complete interactive PATH setup: %q", result.output)
	}
	if _, err := os.Stat(filepath.Join(result.root, "home", ".zshrc")); err != nil {
		t.Fatalf("stdin installer did not update isolated shell file: %v", err)
	}
	if info, err := os.Stat(result.target); err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("stdin installer target = %v, %v", info, err)
	}
}
