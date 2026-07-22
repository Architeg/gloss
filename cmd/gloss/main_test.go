package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const expectedCLIHelp = `Gloss — command glossary

Terminal (no TUI):
  gloss version                print version
  gloss --version              print version
  gloss -v                     print version
  gloss add                    add an entry (prompts)
  gloss list [--tag <tag>]     list entries, optionally filter by tag
  gloss scan                   print scan suggestions (no import)
  gloss edit <command>         edit description/tags (prompts)
  gloss delete <command>       remove an entry
  gloss alias add              add managed alias (stored only; sync separately)
  gloss alias sync             write managed block to shell file (backup if needed)
  gloss alias delete <name>    remove a managed alias

  gloss help                   show this help

Launch TUI:
  gloss
`

func TestEarlyDispatchStatelessOutput(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantOut  string
		wantErr  string
		wantCode int
	}{
		{name: "version", args: []string{"version"}, wantOut: "gloss 0.1.0\n"},
		{name: "long version flag", args: []string{"--version"}, wantOut: "gloss 0.1.0\n"},
		{name: "short version flag", args: []string{"-v"}, wantOut: "gloss 0.1.0\n"},
		{name: "help", args: []string{"help"}, wantOut: expectedCLIHelp},
		{name: "long help flag", args: []string{"--help"}, wantOut: expectedCLIHelp},
		{name: "short help flag", args: []string{"-h"}, wantOut: expectedCLIHelp},
		{name: "unknown", args: []string{"unknown"}, wantErr: "gloss: unknown command \"unknown\" (try gloss help)\n", wantCode: 1},
		{name: "usage", args: []string{"scan", "extra"}, wantErr: "gloss: usage: gloss scan\n", wantCode: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			_, handled, code := earlyDispatch(tt.args, &stdout, &stderr)
			if !handled || code != tt.wantCode {
				t.Fatalf("handled/code = %v/%d, want true/%d", handled, code, tt.wantCode)
			}
			if stdout.String() != tt.wantOut {
				t.Fatalf("stdout = %q, want %q", stdout.String(), tt.wantOut)
			}
			if stderr.String() != tt.wantErr {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tt.wantErr)
			}
		})
	}
}

func TestEarlyDispatchLeavesStatefulInvocationsUnhandled(t *testing.T) {
	for _, args := range [][]string{nil, {"list"}, {"add"}, {"scan"}, {"alias", "sync"}} {
		var stdout, stderr bytes.Buffer
		_, handled, code := earlyDispatch(args, &stdout, &stderr)
		if handled || code != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
			t.Fatalf("earlyDispatch(%q) = handled %v, code %d, stdout %q, stderr %q", args, handled, code, stdout.String(), stderr.String())
		}
	}
}

func TestStatelessSubprocessDoesNotInitializeApplication(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--version"}, {"-v"}, {"help"}, {"--help"}, {"-h"}, {"unknown"}, {"scan", "extra"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			home := filepath.Join(t.TempDir(), "home")
			result := runGlossSubprocess(t, home, args...)
			wantSuccess := args[0] != "unknown" && args[0] != "scan"
			if (result.err == nil) != wantSuccess {
				t.Fatalf("error = %v, stdout = %q, stderr = %q", result.err, result.stdout, result.stderr)
			}
			if _, err := os.Lstat(home); !os.IsNotExist(err) {
				t.Fatalf("stateless invocation created HOME content: %v", err)
			}
		})
	}
}

func TestStatelessCommandsWorkWithUnusableHome(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"help"}} {
		t.Run(args[0], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "not-a-directory")
			if err := os.WriteFile(path, []byte("unchanged"), 0o600); err != nil {
				t.Fatal(err)
			}
			result := runGlossSubprocess(t, path, args...)
			if result.err != nil || result.stderr != "" {
				t.Fatalf("%s result = %#v", args[0], result)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "unchanged" {
				t.Fatalf("unusable HOME changed: %q, %v", data, err)
			}
		})
	}
}

func TestDataCommandStillInitializesApplication(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	result := runGlossSubprocess(t, home, "list")
	if result.err != nil || result.stdout != "No entries in glossary.\n" || result.stderr != "" {
		t.Fatalf("list result = %#v", result)
	}
	for _, path := range []string{
		filepath.Join(home, ".config", "gloss", "config.yaml"),
		filepath.Join(home, ".config", "gloss", "gloss.db"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("stateful command did not create %s: %v", path, err)
		}
	}
}

func TestGlossMainHelperProcess(t *testing.T) {
	if os.Getenv("GLOSS_MAIN_HELPER") != "1" {
		return
	}
	var args []string
	if err := json.Unmarshal([]byte(os.Getenv("GLOSS_MAIN_ARGS")), &args); err != nil {
		os.Exit(99)
	}
	os.Args = append([]string{"gloss"}, args...)
	main()
	os.Exit(0)
}

type subprocessResult struct {
	stdout string
	stderr string
	err    error
}

func runGlossSubprocess(t *testing.T, home string, args ...string) subprocessResult {
	t.Helper()
	encoded, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestGlossMainHelperProcess$")
	cmd.Env = append(os.Environ(),
		"GLOSS_MAIN_HELPER=1",
		"GLOSS_MAIN_ARGS="+string(encoded),
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	return subprocessResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}
