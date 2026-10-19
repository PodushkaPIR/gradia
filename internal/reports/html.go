package reports

import (
	"bytes"
	"context"
	_ "embed"
	"html/template"
	"os"
	"path/filepath"

	"github.com/shesh/gradia/internal/app"
	"github.com/shesh/gradia/internal/domain"
)

//go:embed learning_report.html
var reportTemplate string

var htmlReport = template.Must(template.New("learning-report").Funcs(template.FuncMap{
	"current": func(s domain.ProgressSummary) int { return s.Learning + s.NeedsPractice },
	"percent": func(s domain.ProgressSummary) int {
		if s.Total() == 0 {
			return 0
		}
		return 100 * s.Mastered / s.Total()
	},
	"status": func(s domain.ProgressStatus) string {
		switch s.Effective() {
		case domain.ProgressLearning:
			return "изучается · текущая"
		case domain.ProgressNeedsPractice:
			return "нужна практика · текущая"
		case domain.ProgressMastered:
			return "освоено · завершённая"
		default:
			return "не начато · предстоящая"
		}
	},
}).Parse(reportTemplate))

type HTMLWriter struct{ directory string }

func NewHTMLWriter(directory string) *HTMLWriter { return &HTMLWriter{directory: directory} }

func (w *HTMLWriter) Write(ctx context.Context, report app.LearningReport) (path string, err error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var body bytes.Buffer
	if err := htmlReport.Execute(&body, report); err != nil {
		return "", err
	}
	directory, err := filepath.Abs(w.directory)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(directory, "learning-plan-*.html")
	if err != nil {
		return "", err
	}
	defer func() {
		file.Close()
		if err != nil {
			os.Remove(file.Name())
		}
	}()
	if _, err := file.Write(body.Bytes()); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return file.Name(), nil
}
