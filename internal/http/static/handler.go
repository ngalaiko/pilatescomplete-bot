package static

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
)

func NewFilesystemHandler(path string) http.HandlerFunc {
	return http.FileServer(http.Dir(path)).ServeHTTP
}

//go:embed files/*
var embedFS embed.FS

func NewEmbedHandler() http.HandlerFunc {
	files, err := fs.Sub(embedFS, "files")
	if err != nil {
		log.Fatal(err)
	}
	return http.FileServer(http.FS(files)).ServeHTTP
}
