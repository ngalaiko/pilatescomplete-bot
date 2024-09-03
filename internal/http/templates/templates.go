package templates

import (
	"embed"
	"io"
	"text/template"
	"time"

	"github.com/pilatescomplete-bot/internal/events"
)

//go:embed *.template
var fs embed.FS

var funcs = template.New("").Funcs(map[string]interface{}{
	"now": time.Now,
})

var templates = template.Must(funcs.ParseFS(fs, "*.template"))

var layoutTemplate = templates.Lookup("_layout.html.template")

var loginTemplate = template.Must(template.Must(layoutTemplate.Clone()).ParseFS(fs, "login.html.template"))

type LoginData struct{}

func Login(w io.Writer, data LoginData) error {
	return loginTemplate.Execute(w, data)
}

var eventsTemplate = template.Must(template.Must(layoutTemplate.Clone()).ParseFS(fs, "events.html.template"))

type EventsData struct {
	Events []*events.Event
	From   time.Time
	To     time.Time
}

func Events(w io.Writer, data EventsData) error {
	return eventsTemplate.Execute(w, data)
}
