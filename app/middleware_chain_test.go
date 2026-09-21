package app

import (
	"reflect"
	"sync/atomic"
	"testing"

	ctxpkg "github.com/linxlib/fw/v2/context"
	"github.com/linxlib/fw/v2/middleware"
	"github.com/linxlib/fw/v2/response"
	"github.com/valyala/fasthttp"
)

// countingMiddleware 统计自己被调用了多少次, 可选记录进入顺序.
type countingMiddleware struct {
	name  string
	count *int32
	trace *[]string // 可选: 记录进入顺序, 用于校验洋葱序
}

func (m countingMiddleware) Spec() middleware.AnnotationSpec {
	return middleware.AnnotationSpec{
		Name:  m.name,
		Scope: middleware.ScopeBoth,
		Stage: middleware.StageBoth,
	}
}

func (m countingMiddleware) Handle(ctx ctxpkg.Context, _ middleware.AnnotationArgs, next middleware.Handler) error {
	atomic.AddInt32(m.count, 1)
	if m.trace != nil {
		*m.trace = append(*m.trace, m.name)
	}
	return next(ctx)
}

func newCountingMiddleware(name string) (countingMiddleware, *int32) {
	var n int32
	return countingMiddleware{name: name, count: &n}, &n
}

// newMiddlewareTestContext 构造一个可用的请求上下文.
func newMiddlewareTestContext() ctxpkg.Context {
	var req fasthttp.Request
	req.SetRequestURI("/users")
	req.Header.SetMethod(fasthttp.MethodGet)
	var raw fasthttp.RequestCtx
	raw.Init(&req, nil, nil)
	return ctxpkg.New(&raw, response.NewManager(nil, nil))
}

// TestChainDeduplicatesAcrossLayers 保证同一中间件跨层绑定时只执行一次.
//
// 回归背景: Chain 曾把 global/controller/method 三层直接拼接, 于是一个既被 Engine.Use()
// 注册为全局、又在控制器上标了 @Log 的中间件会跑两遍, 日志 -->/<-- 各打两条.
func TestChainDeduplicatesAcrossLayers(t *testing.T) {
	mw, count := newCountingMiddleware("Log")

	h := middleware.Chain(
		func(ctx ctxpkg.Context) error { return nil },
		[]middleware.Bound{{MW: mw}}, // global
		[]middleware.Bound{{MW: mw}}, // controller
		[]middleware.Bound{{MW: mw}}, // method
	)
	if err := h(newMiddlewareTestContext()); err != nil {
		t.Fatalf("chain returned error: %v", err)
	}
	if got := atomic.LoadInt32(count); got != 1 {
		t.Fatalf("middleware invoked %d times, want 1 (deduplicated across layers)", got)
	}
}

// TestChainKeepsDistinctMiddlewares 保证不同中间件仍然全部保留, 且洋葱序不变:
// 全局 -> 控制器 -> 方法 -> handler.
func TestChainKeepsDistinctMiddlewares(t *testing.T) {
	var order []string

	mk := func(name string) (countingMiddleware, *int32) {
		var n int32
		return countingMiddleware{name: name, count: &n, trace: &order}, &n
	}

	a, aCount := mk("A") // 全局
	b, bCount := mk("B") // 控制器
	c, cCount := mk("C") // 方法

	h := middleware.Chain(
		func(ctx ctxpkg.Context) error {
			order = append(order, "handler")
			return nil
		},
		[]middleware.Bound{{MW: a}},
		[]middleware.Bound{{MW: b}},
		[]middleware.Bound{{MW: c}},
	)
	if err := h(newMiddlewareTestContext()); err != nil {
		t.Fatalf("chain returned error: %v", err)
	}
	for _, n := range []struct {
		name string
		c    *int32
	}{{"A", aCount}, {"B", bCount}, {"C", cCount}} {
		if got := atomic.LoadInt32(n.c); got != 1 {
			t.Errorf("middleware %s invoked %d times, want 1", n.name, got)
		}
	}
	want := []string{"A", "B", "C", "handler"}
	if len(order) != len(want) {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("execution order = %v, want %v", order, want)
		}
	}
}

// TestChainKeepsMostSpecificBinding 保证去重后保留的是最具体那一层(方法 > 控制器 > 全局).
func TestChainKeepsMostSpecificBinding(t *testing.T) {
	mwGlobal, globalCount := newCountingMiddleware("Log")
	mwMethod, methodCount := newCountingMiddleware("Log")

	// 同名的两个不同实现, 分别绑在全局与方法层
	h := middleware.Chain(
		func(ctx ctxpkg.Context) error { return nil },
		[]middleware.Bound{{MW: mwGlobal}},
		nil,
		[]middleware.Bound{{MW: mwMethod}},
	)
	if err := h(newMiddlewareTestContext()); err != nil {
		t.Fatalf("chain returned error: %v", err)
	}
	if got := atomic.LoadInt32(globalCount); got != 0 {
		t.Errorf("global binding invoked %d times, want 0 (shadowed by method level)", got)
	}
	if got := atomic.LoadInt32(methodCount); got != 1 {
		t.Errorf("method binding invoked %d times, want 1", got)
	}
}

// TestEngineDeduplicatesGlobalAndControllerMiddleware 从引擎层面验证:
// 全局中间件 + 控制器注解同名时, 一次请求只跑一遍.
func TestEngineDeduplicatesGlobalAndControllerMiddleware(t *testing.T) {
	e := newBindingTestEngine(t)
	mw, count := newCountingMiddleware("Log")

	var ran bool
	e.routes = []routeDef{{
		Method:     fasthttp.MethodGet,
		Path:       "/users",
		ParamNames: nil,
		HandlerValue: reflect.ValueOf(func(ctx ctxpkg.Context) (map[string]any, error) {
			ran = true
			return map[string]any{"ok": true}, nil
		}),
		AstMethod:  funcParamsOf("ctx"),
		ParamHints: map[string]ParamHint{},
		GlobalMW:   []middleware.Bound{{MW: mw}}, // e.Use() 注册的全局中间件
		CtrlMW:     []middleware.Bound{{MW: mw}}, // 控制器上的 @Log
	}}
	if err := e.registerRoutes(); err != nil {
		t.Fatalf("registerRoutes: %v", err)
	}

	status, _ := executeTestRequest(t, e, fasthttp.MethodGet, "/users")
	if status != fasthttp.StatusOK {
		t.Fatalf("status = %d, want %d", status, fasthttp.StatusOK)
	}
	if !ran {
		t.Fatal("handler was not invoked")
	}
	if got := atomic.LoadInt32(count); got != 1 {
		t.Fatalf("middleware invoked %d times, want 1", got)
	}
}
