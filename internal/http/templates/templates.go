package templates

import (
	"embed"
	"io"
	"text/template"
	"time"
)

//go:embed *.template
var fs embed.FS

var templates = template.Must(template.ParseFS(fs, "*.template"))

var layoutTemplate = templates.Lookup("_layout.html.template")

var loginTemplate = template.Must(template.Must(layoutTemplate.Clone()).ParseFS(fs, "login.html.template"))

type LoginData struct{}

func Login(w io.Writer, data LoginData) error {
	return loginTemplate.Execute(w, data)
}

var eventsTemplate = template.Must(template.Must(layoutTemplate.Clone()).ParseFS(fs, "events.html.template"))

type Event struct {
	ID       string
	Location string
	Name     string
	Time     time.Time
}

type EventsData struct {
	Events []*Event
	From   time.Time
	To     time.Time
}

func Events(w io.Writer, data EventsData) error {
	return eventsTemplate.Execute(w, data)
}
