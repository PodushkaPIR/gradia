package desktop

import (
	"context"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func FileURL(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	uriPath := filepath.ToSlash(abs)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	return (&url.URL{Scheme: "file", Path: uriPath}).String(), nil
}

func OpenFile(ctx context.Context, path string) error {
	uri, err := FileURL(path)
	if err != nil {
		return err
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.CommandContext(ctx, "open", uri).Run()
	case "windows":
		return exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", uri).Run()
	default:
		return exec.CommandContext(ctx, "xdg-open", uri).Run()
	}
}
