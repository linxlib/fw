package middlewares

import (
	"fmt"

	"github.com/linxlib/fw/v2/config"
	ctxpkg "github.com/linxlib/fw/v2/context"
	"github.com/linxlib/fw/v2/middleware"
	"github.com/valyala/fasthttp"
)

const (
	AuthorizationKeyUserID   = "Authorization.UserID"
	AuthorizationKeyUserRole = "Authorization.UserRole"
)

// @Middleware
// @Global
type AuthorizationMiddleware struct {
	Config *config.Section `inject:""`
}

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

func (am *AuthorizationMiddleware) Handle(ctx ctxpkg.Context, _ middleware.AnnotationArgs, next middleware.Handler) error {
	expectedAPIKey := am.Config.Get("api-key")
	if expectedAPIKey == "" {
		_ = ctx.Respond(fasthttp.StatusUnauthorized, 40100, "authorization middleware api-key is not configured", nil)
		return fmt.Errorf("unauthorized: authorization middleware api-key is not configured")
	}

	apiKey := string(ctx.Raw().Request.Header.Peek("x-api-key"))
	if apiKey == "" {
		_ = ctx.Respond(fasthttp.StatusUnauthorized, 40100, "missing x-api-key header", nil)
		return fmt.Errorf("unauthorized: missing x-api-key")
	}
	if apiKey != expectedAPIKey {
		_ = ctx.Respond(fasthttp.StatusUnauthorized, 40100, "invalid x-api-key", nil)
		return fmt.Errorf("unauthorized: invalid x-api-key")
	}

	ctx.Set(AuthorizationKeyUserID, 1001)
	ctx.Set(AuthorizationKeyUserRole, "Admin")
	return next(ctx)
}
