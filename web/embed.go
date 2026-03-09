package web

import (
	"embed"
	"io/fs"
)

//go:embed all:ui
var uiFS embed.FS

//go:embed all:docs
var docsFS embed.FS

func GetUIFS() (fs.FS, error) {
	return fs.Sub(uiFS, "ui")
}

func GetDocsFS() (fs.FS, error) {
	return fs.Sub(docsFS, "docs")
}
