package middlewares

import (
	"github.com/linxlib/fw"
	"io"
	"net/http"
	"strings"
)

// ServerDownMiddleware is a middleware which provides an api to mark server down.
type ServerDownMiddleware struct {
	*fw.MiddlewareGlobal
	key        string
	serverDown bool
}

func (s *ServerDownMiddleware) Execute(ctx *fw.MiddlewareContext) fw.HandlerFunc {
	return func(context *fw.Context) {
		if s.serverDown {
			resp, _ := http.Get("https://shuye.dev/maintenance-page/")
			bs, _ := io.ReadAll(resp.Body)
			context.Data(200, "text/html", bs)
			return
		}

		ctx.Next(context)
	}
}

func (s *ServerDownMiddleware) Router(ctx *fw.MiddlewareContext) []*fw.RouteItem {
	return []*fw.RouteItem{&fw.RouteItem{
		Method: "PATCH",
		Path:   "/serverDown/{key}",
		IsHide: true,
		H: func(context *fw.Context) {
			str := context.GetFastContext().UserValue("key").(string)
			str = strings.TrimSpace(str)
			if str == s.key {
				s.serverDown = !s.serverDown
			}
			context.String(200, "ok")
		},
		Middleware: s,
	}}
}

const serverDownName = "ServerDown"

func NewServerDownMiddleware(key string) fw.IMiddlewareGlobal {

	return &ServerDownMiddleware{
		key:              key,
		MiddlewareGlobal: fw.NewMiddlewareGlobal(serverDownName),
	}
}
