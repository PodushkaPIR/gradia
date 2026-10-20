//go:build !unix

package localfiles

import "os"

func OpenSource(path string) (*os.File, error) { return os.Open(path) }
