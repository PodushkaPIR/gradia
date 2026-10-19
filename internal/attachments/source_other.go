//go:build !unix

package attachments

import "os"

func openAttachmentSource(path string) (*os.File, error) { return os.Open(path) }
