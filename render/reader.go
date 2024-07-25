package render

import (
	"github.com/valyala/fasthttp"
	"io"
	"strconv"
)

type Reader struct {
	ContentType   string
	ContentLength int64
	Reader        io.Reader
	Headers       map[string]string
}

func (r Reader) Render(w *fasthttp.RequestCtx) (err error) {
	r.WriteContentType(w)
	if r.ContentLength >= 0 {
		if r.Headers == nil {
			r.Headers = map[string]string{}
		}
		r.Headers["Content-Length"] = strconv.FormatInt(r.ContentLength, 10)
	}
	r.writeHeaders(w, r.Headers)
	_, err = io.Copy(w, r.Reader)
	return
}
func (r Reader) WriteContentType(w *fasthttp.RequestCtx) {
	writeContentType(w, []string{r.ContentType})
}
func (r Reader) writeHeaders(w *fasthttp.RequestCtx, headers map[string]string) {
	for k, v := range headers {
		if w.Response.Header.Peek(k) == nil {
			w.Response.Header.Set(k, v)
		}
	}
}
