//go:build unix

package reports

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func runPDFCommand(_ context.Context, command *exec.Cmd) error {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	killGroup := func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	command.Cancel = killGroup
	command.WaitDelay = 2 * time.Second
	// Also terminate leftover children of a launcher that exits before its children.
	defer func() { _ = killGroup() }()
	return command.Run()
}
