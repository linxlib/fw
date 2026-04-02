package middlewares

import (
	"fmt"

	"github.com/fasthttp/websocket"
	ctxpkg "github.com/linxlib/fw/context"
	"github.com/linxlib/fw/middleware"
	"github.com/valyala/fasthttp"
)

const (
	WSKeyConn  = "WS.Conn"
	WSKeyReply = "WS.Reply"
)

type WebSocketMiddleware struct{}

func (WebSocketMiddleware) Spec() middleware.AnnotationSpec {
	return middleware.AnnotationSpec{Name: "WS", Scope: middleware.ScopeMethod, Stage: middleware.StageBoth}
}

func (WebSocketMiddleware) Handle(ctx ctxpkg.Context, _ middleware.AnnotationArgs, next middleware.Handler) error {
	upgrader := websocket.FastHTTPUpgrader{CheckOrigin: func(ctx *fasthttp.RequestCtx) bool { return true }}

	return upgrader.Upgrade(ctx.Raw(), func(conn *websocket.Conn) {
		defer conn.Close()
		ctx.Set(WSKeyConn, conn)

		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			if msgType != websocket.TextMessage && msgType != websocket.BinaryMessage {
				continue
			}

			container := ctx.Container()
			if container != nil {
				container.Map(msg)
			}

			if err := next(ctx); err != nil {
				_ = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"error":"%s"}`, err.Error())))
				continue
			}

			if reply, ok := ctx.Get(WSKeyReply); ok {
				switch v := reply.(type) {
				case []byte:
					_ = conn.WriteMessage(msgType, v)
				case string:
					_ = conn.WriteMessage(websocket.TextMessage, []byte(v))
				}
				ctx.Set(WSKeyReply, nil)
			}
		}
	})
}
