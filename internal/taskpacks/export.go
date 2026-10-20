package taskpacks

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

type ZIPWriter struct{ directory string }

func NewZIPWriter(directory string) *ZIPWriter { return &ZIPWriter{directory: directory} }

func (w *ZIPWriter) Write(ctx context.Context, source app.TaskPackSource, load func(context.Context, domain.TaskBankAttachment) ([]byte, error)) (path string, err error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	root, err := filepath.Abs(w.directory)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(root, "task-pack-*.zip")
	if err != nil {
		return "", err
	}
	defer func() {
		file.Close()
		if err != nil {
			os.Remove(file.Name())
		}
	}()
	archive := zip.NewWriter(file)
	defer archive.Close()
	manifest := Manifest{Format: Format, Version: Version}
	written := make(map[string]bool)
	for _, item := range source.Tasks {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		task := Task{ID: item.Key, Content: item.Task.Content, Subject: item.Task.Subject, Topics: slices.Clone(item.Task.Topics), Difficulty: item.Task.Difficulty, Collections: append([]string{}, item.Collections...), Attachments: make([]Attachment, 0, len(item.Attachments))}
		slices.Sort(task.Collections)
		for _, metadata := range item.Attachments {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			data, err := load(ctx, metadata)
			if err != nil {
				return "", err
			}
			if int64(len(data)) != metadata.Size {
				return "", errors.New("размер вложения изменился во время экспорта")
			}
			hash := sha256.Sum256(data)
			digest := hex.EncodeToString(hash[:])
			assetPath := "assets/" + digest + assetExtension(metadata.MediaType)
			if !written[assetPath] {
				entry, err := archive.Create(assetPath)
				if err != nil {
					return "", err
				}
				if _, err := entry.Write(data); err != nil {
					return "", err
				}
				written[assetPath] = true
			}
			task.Attachments = append(task.Attachments, Attachment{Name: metadata.Name, MediaType: metadata.MediaType, Pages: metadata.Pages, Path: assetPath, SHA256: digest, Size: metadata.Size})
		}
		task.SHA256 = TaskFingerprint(task)
		manifest.Tasks = append(manifest.Tasks, task)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	entry, err := archive.Create("manifest.json")
	if err != nil {
		return "", err
	}
	encoder := json.NewEncoder(entry)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		return "", err
	}
	if err := archive.Close(); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return file.Name(), nil
}

func assetExtension(mediaType string) string {
	switch mediaType {
	case "application/pdf":
		return ".pdf"
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	case "image/tiff":
		return ".tiff"
	case "image/x-icon":
		return ".ico"
	default:
		return ".bin"
	}
}
