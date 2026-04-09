package middlewares

import (
	ctxpkg "github.com/linxlib/fw/v2/context"
	"github.com/linxlib/fw/v2/middleware"
)

// @Middleware
// @Global
type LogMiddleware struct{}

func (LogMiddleware) Spec() middleware.AnnotationSpec {
	return middleware.AnnotationSpec{Name: "Log", Scope: middleware.ScopeBoth, Stage: middleware.StageBoth}
}

func (LogMiddleware) Handle(ctx ctxpkg.Context, _ middleware.AnnotationArgs, next middleware.Handler) error {
	ctx.Raw().Response.Header.Set("X-Log-Before", "1")
	err := next(ctx)
	ctx.Raw().Response.Header.Set("X-Log-After", "1")
	return err
}
