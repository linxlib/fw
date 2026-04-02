package middlewares

import (
	"fmt"

	ctxpkg "github.com/linxlib/fw/context"
	"github.com/linxlib/fw/middleware"
	"github.com/valyala/fasthttp"
)

const (
	AuthorizationKeyUserID   = "Authorization.UserID"
	AuthorizationKeyUserRole = "Authorization.UserRole"
)

type AuthorizationMiddleware struct{}

func (AuthorizationMiddleware) Spec() middleware.AnnotationSpec {
	return middleware.AnnotationSpec{Name: "Authorization", Scope: middleware.ScopeBoth, Stage: middleware.StageBefore}
}

func (AuthorizationMiddleware) SecurityScheme() middleware.SecurityScheme {
	return middleware.SecurityScheme{
		Name:        "ApiKeyAuth",
		Type:        middleware.SecurityAPIKey,
		In:          middleware.SecurityInHeader,
		FieldName:   "x-api-key",
		Description: "API Key authentication via x-api-key header",
	}
}

func (AuthorizationMiddleware) Handle(ctx ctxpkg.Context, _ middleware.AnnotationArgs, next middleware.Handler) error {
	apiKey := string(ctx.Raw().Request.Header.Peek("x-api-key"))
	if apiKey == "" {
		_ = ctx.Respond(fasthttp.StatusUnauthorized, 40100, "missing x-api-key header", nil)
		return fmt.Errorf("unauthorized: missing x-api-key")
	}

	ctx.Set(AuthorizationKeyUserID, 1001)
	ctx.Set(AuthorizationKeyUserRole, "Admin")
	return next(ctx)
}
