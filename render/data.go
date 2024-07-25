package render

import (
	"github.com/valyala/fasthttp"
)

type Data struct {
	ContentType string
	Data        []byte
}

func (r Data) Render(w *fasthttp.RequestCtx) (err error) {
	r.WriteContentType(w)
	_, err = w.Write(r.Data)
	return
}
func (r Data) WriteContentType(w *fasthttp.RequestCtx) {
	writeContentType(w, []string{r.ContentType})
}
