package templates

import (
	"embed"
	"io"
	"io/fs"
	"log"
	"text/template"
)

//go:embed *.gotmpl
var templatesFS embed.FS

var t = mustParseFS(templatesFS)

func mustParseFS(fs fs.FS) *template.Template {
	templates, err := template.ParseFS(fs, "*.gotmpl")
	if err != nil {
		panic(err)
	}
	for _, t := range templates.Templates() {
		log.Printf("[INFO] parsed template: %s", t.Name())
	}
	return templates
}

type IndexData struct {
	Authenticated bool
}

func Index(w io.Writer, data IndexData) error {
	return t.ExecuteTemplate(w, "index.html.gotmpl", data)
}
