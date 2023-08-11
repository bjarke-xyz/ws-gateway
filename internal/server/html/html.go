package html

import (
	"embed"
	"io"
	"text/template"

	"github.com/bjarke-xyz/ws-gateway/internal/domain"
)

//go:embed pages/*.html
var files embed.FS

var (
	adminTemplate = parse("pages/admin.html")
	appTemplate   = parse("pages/app.html")
	loginTemplate = parse("pages/login.html")
)

type AdminParams struct {
	Title string
	Error string
	Apps  []domain.Application
}

func AdminPage(w io.Writer, p AdminParams) error {
	return adminTemplate.Execute(w, p)
}

type AppParams struct {
	Title string
	Error string
	App   domain.Application
}

func AppPage(w io.Writer, p AppParams) error {
	return appTemplate.Execute(w, p)
}

type LoginParams struct {
	Title string
	Error string
}

func LoginPage(w io.Writer, p LoginParams) error {
	return loginTemplate.Execute(w, p)
}

func parse(file string) *template.Template {
	return template.Must(
		template.New("layout.html").ParseFS(files, "pages/layout.html", file))
}
