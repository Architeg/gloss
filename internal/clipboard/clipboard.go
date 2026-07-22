// Package clipboard writes text to the host system clipboard without invoking a shell.
package clipboard

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Writer accepts text exactly as it should appear on the clipboard.
type Writer interface {
	WriteText(string) error
}

// System writes through the first supported clipboard program available.
type System struct{}

// WriteText writes text verbatim to the platform clipboard.
func (System) WriteText(value string) error {
	name, args, err := provider()
	if err != nil {
		return err
	}
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(value)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return fmt.Errorf("clipboard provider %s: %w", name, err)
		}
		return fmt.Errorf("clipboard provider %s: %w: %s", name, err, message)
	}
	return nil
}

func provider() (string, []string, error) {
	return platformProvider(runtime.GOOS, exec.LookPath)
}

func platformProvider(goos string, lookPath func(string) (string, error)) (string, []string, error) {
	switch goos {
	case "darwin":
		return findProvider([]providerSpec{{name: "pbcopy"}}, lookPath)
	case "linux":
		return findProvider([]providerSpec{
			{name: "wl-copy"},
			{name: "xclip", args: []string{"-selection", "clipboard"}},
			{name: "xsel", args: []string{"--clipboard", "--input"}},
		}, lookPath)
	default:
		return "", nil, fmt.Errorf("clipboard is unsupported on %s", goos)
	}
}

type providerSpec struct {
	name string
	args []string
}

func findProvider(specs []providerSpec, lookPath func(string) (string, error)) (string, []string, error) {
	for _, spec := range specs {
		path, err := lookPath(spec.name)
		if err == nil {
			return path, spec.args, nil
		}
	}
	return "", nil, fmt.Errorf("no supported clipboard provider found")
}
