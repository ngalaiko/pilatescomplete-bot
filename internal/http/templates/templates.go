package templates

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math"
	"os"
	"text/template"
	"time"

	"github.com/pilatescomplete-bot/internal/events"
	"github.com/pilatescomplete-bot/internal/statistics"
)

type WeekStatisticsData struct {
	Total    int
	Year     int
	Month    int
	Week     int
	PrevYear int
	PrevWeek int
	NextYear int
	NextWeek int
	Days     []statistics.Day
	Classes  []statistics.Class
}

type MonthStatisticsData struct {
	Total     int
	Year      int
	Month     int
	Week      int
	PrevYear  int
	PrevMonth int
	NextYear  int
	NextMonth int
	Weeks     []statistics.Week
	Classes   []statistics.Class
}

type YearStatisticsData struct {
	Total   int
	Year    int
	Month   int
	Week    int
	Months  []statistics.Month
	Classes []statistics.Class
}

type LoginData struct{}

type EventsData struct {
	Events []*events.Event
}

type Renderer interface {
	RenderEventsPage(io.Writer, EventsData) error
	RenderEvent(io.Writer, *events.Event) error
	RenderLoginPage(io.Writer, LoginData) error
	RenderYearStatisticsPage(io.Writer, YearStatisticsData) error
	RenderMonthStatisticsPage(io.Writer, MonthStatisticsData) error
	RenderWeekStatisticsPage(io.Writer, WeekStatisticsData) error
}

var _ Renderer = &FilesystemTemplates{}

type FilesystemTemplates struct {
	filesystem fs.FS
}

func NewFilesystemTemplates(path string) *FilesystemTemplates {
	log.Printf("[INFO] watching templates at: %s", path)
	return &FilesystemTemplates{
		filesystem: os.DirFS(path),
	}
}

func (e *FilesystemTemplates) RenderLoginPage(w io.Writer, data LoginData) error {
	templates, err := template.New("").Funcs(functions).ParseFS(e.filesystem, "*.template")
	if err != nil {
		return fmt.Errorf("parse fs: %w", err)
	}
	loginTemplate, err := templates.Lookup("_layout.html.template").ParseFS(e.filesystem, "login.html.template")
	if err != nil {
		return fmt.Errorf("parse login template: %w", err)
	}
	return loginTemplate.Execute(w, data)
}

func (e *FilesystemTemplates) RenderWeekStatisticsPage(w io.Writer, data WeekStatisticsData) error {
	templates, err := template.New("").Funcs(functions).ParseFS(e.filesystem, "*.template")
	if err != nil {
		return fmt.Errorf("parse fs: %w", err)
	}
	template, err := templates.Lookup("_layout.html.template").ParseFS(e.filesystem, "week_statistics.html.template")
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	return template.Execute(w, data)
}

func (e *FilesystemTemplates) RenderMonthStatisticsPage(w io.Writer, data MonthStatisticsData) error {
	templates, err := template.New("").Funcs(functions).ParseFS(e.filesystem, "*.template")
	if err != nil {
		return fmt.Errorf("parse fs: %w", err)
	}
	template, err := templates.Lookup("_layout.html.template").ParseFS(e.filesystem, "month_statistics.html.template")
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	return template.Execute(w, data)
}

func (e *FilesystemTemplates) RenderYearStatisticsPage(w io.Writer, data YearStatisticsData) error {
	templates, err := template.New("").Funcs(functions).ParseFS(e.filesystem, "*.template")
	if err != nil {
		return fmt.Errorf("parse fs: %w", err)
	}
	template, err := templates.Lookup("_layout.html.template").ParseFS(e.filesystem, "year_statistics.html.template")
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	return template.Execute(w, data)
}

func (e *FilesystemTemplates) RenderEventsPage(w io.Writer, data EventsData) error {
	templates, err := template.New("").Funcs(functions).ParseFS(e.filesystem, "*.template")
	if err != nil {
		return fmt.Errorf("parse fs: %w", err)
	}
	eventsTemplate, err := templates.Lookup("_layout.html.template").ParseFS(e.filesystem, "events.html.template")
	if err != nil {
		return fmt.Errorf("parse events template: %w", err)
	}
	return eventsTemplate.Execute(w, data)
}

func (e *FilesystemTemplates) RenderEvent(w io.Writer, event *events.Event) error {
	templates, err := template.New("").Funcs(functions).ParseFS(e.filesystem, "*.template")
	if err != nil {
		return fmt.Errorf("parse fs: %w", err)
	}
	return templates.Lookup("event").Execute(w, event)
}

var _ Renderer = &EmbedTemplates{}

type EmbedTemplates struct {
	loginTemplate           *template.Template
	eventTemplate           *template.Template
	eventsTemplate          *template.Template
	yearStatisticsTemplate  *template.Template
	monthStatisticsTemplate *template.Template
	weekStatisticsTemplate  *template.Template
}

//go:embed *.template
var embedFS embed.FS

var (
	minute = time.Second * 60
	hour   = minute * 60
	day    = hour * 60
)

var functions = map[string]interface{}{
	"now":        time.Now,
	"startOfDay": func(ts time.Time) time.Time { return ts.Round(day) },
	"plusHours":  func(hours int, ts time.Time) time.Time { return ts.Add(time.Duration(hours) * hour) },
	"count": func(n int) []int {
		c := make([]int, n)
		for i := 0; i < n; i++ {
			c[i] = i
		}
		return c
	},
	"fraction": func(a, b int) float64 {
		return math.Trunc(float64(a)/float64(b)*1000) / 10
	},
	"inc":              func(value int) int { return value + 1 },
	"dec":              func(value int) int { return value - 1 },
	"shortMonthName":   func(i int) string { return time.Month(i).String()[:3] },
	"shortWeekdayName": func(i int) string { return time.Weekday(i).String()[:3] },
	"monthName":        func(i int) string { return time.Month(i).String() },
}

func NewEmbedTemplates() *EmbedTemplates {
	base := template.New("").Funcs(functions)

	log.Printf("[INFO] using embedded templates")
	templates := template.Must(base.ParseFS(embedFS, "*.template"))
	layoutTemplate := templates.Lookup("_layout.html.template")
	return &EmbedTemplates{
		loginTemplate:           template.Must(template.Must(layoutTemplate.Clone()).ParseFS(embedFS, "login.html.template")),
		eventsTemplate:          template.Must(template.Must(layoutTemplate.Clone()).ParseFS(embedFS, "events.html.template")),
		eventTemplate:           templates.Lookup("event"),
		yearStatisticsTemplate:  template.Must(template.Must(layoutTemplate.Clone()).ParseFS(embedFS, "year_statistics.html.template")),
		monthStatisticsTemplate: template.Must(template.Must(layoutTemplate.Clone()).ParseFS(embedFS, "month_statistics.html.template")),
		weekStatisticsTemplate:  template.Must(template.Must(layoutTemplate.Clone()).ParseFS(embedFS, "week_statistics.html.template")),
	}
}

func (e *EmbedTemplates) RenderLoginPage(w io.Writer, data LoginData) error {
	return e.loginTemplate.Execute(w, data)
}

func (e *EmbedTemplates) RenderEventsPage(w io.Writer, data EventsData) error {
	return e.eventsTemplate.Execute(w, data)
}

func (e *EmbedTemplates) RenderEvent(w io.Writer, event *events.Event) error {
	return e.eventTemplate.Execute(w, event)
}

func (e *EmbedTemplates) RenderYearStatisticsPage(w io.Writer, data YearStatisticsData) error {
	return e.yearStatisticsTemplate.Execute(w, data)
}

func (e *EmbedTemplates) RenderMonthStatisticsPage(w io.Writer, data MonthStatisticsData) error {
	return e.monthStatisticsTemplate.Execute(w, data)
}

func (e *EmbedTemplates) RenderWeekStatisticsPage(w io.Writer, data WeekStatisticsData) error {
	return e.weekStatisticsTemplate.Execute(w, data)
}
