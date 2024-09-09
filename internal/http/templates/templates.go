package templates

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"text/template"
	"time"

	"github.com/pilatescomplete-bot/internal/events"
)

type LoginData struct{}

type EventsData struct {
	Events []*events.Event
}

type Renderer interface {
	RenderEventsPage(io.Writer, EventsData) error
	RenderLoginPage(io.Writer, LoginData) error
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

var _ Renderer = &EmbedTemplates{}

type EmbedTemplates struct {
	loginTemplate  *template.Template
	eventsTemplate *template.Template
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
}

func NewEmbedTemplates() *EmbedTemplates {
	base := template.New("").Funcs(functions)

	log.Printf("[INFO] using embedded templates")
	templates := template.Must(base.ParseFS(embedFS, "*.template"))
	layoutTemplate := templates.Lookup("_layout.html.template")
	return &EmbedTemplates{
		loginTemplate:  template.Must(template.Must(layoutTemplate.Clone()).ParseFS(embedFS, "login.html.template")),
		eventsTemplate: template.Must(template.Must(layoutTemplate.Clone()).ParseFS(embedFS, "events.html.template")),
	}
}

func (e *EmbedTemplates) RenderLoginPage(w io.Writer, data LoginData) error {
	return e.loginTemplate.Execute(w, data)
}

func (e *EmbedTemplates) RenderEventsPage(w io.Writer, data EventsData) error {
	return e.eventsTemplate.Execute(w, data)
}
