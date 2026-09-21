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

func TestJoinPath(t *testing.T) {
	tests := []struct {
		base string
		sub  string
		want string
	}{
		// 指向控制器根路径的注解收敛为不带尾斜杠的集合路径
		{"/users", "/", "/users"},
		{"/users/", "/", "/users"},
		{"/api/v1", "/", "/api/v1"},
		// 常规拼接不受影响
		{"/users", "/:id", "/users/:id"},
		{"/users", "/:id/disable", "/users/:id/disable"},
		{"/api", "/hello", "/api/hello"},
		// 根控制器
		{"/", "/", "/"},
		{"/", "/hello", "/hello"},
	}
	for _, tt := range tests {
		if got := joinPath(tt.base, tt.sub); got != tt.want {
			t.Errorf("joinPath(%q, %q) = %q, want %q", tt.base, tt.sub, got, tt.want)
		}
	}
}

// newBindingTestEngine 构造一个只带路由表的引擎, 用于直接驱动 router.Handler.
func newBindingTestEngine(t *testing.T) *Engine {
	t.Helper()
	logg, err := logger.New("info", "console", "", false)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	return &Engine{
		cfg:             ptrEngineConfig(DefaultEngineConfig()),
		log:             logg,
		router:          router.New(),
		globalContainer: inject.New(),
		responseManager: response.NewManager(nil, nil),
		controllers:     make(map[string]reflect.Value),
		middlewares:     make(map[string]middleware.Middleware),
	}
}

func funcParamsOf(names ...string) *astp.Func {
	fn := &astp.Func{Name: "handler"}
	for _, n := range names {
		fn.Params = append(fn.Params, &astp.Param{Name: n, Type: &astp.TypeRef{Name: "int", Kind: astp.KindBasic}})
	}
	return fn
}

// TestQueryParamBinding 覆盖分页接口的两个真实缺陷:
//
//  1. 同类型形参串值: 解析器曾把结果按类型写回 container, 导致 List(ctx, page, size)
//     里的 size 拿到 page 的值;
//  2. 缺省查询参数直接 500: 不传 page/size 时 "Value not found for type int".
func TestQueryParamBinding(t *testing.T) {
	t.Run("同类型形参各自取值", func(t *testing.T) {
		e := newBindingTestEngine(t)
		e.routes = []routeDef{{
			Method: fasthttp.MethodGet,
			Path:   "/users",
			HandlerValue: reflect.ValueOf(func(ctx ctxpkg.Context, page int, size int) (map[string]any, error) {
				return map[string]any{"page": page, "size": size}, nil
			}),
			AstMethod: funcParamsOf("ctx", "page", "size"),
			ParamHints: map[string]ParamHint{
				"page": {Name: "page", Source: BindQuery},
				"size": {Name: "size", Source: BindQuery},
			},
		}}
		if err := e.registerRoutes(); err != nil {
			t.Fatalf("registerRoutes: %v", err)
		}

		status, body := executeTestRequest(t, e, fasthttp.MethodGet, "/users?page=2&size=5")
		if status != fasthttp.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", status, fasthttp.StatusOK, body)
		}
		if !strings.Contains(string(body), `"page":2`) || !strings.Contains(string(body), `"size":5`) {
			t.Fatalf("body = %s, want page=2 and size=5", body)
		}
	})

	t.Run("缺省查询参数回退零值", func(t *testing.T) {
		e := newBindingTestEngine(t)
		e.routes = []routeDef{{
			Method: fasthttp.MethodGet,
			Path:   "/users",
			HandlerValue: reflect.ValueOf(func(ctx ctxpkg.Context, page int, size int) (map[string]any, error) {
				return map[string]any{"page": page, "size": size}, nil
			}),
			AstMethod: funcParamsOf("ctx", "page", "size"),
			ParamHints: map[string]ParamHint{
				"page": {Name: "page", Source: BindQuery},
				"size": {Name: "size", Source: BindQuery},
			},
		}}
		if err := e.registerRoutes(); err != nil {
			t.Fatalf("registerRoutes: %v", err)
		}

		status, body := executeTestRequest(t, e, fasthttp.MethodGet, "/users")
		if status != fasthttp.StatusOK {
			t.Fatalf("status = %d, want %d (missing query params must not fail); body=%s",
				status, fasthttp.StatusOK, body)
		}
		if !strings.Contains(string(body), `"page":0`) || !strings.Contains(string(body), `"size":0`) {
			t.Fatalf("body = %s, want page=0 and size=0", body)
		}
	})

	t.Run("路径参数缺失仍然响亮失败", func(t *testing.T) {
		e := newBindingTestEngine(t)
		e.routes = []routeDef{{
			Method: fasthttp.MethodGet,
			Path:   "/users/:id",
			HandlerValue: reflect.ValueOf(func(ctx ctxpkg.Context, id int64) (map[string]any, error) {
				return map[string]any{"id": id}, nil
			}),
			// 形参名和路由占位符不一致: userID vs id
			AstMethod: funcParamsOf("ctx", "userID"),
			ParamHints: map[string]ParamHint{
				"userID": {Name: "userID", Source: BindPath},
			},
			ParamNames: []string{"id"},
		}}
		if err := e.registerRoutes(); err != nil {
			t.Fatalf("registerRoutes: %v", err)
		}

		status, _ := executeTestRequest(t, e, fasthttp.MethodGet, "/users/42")
		if status != fasthttp.StatusInternalServerError {
			t.Fatalf("status = %d, want %d (mismatched path param name must stay loud)",
				status, fasthttp.StatusInternalServerError)
		}
	})
}

// TestCollectionRouteHasNoTrailingSlash 保证集合路由注册为 /users 而不是 /users/,
// 否则 GET /users 会被 301 弹到 /users/, 和 /users/{id} 的形态也容易混淆.
func TestCollectionRouteHasNoTrailingSlash(t *testing.T) {
	basePath := "/users"
	full := joinPath(basePath, "/")
	if full != "/users" {
		t.Fatalf("collection route = %q, want /users", full)
	}
	if got := routerPath(full); got != "/users" {
		t.Fatalf("routerPath(%q) = %q, want /users", full, got)
	}
}
