package middlewares

import (
	ctxpkg "github.com/linxlib/fw/v2/context"
	"github.com/linxlib/fw/v2/middleware"
)

// @Middleware
type SSEMiddleware struct{}

func (SSEMiddleware) Spec() middleware.AnnotationSpec {
	return middleware.AnnotationSpec{Name: "SSE", Scope: middleware.ScopeMethod, Stage: middleware.StageBoth}
}

func (SSEMiddleware) Handle(ctx ctxpkg.Context, _ middleware.AnnotationArgs, next middleware.Handler) error {
	return ctx.SSE(func(w *ctxpkg.SSEWriter) {
		container := ctx.Container()
		if container != nil {
			container.Map(w)
		}

		if err := next(ctx); err != nil {
			_ = w.WriteEvent("error", err.Error())
		}
	})
}
