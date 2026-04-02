package main

import (
	"fmt"

	"github.com/linxlib/fw/app"
	ctxpkg "github.com/linxlib/fw/context"
	"github.com/linxlib/fw/middleware"
	"github.com/valyala/fasthttp"
)

type AuthorizationMiddleware struct{}

func (AuthorizationMiddleware) Spec() middleware.AnnotationSpec {
	return middleware.AnnotationSpec{Name: "Authorization", Scope: middleware.ScopeBoth, Stage: middleware.StageBefore}
}

func (AuthorizationMiddleware) Before(ctx ctxpkg.Context, args middleware.AnnotationArgs) error {
	if len(args.Args) == 0 {
		return nil
	}
	required := args.Args[0]
	role := string(ctx.Raw().Request.Header.Peek("X-Role"))
	if role != required {
		_ = ctx.Respond(fasthttp.StatusForbidden, 40300, "forbidden", map[string]any{"required_role": required})
		return fmt.Errorf("unauthorized")
	}
	return nil
}

func (AuthorizationMiddleware) After(ctx ctxpkg.Context, args middleware.AnnotationArgs) error {
	return nil
}

type LogMiddleware struct{}

func (LogMiddleware) Spec() middleware.AnnotationSpec {
	return middleware.AnnotationSpec{Name: "Log", Scope: middleware.ScopeBoth, Stage: middleware.StageBoth}
}

func (LogMiddleware) Before(ctx ctxpkg.Context, args middleware.AnnotationArgs) error {
	ctx.Raw().Response.Header.Set("X-Log-Before", "1")
	return nil
}

func (LogMiddleware) After(ctx ctxpkg.Context, args middleware.AnnotationArgs) error {
	ctx.Raw().Response.Header.Set("X-Log-After", "1")
	return nil
}

type UserQuery struct {
	Name string `required:"true"` //name
	Age  int    //年龄
}

// UserController 用户
// @Route /api/v1
// @Authorization(Admin)
type UserController struct{}

// ModifyUser 修改用户
// @POST /user
// @POST /user_info
// @Log(stage=before)
func (c *UserController) ModifyUser(ctx ctxpkg.Context, query UserQuery) (int, error) {
	return 1, ctx.Respond(fasthttp.StatusOK, 0, "ok", map[string]any{"name": query.Name, "age": query.Age})
}

func main() {
	e, err := app.New("")
	if err != nil {
		panic(err)
	}
	e.RegisterMiddleware(AuthorizationMiddleware{})
	e.RegisterMiddleware(LogMiddleware{})
	e.RegisterController(&UserController{})
	if err := e.ListenAndServe(); err != nil {
		panic(err)
	}
}
