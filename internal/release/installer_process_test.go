package release

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

const (
	installerSubprocessTimeout         = 30 * time.Second
	detachedInstallerSubprocessTimeout = 5 * time.Second
)

func newInstallerTestCommand(t *testing.T, detached bool, name string, args ...string) *exec.Cmd {
	t.Helper()

	timeout := installerSubprocessTimeout
	if detached {
		timeout = detachedInstallerSubprocessTimeout
	}
	return newInstallerTestCommandWithTimeout(t, detached, timeout, name, args...)
}

func newInstallerTestCommandWithTimeout(
	t *testing.T,
	detached bool,
	timeout time.Duration,
	name string,
	args ...string,
) *exec.Cmd {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)

	command := exec.CommandContext(ctx, name, args...)
	command.WaitDelay = time.Second
	if detached {
		detachInstallerTestCommand(command)
		command.Cancel = func() error {
			return killDetachedInstallerTestCommand(command)
		}
	}
	return command
}
