//go:build unix

package localfiles

import (
	"os"
	"syscall"
)

// OpenSource prevents a FIFO from blocking Open on Unix. Callers must verify the
// opened handle is a regular file before reading it.
func OpenSource(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
