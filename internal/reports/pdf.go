package reports

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"time"

	"github.com/shesh/gradia/internal/desktop"
)

var ErrPDFUtilityNotFound = errors.New("Chromium/Chrome не найден; укажите путь через --pdf-browser или сохраните PDF из браузера")

var pdfTrailer = regexp.MustCompile(`startxref\s+([0-9]+)\s+%%EOF\s*$`)

type ChromiumPDFExporter struct{ executable string }

func NewChromiumPDFExporter(executable string) *ChromiumPDFExporter {
	return &ChromiumPDFExporter{executable: executable}
}

func (e *ChromiumPDFExporter) findExecutable() (string, error) {
	if e.executable != "" {
		path, err := exec.LookPath(e.executable)
		if err != nil {
			return "", fmt.Errorf("%w: %s", ErrPDFUtilityNotFound, e.executable)
		}
		return path, nil
	}
	candidates := []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "chrome", "chrome.exe"}
	if runtime.GOOS == "darwin" {
		candidates = append(candidates, "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "/Applications/Chromium.app/Contents/MacOS/Chromium")
	}
	if runtime.GOOS == "windows" {
		for _, variable := range []string{"ProgramFiles", "ProgramFiles(x86)", "LOCALAPPDATA"} {
			if root := os.Getenv(variable); root != "" {
				candidates = append(candidates, filepath.Join(root, "Google", "Chrome", "Application", "chrome.exe"))
			}
		}
	}
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", ErrPDFUtilityNotFound
}

// Export prints the same autonomous HTML as the browser path, retaining it on failure.
func (e *ChromiumPDFExporter) Export(ctx context.Context, htmlPath string) (path string, err error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	executable, err := e.findExecutable()
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(htmlPath)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("HTML-отчёт не является файлом")
	}
	uri, err := desktop.FileURL(abs)
	if err != nil {
		return "", err
	}
	profile, err := os.MkdirTemp("", "gradia-pdf-profile-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(profile)
	output, err := os.CreateTemp(filepath.Dir(abs), "learning-plan-*.pdf")
	if err != nil {
		return "", err
	}
	pdfPath := output.Name()
	defer func() {
		if err != nil {
			os.Remove(pdfPath)
		}
	}()
	if err := output.Close(); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "--headless", "--disable-gpu", "--disable-dev-shm-usage", "--disable-background-networking", "--no-first-run", "--no-default-browser-check", "--no-pdf-header-footer", "--user-data-dir="+profile, "--print-to-pdf="+pdfPath, uri)
	if err := runPDFCommand(ctx, command); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("экспорт PDF прерван: %w", ctx.Err())
		}
		return "", fmt.Errorf("не удалось экспортировать PDF через Chromium: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	file, err := os.Open(pdfPath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if !completePDF(file) {
		return "", errors.New("утилита не создала корректный PDF")
	}
	return pdfPath, nil
}

// Check completion markers and the cross-reference offset, not just the magic header.
func completePDF(file *os.File) bool {
	var header [5]byte
	if _, err := io.ReadFull(file, header[:]); err != nil || string(header[:]) != "%PDF-" {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	tail := make([]byte, min(info.Size(), 1024))
	if _, err := file.ReadAt(tail, info.Size()-int64(len(tail))); err != nil {
		return false
	}
	match := pdfTrailer.FindSubmatch(tail)
	if match == nil {
		return false
	}
	offset, err := strconv.ParseInt(string(match[1]), 10, 64)
	if err != nil || offset <= 0 || offset >= info.Size() {
		return false
	}
	var crossReference [4]byte
	if _, err := file.ReadAt(crossReference[:], offset); err != nil {
		return false
	}
	// Classic xref tables begin with xref; xref streams begin with an indirect object number.
	return string(crossReference[:]) == "xref" || crossReference[0] >= '0' && crossReference[0] <= '9'
}
