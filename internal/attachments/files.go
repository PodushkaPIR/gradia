package attachments

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type LocalFiles struct{ directory string }

func NewLocalFiles(directory string) *LocalFiles { return &LocalFiles{directory: directory} }

func (f *LocalFiles) Read(ctx context.Context, path string) (domain.AttachmentFile, error) {
	if err := ctx.Err(); err != nil {
		return domain.AttachmentFile{}, err
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return domain.AttachmentFile{}, err
		}
		path = filepath.Join(home, path[2:])
	}
	type result struct {
		file domain.AttachmentFile
		err  error
	}
	done := make(chan result, 1)
	go func() { file, err := readRegularAttachment(ctx, path); done <- result{file: file, err: err} }()
	select {
	case <-ctx.Done():
		return domain.AttachmentFile{}, ctx.Err()
	case result := <-done:
		if err := ctx.Err(); err != nil {
			return domain.AttachmentFile{}, err
		}
		return result.file, result.err
	}
}

func readRegularAttachment(ctx context.Context, path string) (domain.AttachmentFile, error) {
	if err := ctx.Err(); err != nil {
		return domain.AttachmentFile{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return domain.AttachmentFile{}, err
	}
	if !info.Mode().IsRegular() {
		return domain.AttachmentFile{}, app.ErrAttachmentRegularFileRequired
	}
	if err := ctx.Err(); err != nil {
		return domain.AttachmentFile{}, err
	}
	file, err := openAttachmentSource(path)
	if err != nil {
		return domain.AttachmentFile{}, err
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil {
		return domain.AttachmentFile{}, err
	}
	if !info.Mode().IsRegular() || info.Size() < 0 {
		return domain.AttachmentFile{}, app.ErrAttachmentRegularFileRequired
	}
	// Read only the initial size; a growing file cannot turn this into an unbounded read.
	remaining := info.Size()
	buffer := make([]byte, 64*1024)
	var data bytes.Buffer
	for remaining > 0 {
		if err := ctx.Err(); err != nil {
			return domain.AttachmentFile{}, err
		}
		count := min(remaining, int64(len(buffer)))
		if _, err := io.ReadFull(file, buffer[:count]); err != nil {
			return domain.AttachmentFile{}, err
		}
		data.Write(buffer[:count])
		remaining -= count
	}
	return domain.AttachmentFile{Name: filepath.Base(path), Data: data.Bytes()}, nil
}

func (f *LocalFiles) Write(ctx context.Context, file domain.AttachmentFile) (path string, err error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	name := filepath.Base(file.Name)
	if name == "." || name == ".." || name == string(filepath.Separator) || name == "" {
		return "", errors.New("некорректное имя вложения")
	}
	root, err := filepath.Abs(f.directory)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	directory, err := os.MkdirTemp(root, "homework-file-")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(directory)
		}
	}()
	path = filepath.Join(directory, name)
	if err := os.WriteFile(path, file.Data, 0600); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return path, nil
}
