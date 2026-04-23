// Package openurl launches a URL in the system default browser.
package openurl

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Open starts the platform default handler for url (non-blocking).
func Open(url string) error {
	if url == "" {
		return fmt.Errorf("empty URL")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
