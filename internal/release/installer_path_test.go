package release

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallerPATHPresentDoesNotPromptOrEdit(t *testing.T) {
	home := canonicalTempDir(t)
	directory := filepath.Join(home, "bin")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	output, err := runConfigurePATH(t, pathTestOptions{
		home:      home,
		shell:     "/bin/zsh",
		directory: directory,
		path:      "/usr/bin:" + directory + ":/bin",
		reply:     stringPointer("Y\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output, "Add Gloss to PATH") {
		t.Fatalf("PATH-present run prompted: %q", output)
	}
	if _, err := os.Lstat(filepath.Join(home, ".zshrc")); !os.IsNotExist(err) {
		t.Fatalf("PATH-present run changed shell file: %v", err)
	}
}

func TestInstallerPATHMembershipUsesExactEntries(t *testing.T) {
	home := canonicalTempDir(t)
	directory := filepath.Join(home, "bin")
	output, err := runConfigurePATH(t, pathTestOptions{
		home: home, shell: "/bin/zsh", directory: directory,
		path: directory + "ary:/usr/bin", reply: stringPointer("N\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "Add Gloss to PATH") {
		t.Fatalf("substring PATH entry was treated as exact: %q", output)
	}
}

func TestInstallerAcceptedPATHUpdates(t *testing.T) {
	tests := []struct {
		name    string
		shell   string
		rc      string
		reply   string
		initial string
	}{
		{name: "zsh Enter", shell: "/bin/zsh", rc: ".zshrc", reply: "\n", initial: "# existing zsh"},
		{name: "zsh y", shell: "/usr/local/bin/zsh", rc: ".zshrc", reply: "y\n", initial: "# existing zsh"},
		{name: "zsh Y", shell: "/bin/zsh", rc: ".zshrc", reply: "Y\n", initial: "# existing zsh"},
		{name: "bash y", shell: "/bin/bash", rc: ".bashrc", reply: "y\n", initial: "# existing bash"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := canonicalTempDir(t)
			directory := filepath.Join(home, ".local", "bin")
			if err := os.MkdirAll(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			rc := filepath.Join(home, tt.rc)
			if err := os.WriteFile(rc, []byte(tt.initial), 0o640); err != nil {
				t.Fatal(err)
			}
			output, err := runConfigurePATH(t, pathTestOptions{
				home: home, shell: tt.shell, directory: directory,
				path: "/usr/bin:/bin", reply: stringPointer(tt.reply),
			})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output, "Add Gloss to PATH in ~/"+tt.rc+"? [Y/n]") ||
				!strings.Contains(output, "Run: source ~/"+tt.rc) {
				t.Fatalf("accepted output = %q", output)
			}
			data, err := os.ReadFile(rc)
			if err != nil {
				t.Fatal(err)
			}
			wantLine := `export PATH="$HOME/.local/bin:$PATH"`
			if !strings.HasPrefix(string(data), tt.initial) || strings.Count(string(data), wantLine) != 1 {
				t.Fatalf("shell file = %q", data)
			}
			info, err := os.Stat(rc)
			if err != nil || info.Mode().Perm() != 0o640 {
				t.Fatalf("shell mode = %v, %v", info.Mode(), err)
			}
			assertNoPATHTemps(t, home)
		})
	}
}

func TestInstallerDeclinedPATHUpdates(t *testing.T) {
	for _, reply := range []string{"n\n", "N\n"} {
		t.Run(strings.TrimSpace(reply), func(t *testing.T) {
			home := canonicalTempDir(t)
			rc := filepath.Join(home, ".bashrc")
			original := []byte("# unchanged\n")
			if err := os.WriteFile(rc, original, 0o600); err != nil {
				t.Fatal(err)
			}
			output, err := runConfigurePATH(t, pathTestOptions{
				home: home, shell: "/bin/bash", directory: filepath.Join(home, "bin"),
				path: "/usr/bin:/bin", reply: stringPointer(reply),
			})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output, "Skipped PATH update.") {
				t.Fatalf("decline output = %q", output)
			}
			assertFileBytes(t, rc, original)
			assertNoPATHTemps(t, home)
		})
	}
}

func TestInstallerReadsConfirmationFromTTYNotStdin(t *testing.T) {
	home := canonicalTempDir(t)
	output, err := runConfigurePATH(t, pathTestOptions{
		home: home, shell: "/bin/zsh", directory: filepath.Join(home, "bin"),
		path: "/usr/bin:/bin", reply: stringPointer("Y\n"), stdin: "N\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "Gloss PATH entry added") {
		t.Fatalf("TTY confirmation was not used: %q", output)
	}
	if _, err := os.Stat(filepath.Join(home, ".zshrc")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallerNoninteractiveAndUnknownShellFallback(t *testing.T) {
	tests := []struct {
		name  string
		shell string
		want  string
	}{
		{name: "no terminal", shell: "/bin/zsh", want: "No interactive terminal is available"},
		{name: "unknown shell", shell: "/bin/fish", want: "Could not determine a supported zsh or bash startup file"},
		{name: "missing shell", want: "Could not determine a supported zsh or bash startup file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := canonicalTempDir(t)
			output, err := runConfigurePATH(t, pathTestOptions{
				home: home, shell: tt.shell, directory: filepath.Join(home, ".local", "bin"),
				path: "/usr/bin:/bin",
			})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output, tt.want) ||
				!strings.Contains(output, `export PATH="$HOME/.local/bin:$PATH"`) {
				t.Fatalf("fallback output = %q", output)
			}
			for _, rc := range []string{".zshrc", ".bashrc"} {
				if _, err := os.Lstat(filepath.Join(home, rc)); !os.IsNotExist(err) {
					t.Fatalf("fallback created %s: %v", rc, err)
				}
			}
		})
	}
}

func TestInstallerCreatesMissingShellFileSafelyAndIsIdempotent(t *testing.T) {
	home := canonicalTempDir(t)
	directory := filepath.Join(home, ".local", "bin")
	options := pathTestOptions{
		home: home, shell: "/bin/zsh", directory: directory,
		path: "/usr/bin:/bin", reply: stringPointer("Y\n"),
	}
	if _, err := runConfigurePATH(t, options); err != nil {
		t.Fatal(err)
	}
	rc := filepath.Join(home, ".zshrc")
	info, err := os.Stat(rc)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("new shell file mode = %v, %v", info.Mode(), err)
	}
	first, err := os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	options.reply = nil
	output, err := runConfigurePATH(t, options)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "already contains this PATH entry") {
		t.Fatalf("repeat output = %q", output)
	}
	assertFileBytes(t, rc, first)
	assertNoPATHTemps(t, home)
}

func TestInstallerRecognizesEquivalentExistingPATHLines(t *testing.T) {
	home := canonicalTempDir(t)
	directory := filepath.Join(home, ".local", "bin")
	for _, line := range []string{
		`export PATH="$HOME/.local/bin:$PATH"`,
		"export PATH='" + directory + "':$PATH",
		`export PATH="` + directory + `:$PATH"`,
		"export PATH=" + directory + ":$PATH",
	} {
		t.Run(line, func(t *testing.T) {
			rc := filepath.Join(home, ".zshrc")
			if err := os.WriteFile(rc, []byte(line+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			output, err := runConfigurePATH(t, pathTestOptions{
				home: home, shell: "/bin/zsh", directory: directory,
				path: "/usr/bin:/bin", reply: stringPointer("Y\n"),
			})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output, "already contains this PATH entry") {
				t.Fatalf("equivalent line was not recognized: %q", output)
			}
			data, err := os.ReadFile(rc)
			if err != nil || string(data) != line+"\n" {
				t.Fatalf("equivalent file changed: %q, %v", data, err)
			}
		})
	}
}

func TestInstallerQuotesCustomPATHLiterally(t *testing.T) {
	home := canonicalTempDir(t)
	directory := filepath.Join(home, "bin $(touch sentinel);'quoted`tick`\\slash")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := runConfigurePATH(t, pathTestOptions{
		home: home, shell: "/bin/bash", directory: directory,
		path: "/usr/bin:/bin", reply: stringPointer("Y\n"),
	}); err != nil {
		t.Fatal(err)
	}
	rc := filepath.Join(home, ".bashrc")
	data, err := os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `export PATH="`+directory) {
		t.Fatalf("unsafe double-quoted path = %q", data)
	}

	command := exec.Command("bash", "--noprofile", "--norc", "-c", `source "$RC"; printf '%s\n' "${PATH%%:*}"`)
	command.Dir = home
	command.Env = mergeInstallerEnvironment(os.Environ(),
		"RC="+rc,
		"HOME="+home,
		"PATH=/usr/bin:/bin",
	)
	output, err := command.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != directory {
		t.Fatalf("quoted PATH result = %q, %v; file=%q", output, err, data)
	}
	if _, err := os.Lstat(filepath.Join(home, "sentinel")); !os.IsNotExist(err) {
		t.Fatalf("shell-sensitive path executed a command: %v", err)
	}
}

func TestInstallerRejectsUnsafeShellTargetsWithoutFailing(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		home := canonicalTempDir(t)
		target := filepath.Join(home, "real-zshrc")
		original := []byte("# real\n")
		if err := os.WriteFile(target, original, 0o600); err != nil {
			t.Fatal(err)
		}
		rc := filepath.Join(home, ".zshrc")
		if err := os.Symlink(target, rc); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		output, err := runConfigurePATH(t, pathTestOptions{
			home: home, shell: "/bin/zsh", directory: filepath.Join(home, "bin"),
			path: "/usr/bin:/bin", reply: stringPointer("Y\n"),
		})
		if err != nil || !strings.Contains(output, "refusing to modify symlinked") {
			t.Fatalf("symlink result = %v, %q", err, output)
		}
		assertFileBytes(t, target, original)
	})

	t.Run("nonregular", func(t *testing.T) {
		home := canonicalTempDir(t)
		rc := filepath.Join(home, ".bashrc")
		if err := os.Mkdir(rc, 0o700); err != nil {
			t.Fatal(err)
		}
		output, err := runConfigurePATH(t, pathTestOptions{
			home: home, shell: "/bin/bash", directory: filepath.Join(home, "bin"),
			path: "/usr/bin:/bin", reply: stringPointer("Y\n"),
		})
		if err != nil || !strings.Contains(output, "refusing to modify nonregular") {
			t.Fatalf("nonregular result = %v, %q", err, output)
		}
		info, statErr := os.Stat(rc)
		if statErr != nil || !info.IsDir() {
			t.Fatalf("nonregular target changed: %v, %v", info, statErr)
		}
	})
}

func TestInstallerShellEditFailuresPreserveOriginalAndCleanTemps(t *testing.T) {
	for _, failure := range []string{"staging", "write", "revalidation", "replacement"} {
		t.Run(failure, func(t *testing.T) {
			home := canonicalTempDir(t)
			rc := filepath.Join(home, ".zshrc")
			original := []byte("# original\n")
			if err := os.WriteFile(rc, original, 0o640); err != nil {
				t.Fatal(err)
			}
			root := repositoryRoot(t)
			expression := `
source "$INSTALL_SCRIPT"
staged_shell_file=""
trap cleanup EXIT
`
			switch failure {
			case "staging":
				expression += `mktemp() { return 1; }` + "\n"
			case "write":
				expression += `cp() { return 1; }` + "\n"
			case "revalidation":
				expression += `
file_identity() {
  if [[ -e "$IDENTITY_MARKER" ]]; then
    printf 'changed\n'
  else
    : > "$IDENTITY_MARKER"
    printf 'original\n'
  fi
}
`
			case "replacement":
				expression += `mv() { return 1; }` + "\n"
			}
			expression += `
if append_path_line_atomically "$RC" 'export PATH="$HOME/bin:$PATH"'; then
  exit 90
fi
`
			command := exec.Command("bash", "-c", expression)
			command.Env = mergeInstallerEnvironment(os.Environ(),
				"INSTALL_SCRIPT="+filepath.Join(root, "scripts", "install.sh"),
				"HOME="+home,
				"RC="+rc,
				"IDENTITY_MARKER="+filepath.Join(home, "identity-read"),
			)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("failure injection command = %v, %q", err, output)
			}
			assertFileBytes(t, rc, original)
			info, err := os.Stat(rc)
			if err != nil || info.Mode().Perm() != 0o640 {
				t.Fatalf("mode after failure = %v, %v", info.Mode(), err)
			}
			assertNoPATHTemps(t, home)
		})
	}
}

func TestInstallerCompletesWhenShellEditIsUnsafe(t *testing.T) {
	fixture := validInstallerFixture(t)
	server := newInstallerServer(t, fixture)
	home := canonicalTempDir(t)
	realRC := filepath.Join(home, "real-zshrc")
	if err := os.WriteFile(realRC, []byte("# unchanged\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realRC, filepath.Join(home, ".zshrc")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	reply := filepath.Join(home, "reply")
	if err := os.WriteFile(reply, []byte("Y\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := runInstallerIntegrationWithOptions(
		t, server.URL, "v0.1.1", fixture, false, nil,
		[]string{"HOME=" + home, "SHELL=/bin/zsh", "GLOSS_TEST_TTY=" + reply},
		"", "", nil,
	)
	if result.err != nil || !strings.Contains(result.output, "refusing to modify symlinked") {
		t.Fatalf("unsafe shell edit changed install result: %v\n%s", result.err, result.output)
	}
	if info, err := os.Stat(result.target); err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("installed executable = %v, %v", info, err)
	}
	assertFileBytes(t, realRC, []byte("# unchanged\n"))
}

func TestInstallerDoesNotUsePrivilegeOrDynamicShellCommands(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repositoryRoot(t), "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, forbidden := range []string{"sudo", "\neval ", "\nsu ", "bash -c", "sh -c"} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("installer contains forbidden command %q", forbidden)
		}
	}
}

type pathTestOptions struct {
	home      string
	shell     string
	directory string
	path      string
	reply     *string
	stdin     string
}

func runConfigurePATH(t *testing.T, options pathTestOptions) (string, error) {
	t.Helper()
	root := repositoryRoot(t)
	environment := []string{
		"INSTALL_SCRIPT=" + filepath.Join(root, "scripts", "install.sh"),
		"GLOSS_INSTALL_TESTING=1",
		"HOME=" + options.home,
		"SHELL=" + options.shell,
		"PATH=" + options.path,
		"DIRECTORY=" + options.directory,
	}
	if options.reply != nil {
		reply := filepath.Join(options.home, "tty-input")
		if err := os.WriteFile(reply, []byte(*options.reply), 0o600); err != nil {
			t.Fatal(err)
		}
		environment = append(environment, "GLOSS_TEST_TTY="+reply)
	}
	command := exec.Command("bash", "-c", `
source "$INSTALL_SCRIPT"
trap cleanup EXIT
configure_path "$DIRECTORY"
`)
	command.Env = mergeInstallerEnvironment(os.Environ(), environment...)
	command.Stdin = strings.NewReader(options.stdin)
	output, err := command.CombinedOutput()
	return string(output), err
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func canonicalTempDir(t *testing.T) string {
	t.Helper()
	path, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func assertFileBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("%s = %q, %v; want %q", path, got, err, want)
	}
}

func assertNoPATHTemps(t *testing.T, home string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(home, ".gloss-path.*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("PATH temporary files = %v, %v", matches, err)
	}
}

func stringPointer(value string) *string {
	return &value
}
