package openapi

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
)

//go:embed swagger.html
var swaggerHTML []byte

func RegisterDocsRoutes(r *router.Router, openAPIFilePath string) {
	if r == nil {
		return
	}
	r.GET("/docs", func(ctx *fasthttp.RequestCtx) {
		ctx.SetContentType("text/html; charset=utf-8")
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetBody(swaggerHTML)
	})
	r.GET("/docs/config", func(ctx *fasthttp.RequestCtx) {
		cfg := map[string]any{
			"url":                "/docs/openapi.json",
			"deepLinking":        true,
			"docExpansion":       "none",
			"queryConfigEnabled": true,
		}
		b, _ := json.Marshal(cfg)
		ctx.SetContentType("application/json")
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetBody(b)
	})
	r.GET("/docs/openapi.json", func(ctx *fasthttp.RequestCtx) {
		data, err := os.ReadFile(openAPIFilePath)
		if err != nil {
			ctx.SetStatusCode(fasthttp.StatusNotFound)
			ctx.SetContentType("application/json")
			ctx.SetBodyString(fmt.Sprintf(`{"error":"openapi not found: %s"}`, openAPIFilePath))
			return
		}
		ctx.SetContentType("application/json")
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetBody(data)
	})
}
