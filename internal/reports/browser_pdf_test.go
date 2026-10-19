package reports_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
	"github.com/shesh/gradia/internal/reports"
)

func TestBrowserPrintsLearningReportToPDF(t *testing.T) {
	var browser string
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome"} {
		if path, err := exec.LookPath(name); err == nil {
			browser = path
			break
		}
	}
	if browser == "" {
		t.Skip("Chromium is not installed; HTML artifact tests still cover report contents")
	}
	short := app.LearningReport{
		StudentName: "Иван Иванов", PlanName: "Индивидуальный Python", Description: "Цель: освоить циклы и функции.",
		Summary: domain.ProgressSummary{Learning: 1, Mastered: 1},
		Topics: []app.LearningReportTopic{
			{Name: "Основы", HasChildren: true, Summary: domain.ProgressSummary{Learning: 1, Mastered: 1}},
			{Name: "Циклы", Depth: 1, Status: domain.ProgressLearning},
			{Name: "Функции", Depth: 1, Status: domain.ProgressMastered, Revision: true},
		},
	}
	long := app.LearningReport{StudentName: "Анна Петрова", PlanName: "Подробный учебный план", Description: "Многостраничный план с вложенными темами и разными состояниями прогресса.", Summary: domain.ProgressSummary{NotStarted: 6, Learning: 6, NeedsPractice: 6, Mastered: 6}}
	statuses := []domain.ProgressStatus{domain.ProgressNotStarted, domain.ProgressLearning, domain.ProgressNeedsPractice, domain.ProgressMastered}
	long.Topics = append(long.Topics, app.LearningReportTopic{Name: "Программа обучения", HasChildren: true, Summary: long.Summary})
	for section := 0; section < 3; section++ {
		long.Topics = append(long.Topics, app.LearningReportTopic{Name: fmt.Sprintf("Раздел %d", section+1), Depth: 1, HasChildren: true, Summary: domain.ProgressSummary{NotStarted: 2, Learning: 2, NeedsPractice: 2, Mastered: 2}})
		for topic := 0; topic < 8; topic++ {
			name := fmt.Sprintf("Тема %02d — практические упражнения и закрепление навыков", section*8+topic+1)
			if section == 0 && topic == 4 {
				name += " " + strings.Repeat("ДлинноеНазваниеБезПробелов", 6)
			}
			long.Topics = append(long.Topics, app.LearningReportTopic{Name: name, Depth: 2, Status: statuses[topic%4], Revision: topic == 3})
		}
	}
	for _, test := range []struct {
		name          string
		report        app.LearningReport
		multiplePages bool
	}{
		{"short", short, false}, {"long", long, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			temporary := t.TempDir()
			dir := temporary
			// Optional retained artifacts allow visual review of the actual browser PDF.
			if root := os.Getenv("GRADIA_PDF_TEST_ARTIFACTS"); root != "" {
				dir = filepath.Join(root, test.name)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			htmlPath, err := reports.NewHTMLWriter(dir).Write(ctx, test.report)
			if err != nil {
				t.Fatal(err)
			}
			pdfPath := filepath.Join(filepath.Dir(htmlPath), "report.pdf")
			uriPath := filepath.ToSlash(htmlPath)
			if !strings.HasPrefix(uriPath, "/") {
				uriPath = "/" + uriPath
			}
			uri := (&url.URL{Scheme: "file", Path: uriPath}).String()
			command := exec.CommandContext(ctx, browser, "--headless", "--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage", "--disable-background-networking", "--no-first-run", "--no-default-browser-check", "--no-pdf-header-footer", "--user-data-dir="+filepath.Join(temporary, "browser-profile"), "--print-to-pdf="+pdfPath, uri)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("browser printing failed: %v\n%s", err, output)
			}
			pdf, err := os.ReadFile(pdfPath)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(pdf), "%PDF-") || len(pdf) < 1500 {
				t.Fatalf("invalid or empty PDF (%d bytes)", len(pdf))
			}
			pages := len(regexp.MustCompile(`/Type\s*/Page\b`).FindAll(pdf, -1))
			if (test.multiplePages && pages < 2) || (!test.multiplePages && pages != 1) {
				t.Fatalf("unexpected page count %d for %s report", pages, test.name)
			}
			t.Logf("PDF: %s (%d pages, %d bytes)", pdfPath, pages, len(pdf))
		})
	}
}
