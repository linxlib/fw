package render

import (
	"fmt"
	"github.com/linxlib/conv"
	"github.com/valyala/fasthttp"
)

type String struct {
	Format string
	Data   []any
}

var plainContentType = []string{"text/plain; charset=utf-8"}

func (r String) Render(w *fasthttp.RequestCtx) error {
	return WriteString(w, r.Format, r.Data)
}
func (r String) WriteContentType(w *fasthttp.RequestCtx) {
	writeContentType(w, plainContentType)
}

func WriteString(w *fasthttp.RequestCtx, format string, data []any) (err error) {
	writeContentType(w, plainContentType)
	if len(data) > 0 {
		_, err = fmt.Fprintf(w, format, data...)
		return
	}
	_, err = w.Write(conv.Bytes(format))
	return
}
