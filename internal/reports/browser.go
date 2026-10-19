package reports

import (
	"context"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func OpenInBrowser(ctx context.Context, path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	uriPath := filepath.ToSlash(abs)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	uri := (&url.URL{Scheme: "file", Path: uriPath}).String()
	switch runtime.GOOS {
	case "darwin":
		return exec.CommandContext(ctx, "open", uri).Run()
	case "windows":
		return exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", uri).Run()
	default:
		return exec.CommandContext(ctx, "xdg-open", uri).Run()
	}
}
