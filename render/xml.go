package render

import (
	"encoding/xml"
	"github.com/valyala/fasthttp"
)

type XML struct {
	Data any
}

var xmlContentType = []string{"application/xml; charset=utf-8"}

func (r XML) Render(w *fasthttp.RequestCtx) error {
	r.WriteContentType(w)
	return xml.NewEncoder(w).Encode(r.Data)
}

func (r XML) WriteContentType(w *fasthttp.RequestCtx) {
	writeContentType(w, xmlContentType)
}
