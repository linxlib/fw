package controllers

import (
	"fmt"
	"time"

	"github.com/linxlib/fw/v2/cmd/example/middlewares"
	"github.com/linxlib/fw/v2/cmd/example/models"
	"github.com/linxlib/fw/v2/cmd/example/services"
	ctxpkg "github.com/linxlib/fw/v2/context"
	"github.com/valyala/fasthttp"
)

// UserController 用户
// @Controller
// @Route /api/v1
// @Authorization(Admin)
type UserController struct {
	Service *services.UserService
}

// ModifyUser 修改用户
// @POST /user
// @POST /user_info
// @Log(stage=before)
func (c *UserController) ModifyUser(ctx ctxpkg.Context, query models.UserQuery, db *services.DemoDB) (int, error) {
	if c.Service == nil {
		c.Service = services.NewUserService()
	}

	userID := ctx.GetInt(middlewares.AuthorizationKeyUserID)
	role := ctx.GetString(middlewares.AuthorizationKeyUserRole)
	fmt.Printf("authenticated user: id=%d role=%s\n", userID, role)

	return 1, ctx.Respond(fasthttp.StatusOK, 0, "ok", c.Service.BuildModifyUserResponse(query, userID, role, db))
}

// HealthCheck 健康检查（无需鉴权）
// @GET /health
// @Ignore(Authorization,Global)
func (c *UserController) HealthCheck(ctx ctxpkg.Context) error {
	return ctx.Respond(fasthttp.StatusOK, 0, "healthy", nil)
}

// Echo handles WebSocket messages. Each incoming []byte is echoed back with a prefix.
// @GET /ws
// @WS
// @Ignore(Authorization)
func (c *UserController) Echo(ctx ctxpkg.Context, msg []byte) error {
	if c.Service == nil {
		c.Service = services.NewUserService()
	}
	reply := c.Service.BuildWSReply(msg)
	ctx.Set(middlewares.WSKeyReply, reply)
	return nil
}

// Stream demonstrates Server-Sent Events. The handler receives an SSEWriter
// and pushes a series of events to the client.
// @GET /stream
// @SSE
func (c *UserController) Stream(ctx ctxpkg.Context, w *ctxpkg.SSEWriter) error {
	for i := 1; i <= 5; i++ {
		if err := w.WriteEventJSON("tick", map[string]any{
			"seq":  i,
			"time": time.Now().Format(time.RFC3339),
		}); err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
	}
	_ = w.WriteText("[DONE]")
	return nil
}
