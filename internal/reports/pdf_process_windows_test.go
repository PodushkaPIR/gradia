package reports_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/reports"
)

// The test executable acts as a local utility fixture before testing parses Chrome flags.
func init() {
	role := os.Getenv("GRADIA_TEST_PDF_PROCESS")
	if role == "" {
		return
	}
	if role == "child" {
		time.Sleep(1500 * time.Millisecond)
		profile := os.Getenv("GRADIA_TEST_PDF_PROFILE")
		_ = os.MkdirAll(profile, 0700)
		_ = os.WriteFile(filepath.Join(profile, "late-output"), []byte("late"), 0600)
		_ = os.WriteFile(os.Getenv("GRADIA_TEST_PDF_OUTPUT"), []byte("late PDF output"), 0600)
		os.Exit(0)
	}
	var output, profile string
	for _, argument := range os.Args[1:] {
		if strings.HasPrefix(argument, "--print-to-pdf=") {
			output = strings.TrimPrefix(argument, "--print-to-pdf=")
		}
		if strings.HasPrefix(argument, "--user-data-dir=") {
			profile = strings.TrimPrefix(argument, "--user-data-dir=")
		}
	}
	if output == "" || profile == "" {
		os.Exit(8)
	}
	executable, err := os.Executable()
	if err != nil {
		os.Exit(8)
	}
	child := exec.Command(executable)
	child.Env = append(os.Environ(), "GRADIA_TEST_PDF_PROCESS=child", "GRADIA_TEST_PDF_OUTPUT="+output, "GRADIA_TEST_PDF_PROFILE="+profile)
	if err := child.Start(); err != nil {
		os.Exit(8)
	}
	if err := os.WriteFile(os.Getenv("GRADIA_TEST_PDF_JOURNAL"), []byte(profile), 0600); err != nil {
		os.Exit(8)
	}
	if role == "exit" {
		os.Exit(7)
	}
	_ = child.Wait()
	os.Exit(0)
}

func TestPDFUtilityCleansUpWindowsLauncherDescendants(t *testing.T) {
	for _, mode := range []string{"exit", "wait"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			journal := filepath.Join(root, "profile-path")
			t.Setenv("GRADIA_TEST_PDF_PROCESS", mode)
			t.Setenv("GRADIA_TEST_PDF_JOURNAL", journal)
			htmlPath, err := reports.NewHTMLWriter(filepath.Join(root, "reports")).Write(context.Background(), app.LearningReport{StudentName: "Иван", PlanName: "Python"})
			if err != nil {
				t.Fatal(err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if mode == "wait" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, time.Second)
				defer cancel()
			}
			path, exportErr := reports.NewChromiumPDFExporter(executable).Export(ctx, htmlPath)
			if exportErr == nil || path != "" {
				t.Fatalf("expected launcher failure, got %q %v", path, exportErr)
			}
			if mode == "exit" {
				var exitError *exec.ExitError
				if !errors.As(exportErr, &exitError) || exitError.ExitCode() != 7 {
					t.Fatalf("expected launcher exit 7 after starting descendant, got %v", exportErr)
				}
			} else if !errors.Is(exportErr, context.DeadlineExceeded) {
				t.Fatalf("expected cancellation, got %v", exportErr)
			}
			profile, err := os.ReadFile(journal)
			if err != nil {
				t.Fatalf("utility fixture did not start: %v", err)
			}
			time.Sleep(2 * time.Second)
			if _, err := os.Stat(string(profile)); !os.IsNotExist(err) {
				t.Fatalf("profile was recreated by descendant: %v", err)
			}
			files, err := os.ReadDir(filepath.Dir(htmlPath))
			if err != nil || len(files) != 1 {
				t.Fatalf("descendant recreated PDF: %v %v", files, err)
			}
		})
	}
}
