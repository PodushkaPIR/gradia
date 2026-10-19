package app

import (
	"context"
	"errors"

	"github.com/shesh/gradia/internal/domain"
)

// LearningReport deliberately has no tutor notes, history, or unrelated student data.
type LearningReport struct {
	StudentName, PlanName, Description string
	Summary                            domain.ProgressSummary
	Topics                             []LearningReportTopic
}

type LearningReportTopic struct {
	Name        string
	Depth       int
	HasChildren bool
	Status      domain.ProgressStatus
	Revision    bool
	Summary     domain.ProgressSummary
}

type ReportWriter interface {
	Write(context.Context, LearningReport) (string, error)
}

type PDFExporter interface {
	Export(context.Context, string) (string, error)
}

type ReportService struct {
	students *StudentService
	routes   *RouteService
	writer   ReportWriter
	open     func(context.Context, string) error
	pdf      PDFExporter
}

func NewReportService(students *StudentService, routes *RouteService, writer ReportWriter, open func(context.Context, string) error) *ReportService {
	return &ReportService{students: students, routes: routes, writer: writer, open: open}
}

func (s *ReportService) WithPDFExporter(exporter PDFExporter) *ReportService {
	configured := *s
	configured.pdf = exporter
	return &configured
}

func (s *ReportService) PDFExportEnabled() bool { return s.pdf != nil }

func (s *ReportService) ExportPDF(ctx context.Context, htmlPath string) (string, error) {
	if s.pdf == nil {
		return "", errors.New("PDF-экспорт не настроен; используйте печать HTML-отчёта из браузера")
	}
	return s.pdf.Export(ctx, htmlPath)
}

func (s *ReportService) Generate(ctx context.Context, studentID domain.StudentID, routeID domain.RouteID) (string, error) {
	if studentID <= 0 {
		return "", domain.ErrStudentNotFound
	}
	if routeID <= 0 {
		return "", domain.ErrRouteNotFound
	}
	students, err := s.students.List(ctx)
	if err != nil {
		return "", err
	}
	var student domain.Student
	for _, candidate := range students {
		if candidate.ID == studentID {
			student = candidate
			break
		}
	}
	if student.ID == 0 {
		return "", domain.ErrStudentNotFound
	}
	plans, err := s.routes.List(ctx, studentID)
	if err != nil {
		return "", err
	}
	var plan domain.Route
	for _, candidate := range plans {
		if candidate.ID == routeID {
			plan = candidate
			break
		}
	}
	if plan.ID == 0 {
		return "", domain.ErrRouteNotFound
	}
	topics, err := s.routes.Topics(ctx, routeID)
	if err != nil {
		return "", err
	}
	summaries := RouteProgressSummary(topics)
	report := LearningReport{StudentName: student.Name, PlanName: plan.Name, Description: plan.Description, Summary: summaries[0]}
	for _, row := range domain.RouteTopicTreeRows(topics) {
		if row.EffectivelyExcluded {
			continue
		}
		report.Topics = append(report.Topics, LearningReportTopic{Name: row.Topic.Name, Depth: row.Depth, HasChildren: row.HasChildren, Status: row.Topic.Status.Effective(), Revision: !row.HasChildren && row.Topic.Revision, Summary: summaries[row.Topic.ID]})
	}
	return s.writer.Write(ctx, report)
}

func (s *ReportService) Open(ctx context.Context, path string) error {
	if s.open == nil {
		return errors.New("не настроено открытие локального браузера")
	}
	return s.open(ctx, path)
}
