package render

import (
	"github.com/valyala/fasthttp"
	"html/template"
)

type Delims struct {
	// Left delimiter, defaults to {{.
	Left string
	// Right delimiter, defaults to }}.
	Right string
}
type IHTMLRender interface {
	// Instance returns an HTML instance.
	Instance(string, any) IRender
}
type HTMLProduction struct {
	Template *template.Template
	Delims   Delims
}
type HTMLDebug struct {
	Files   []string
	Glob    string
	Delims  Delims
	FuncMap template.FuncMap
}
type HTML struct {
	Template *template.Template
	Name     string
	Data     any
}

var htmlContentType = []string{"text/html; charset=utf-8"}

func (r HTMLProduction) Instance(name string, data any) IRender {
	return HTML{
		Template: r.Template,
		Name:     name,
		Data:     data,
	}
}
func (r HTMLDebug) Instance(name string, data any) IRender {
	return HTML{
		Template: r.loadTemplate(),
		Name:     name,
		Data:     data,
	}
}
func (r HTMLDebug) loadTemplate() *template.Template {
	if r.FuncMap == nil {
		r.FuncMap = template.FuncMap{}
	}
	if len(r.Files) > 0 {
		return template.Must(template.New("").Delims(r.Delims.Left, r.Delims.Right).Funcs(r.FuncMap).ParseFiles(r.Files...))
	}
	if r.Glob != "" {
		return template.Must(template.New("").Delims(r.Delims.Left, r.Delims.Right).Funcs(r.FuncMap).ParseGlob(r.Glob))
	}
	panic("the HTML debug render was created without files or glob pattern")
}
func (r HTML) Render(w *fasthttp.RequestCtx) error {
	r.WriteContentType(w)

	if r.Name == "" {
		return r.Template.Execute(w, r.Data)
	}
	return r.Template.ExecuteTemplate(w, r.Name, r.Data)
}
func (r HTML) WriteContentType(w *fasthttp.RequestCtx) {
	writeContentType(w, htmlContentType)
}
