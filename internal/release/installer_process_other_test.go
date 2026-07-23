//go:build !darwin && !linux

package release

import (
	"os"
	"os/exec"
)

func detachInstallerTestCommand(_ *exec.Cmd) {}

func killDetachedInstallerTestCommand(command *exec.Cmd) error {
	if command.Process == nil {
		return os.ErrProcessDone
	}
	return command.Process.Kill()
}
