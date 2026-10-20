package taskpacks

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/localfiles"
)

const maxPackBytes uint64 = 512 << 20
const maxManifestBytes uint64 = 16 << 20
const maxPackEntries = 100000

type ZIPReader struct{}

func NewZIPReader() *ZIPReader { return &ZIPReader{} }

// Read never extracts archive entries into the filesystem.
func (r *ZIPReader) Read(ctx context.Context, filename string) (app.TaskPackData, error) {
	if err := ctx.Err(); err != nil {
		return app.TaskPackData{}, err
	}
	if strings.HasPrefix(filename, "~/") || strings.HasPrefix(filename, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return app.TaskPackData{}, err
		}
		filename = filepath.Join(home, filename[2:])
	}
	info, err := os.Stat(filename)
	if err != nil {
		return app.TaskPackData{}, err
	}
	if !info.Mode().IsRegular() {
		return app.TaskPackData{}, app.ErrInvalidTaskPack
	}
	file, err := localfiles.OpenSource(filename)
	if err != nil {
		return app.TaskPackData{}, err
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil {
		return app.TaskPackData{}, err
	}
	if !info.Mode().IsRegular() {
		return app.TaskPackData{}, app.ErrInvalidTaskPack
	}
	archive, err := zip.NewReader(file, info.Size())
	if err != nil {
		return app.TaskPackData{}, fmt.Errorf("%w: %v", app.ErrInvalidTaskPack, err)
	}
	if len(archive.File) > maxPackEntries {
		return app.TaskPackData{}, app.ErrInvalidTaskPack
	}
	entries := make(map[string]*zip.File)
	var total uint64
	for _, file := range archive.File {
		if err := ctx.Err(); err != nil {
			return app.TaskPackData{}, err
		}
		name := file.Name
		if name == "" || path.Clean(name) != name || strings.HasPrefix(name, "/") || strings.ContainsAny(name, "\\:\x00") || (!strings.HasPrefix(name, "assets/") && name != "manifest.json") || !file.Mode().IsRegular() || entries[name] != nil {
			return app.TaskPackData{}, app.ErrInvalidTaskPack
		}
		if file.UncompressedSize64 > maxPackBytes-total || (name == "manifest.json" && file.UncompressedSize64 > maxManifestBytes) {
			return app.TaskPackData{}, fmt.Errorf("%w: превышен размер пакета (512 МиБ) или манифеста (16 МиБ)", app.ErrInvalidTaskPack)
		}
		total += file.UncompressedSize64
		entries[name] = file
	}
	manifestFile := entries["manifest.json"]
	if manifestFile == nil {
		return app.TaskPackData{}, app.ErrInvalidTaskPack
	}
	data, err := readPackEntry(ctx, manifestFile)
	if err != nil {
		return app.TaskPackData{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Format != Format || manifest.Version != Version {
		return app.TaskPackData{}, app.ErrInvalidTaskPack
	}
	pack := app.TaskPackData{Tasks: manifest.Tasks, Assets: make(map[string][]byte)}
	for _, task := range manifest.Tasks {
		for _, attachment := range task.Attachments {
			if _, ok := pack.Assets[attachment.Path]; ok {
				continue
			}
			file := entries[attachment.Path]
			if file == nil || !strings.HasPrefix(attachment.Path, "assets/") {
				return app.TaskPackData{}, app.ErrInvalidTaskPack
			}
			data, err := readPackEntry(ctx, file)
			if err != nil {
				return app.TaskPackData{}, err
			}
			pack.Assets[attachment.Path] = data
		}
	}
	return pack, ctx.Err()
}

type packContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r packContextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

func readPackEntry(ctx context.Context, file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", app.ErrInvalidTaskPack, err)
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(packContextReader{ctx, reader}, int64(file.UncompressedSize64)+1))
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil || uint64(len(data)) != file.UncompressedSize64 {
		return nil, app.ErrInvalidTaskPack
	}
	return data, nil
}
