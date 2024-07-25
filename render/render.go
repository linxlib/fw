package render

import (
	"github.com/valyala/fasthttp"
	"strings"
)

type IRender interface {
	Render(*fasthttp.RequestCtx) error
	WriteContentType(*fasthttp.RequestCtx)
}

var (
	_ IRender     = (*JSON)(nil)
	_ IRender     = (*IndentedJSON)(nil)
	_ IRender     = (*SecureJSON)(nil)
	_ IRender     = (*JsonpJSON)(nil)
	_ IRender     = (*XML)(nil)
	_ IRender     = (*String)(nil)
	_ IRender     = (*Redirect)(nil)
	_ IRender     = (*Data)(nil)
	_ IRender     = (*HTML)(nil)
	_ IHTMLRender = (*HTMLDebug)(nil)
	_ IHTMLRender = (*HTMLProduction)(nil)
	//_ IRender = (*YAML)(nil)
	_ IRender = (*Reader)(nil)
	_ IRender = (*AsciiJSON)(nil)
	//_ IRender = (*ProtoBuf)(nil)
	//_ IRender = (*TOML)(nil)
)

func writeContentType(w *fasthttp.RequestCtx, value []string) {
	w.SetContentType(strings.Join(value, ";"))
}
func doWrite(w *fasthttp.RequestCtx, value []byte) (int, error) {
	if w.UserValue("enable_gzip") == true {
		w.Response.Header.Set("Content-Encoding", "gzip")
		return fasthttp.WriteGzip(w.Response.BodyWriter(), value)
	} else {
		return w.Write(value)
	}

}
