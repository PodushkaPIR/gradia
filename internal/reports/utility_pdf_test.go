package reports_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
	"github.com/shesh/gradia/internal/reports"
)

func TestLocalUtilityExportsPDFWithoutChangingHTML(t *testing.T) {
	component := "отчёты # spaces"
	if runtime.GOOS != "windows" {
		component += "?"
	}
	dir := filepath.Join(t.TempDir(), component)
	if root := os.Getenv("GRADIA_PDF_TEST_ARTIFACTS"); root != "" {
		dir = filepath.Join(root, "utility")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	htmlPath, err := reports.NewHTMLWriter(dir).Write(ctx, app.LearningReport{
		StudentName: "Иван Иванов", PlanName: "Индивидуальный Python", Description: "Локальный PDF-экспорт выбранного плана.",
		Summary: domain.ProgressSummary{Learning: 1, Mastered: 1},
		Topics:  []app.LearningReportTopic{{Name: "Циклы", Status: domain.ProgressLearning}, {Name: "Функции", Status: domain.ProgressMastered, Revision: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	pdfPath, err := reports.NewChromiumPDFExporter("").Export(ctx, htmlPath)
	if errors.Is(err, reports.ErrPDFUtilityNotFound) {
		t.Skip("Chromium/Chrome is not installed")
	}
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(pdfPath) != filepath.Dir(htmlPath) || filepath.Ext(pdfPath) != ".pdf" {
		t.Fatalf("wrong PDF path %q", pdfPath)
	}
	pdf, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") || len(pdf) < 1500 {
		t.Fatalf("invalid PDF: %d bytes", len(pdf))
	}
	if after, err := os.ReadFile(htmlPath); err != nil || string(after) != string(before) {
		t.Fatalf("export modified source HTML: %v", err)
	}
	t.Logf("PDF: %s (%d bytes)", pdfPath, len(pdf))
}

func TestPDFUtilityFailuresKeepHTMLAndRemovePartialPDF(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX executable fixture")
	}
	for _, test := range []struct {
		name, body       string
		missing, timeout bool
		lateOutput       bool
	}{
		{name: "missing executable", missing: true},
		{name: "failed command", body: "printf 'partial' > \"$output\"\nexit 7\n"},
		{name: "invalid output", body: "printf 'not a PDF' > \"$output\"\n"},
		{name: "truncated PDF", body: "printf '%s\\n' '%PDF-1.4' > \"$output\"\n"},
		{name: "missing cross reference", body: "printf '%s\\n' '%PDF-1.4' '%%EOF' > \"$output\"\n"},
		{name: "no output", body: "exit 0\n"},
		{name: "canceled command", body: "exec sleep 30\n", timeout: true},
		{name: "canceled launcher with descendant", body: "(sleep 0.3; printf 'late output' > \"$output\") &\nwait\n", timeout: true, lateOutput: true},
		{name: "failed launcher with descendant", body: "(sleep 0.3; printf 'late output' > \"$output\") &\nexit 7\n", lateOutput: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "отчёты #? spaces")
			htmlPath, err := reports.NewHTMLWriter(dir).Write(context.Background(), app.LearningReport{StudentName: "Иван", PlanName: "Python"})
			if err != nil {
				t.Fatal(err)
			}
			original, err := os.ReadFile(htmlPath)
			if err != nil {
				t.Fatal(err)
			}
			previousPDF := filepath.Join(dir, "previous.pdf")
			if err := os.WriteFile(previousPDF, []byte("preserve existing PDF"), 0600); err != nil {
				t.Fatal(err)
			}
			executable := filepath.Join(t.TempDir(), "fake Chromium")
			if !test.missing {
				program := "#!/bin/sh\nfor argument in \"$@\"; do\n case \"$argument\" in\n --print-to-pdf=*) output=${argument#--print-to-pdf=} ;;\n esac\ndone\n" + test.body
				if err := os.WriteFile(executable, []byte(program), 0700); err != nil {
					t.Fatal(err)
				}
			}
			ctx := context.Background()
			if test.timeout {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 150*time.Millisecond)
				defer cancel()
			}
			pdfPath, err := reports.NewChromiumPDFExporter(executable).Export(ctx, htmlPath)
			if err == nil || pdfPath != "" {
				t.Fatalf("expected failure without a published PDF path, got %q %v", pdfPath, err)
			}
			if test.missing && !errors.Is(err, reports.ErrPDFUtilityNotFound) {
				t.Fatalf("unclear missing utility error: %v", err)
			}
			if test.timeout && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("cancellation not propagated: %v", err)
			}
			if test.lateOutput {
				time.Sleep(500 * time.Millisecond)
			}
			if html, err := os.ReadFile(htmlPath); err != nil || string(html) != string(original) {
				t.Fatalf("HTML changed after export failure: %v", err)
			}
			if previous, err := os.ReadFile(previousPDF); err != nil || string(previous) != "preserve existing PDF" {
				t.Fatalf("previous PDF changed: %v", err)
			}
			files, err := os.ReadDir(dir)
			if err != nil || len(files) != 2 {
				t.Fatalf("partial output not removed: %v %v", files, err)
			}
		})
	}
}
