//go:build unix

package attachments

import (
	"os"
	"syscall"
)

func openAttachmentSource(path string) (*os.File, error) {
	// A source replaced by a FIFO between Stat and Open must not block the importer.
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
