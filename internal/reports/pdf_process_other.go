//go:build !unix && !windows

package reports

import (
	"context"
	"os/exec"
	"time"
)

func runPDFCommand(_ context.Context, command *exec.Cmd) error {
	command.WaitDelay = 2 * time.Second
	return command.Run()
}
