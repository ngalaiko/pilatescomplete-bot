package templates

import (
	"embed"
	"io"
	"text/template"
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

var indexTemplate = template.Must(template.Must(layoutTemplate.Clone()).ParseFS(fs, "index.html.template"))

type IndexData struct{}

func Index(w io.Writer, data IndexData) error {
	return indexTemplate.Execute(w, data)
}
