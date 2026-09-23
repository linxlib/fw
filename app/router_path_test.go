package app

import (
	"reflect"
	"strings"
	"testing"

	"github.com/fasthttp/router"
	"github.com/linxlib/fw/v2/astp"
	ctxpkg "github.com/linxlib/fw/v2/context"
	"github.com/linxlib/fw/v2/inject"
	"github.com/linxlib/fw/v2/logger"
	"github.com/linxlib/fw/v2/middleware"
	"github.com/linxlib/fw/v2/response"
	"github.com/valyala/fasthttp"
)

func TestRouterPath(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"/users", "/users"},
		{"/users/", "/users/"},
		{"/users/:id", "/users/{id}"},
		{"/users/:id/disable", "/users/{id}/disable"},
		{"/:a/:b", "/{a}/{b}"},
		// 不含参数的路径必须原样返回
		{"/api/hello", "/api/hello"},
	}
	for _, tt := range tests {
		if got := routerPath(tt.in); got != tt.want {
			t.Errorf("routerPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestPathParamRouteDispatch 保证 ":name" 参数路由真的能把请求分发到处理器,
// 并且路径参数能被解析出来.
//
// 回归背景: github.com/fasthttp/router 从 v1.5 起改用 "{name}" 语法, fw 此前直接把
// ":name" 注册进去, 所有参数路由静默 404.
func TestPathParamRouteDispatch(t *testing.T) {
	logg, err := logger.New("info", "console", "", false)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	newEngine := func() *Engine {
		cfg := DefaultEngineConfig()
		return &Engine{
			cfg:             &cfg,
			log:             logg,
			router:          router.New(),
			globalContainer: inject.New(),
			responseManager: response.NewManager(nil, nil),
			controllers:     make(map[string]reflect.Value),
			middlewares:     make(map[string]middleware.Middleware),
		}
	}

	funcParams := func(names ...string) *astp.Func {
		fn := &astp.Func{Name: "handler"}
		for _, n := range names {
			fn.Params = append(fn.Params, &astp.Param{Name: n, Type: &astp.TypeRef{Name: "int64", Kind: astp.KindBasic}})
		}
		return fn
	}

	t.Run("参数路由可达且路径参数可用", func(t *testing.T) {
		e := newEngine()
		var gotID string
		e.routes = []routeDef{{
			Method:     fasthttp.MethodGet,
			Path:       "/users/:id",
			ParamNames: []string{"id"},
			Controller: "UserController",
			Handler:    "Get",
			HandlerValue: reflect.ValueOf(func(ctx ctxpkg.Context, id int64) (map[string]any, error) {
				gotID = ctx.Param("id")
				return map[string]any{"id": id}, nil
			}),
			AstMethod: funcParams("ctx", "id"),
			ParamHints: map[string]ParamHint{
				"id": {Name: "id", Source: BindPath},
			},
		}}
		if err := e.registerRoutes(); err != nil {
			t.Fatalf("registerRoutes: %v", err)
		}

		status, body := executeTestRequest(t, e, fasthttp.MethodGet, "/users/42")
		if status != fasthttp.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", status, fasthttp.StatusOK, body)
		}
		if gotID != "42" {
			t.Errorf(`ctx.Param("id") = %q, want "42"`, gotID)
		}
		if !strings.Contains(string(body), `"id":42`) {
			t.Errorf("body = %s, want it to contain \"id\":42", body)
		}
	})

	t.Run("多段参数路由", func(t *testing.T) {
		e := newEngine()
		var gotBoth string
		e.routes = []routeDef{{
			Method:     fasthttp.MethodPost,
			Path:       "/users/:id/disable",
			ParamNames: []string{"id"},
			HandlerValue: reflect.ValueOf(func(ctx ctxpkg.Context, id int64) (map[string]any, error) {
				gotBoth = ctx.Param("id")
				return map[string]any{"id": id}, nil
			}),
			AstMethod:  funcParams("ctx", "id"),
			ParamHints: map[string]ParamHint{"id": {Name: "id", Source: BindPath}},
		}}
		if err := e.registerRoutes(); err != nil {
			t.Fatalf("registerRoutes: %v", err)
		}

		status, _ := executeTestRequest(t, e, fasthttp.MethodPost, "/users/7/disable")
		if status != fasthttp.StatusOK {
			t.Fatalf("status = %d, want %d", status, fasthttp.StatusOK)
		}
		if gotBoth != "7" {
			t.Errorf(`ctx.Param("id") = %q, want "7"`, gotBoth)
		}
	})

	t.Run("静态路由不受影响", func(t *testing.T) {
		e := newEngine()
		e.routes = []routeDef{{
			Method: fasthttp.MethodGet,
			Path:   "/users/_meta",
			HandlerValue: reflect.ValueOf(func(ctx ctxpkg.Context) (map[string]any, error) {
				return map[string]any{"ok": true}, nil
			}),
			AstMethod:  funcParams("ctx"),
			ParamHints: map[string]ParamHint{},
		}}
		if err := e.registerRoutes(); err != nil {
			t.Fatalf("registerRoutes: %v", err)
		}

		status, body := executeTestRequest(t, e, fasthttp.MethodGet, "/users/_meta")
		if status != fasthttp.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", status, fasthttp.StatusOK, body)
		}
		if !strings.Contains(string(body), `"ok":true`) {
			t.Errorf("body = %s, want it to contain \"ok\":true", body)
		}
	})
}
