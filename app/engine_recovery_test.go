package app

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/fasthttp/router"
	"github.com/linxlib/fw/v2/astp"
	"github.com/linxlib/fw/v2/inject"
	"github.com/linxlib/fw/v2/logger"
	"github.com/linxlib/fw/v2/middleware"
	"github.com/linxlib/fw/v2/response"
	"github.com/valyala/fasthttp"
)

func TestRecoveryReturnsStackWhenEnabled(t *testing.T) {
	e := newRecoveryTestEngine(t, true)
	status, body := executeTestRequest(t, e, fasthttp.MethodGet, "/panic")
	if status != fasthttp.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", fasthttp.StatusInternalServerError, status)
	}

	payload := decodeJSONBody(t, body)
	if code := int(payload["code"].(float64)); code != 50000 {
		t.Fatalf("expected code 50000, got %d", code)
	}
	if payload["message"] != "internal error" {
		t.Fatalf("expected message %q, got %#v", "internal error", payload["message"])
	}

	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected response data when stack is enabled, got %#v", payload["data"])
	}
	if data["panic"] != "boom" {
		t.Fatalf("expected panic text %q, got %#v", "boom", data["panic"])
	}
	stack, ok := data["stack"].([]any)
	if !ok || len(stack) == 0 {
		t.Fatalf("expected non-empty stack payload, got %#v", data["stack"])
	}
	frame, ok := stack[0].(map[string]any)
	if !ok {
		t.Fatalf("expected structured stack frame, got %#v", stack[0])
	}
	if frame["func"] == "" || frame["file"] == "" {
		t.Fatalf("expected stack frame func/file, got %#v", frame)
	}
}

func TestRecoveryHidesStackWhenDisabled(t *testing.T) {
	e := newRecoveryTestEngine(t, false)
	status, body := executeTestRequest(t, e, fasthttp.MethodGet, "/panic")
	if status != fasthttp.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", fasthttp.StatusInternalServerError, status)
	}

	payload := decodeJSONBody(t, body)
	if _, exists := payload["data"]; exists {
		t.Fatalf("expected no response data when stack is disabled, got %#v", payload["data"])
	}
}

func TestRecoveryCanBeForceDisabled(t *testing.T) {
	e := newRecoveryTestEngine(t, true)
	e.cfg.Recovery.Enabled = false

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected panic to escape when recovery is disabled")
		}
		if recovered != "boom" {
			t.Fatalf("expected panic %q, got %#v", "boom", recovered)
		}
	}()

	_, _ = executeTestRequest(t, e, fasthttp.MethodGet, "/panic")
}

func TestRequestLoggingCanBeDisabled(t *testing.T) {
	e := newRecoveryTestEngine(t, true)
	e.routes = []routeDef{{
		Method:       fasthttp.MethodGet,
		Path:         "/ok",
		HandlerValue: reflect.ValueOf(func() {}),
		AstMethod:    &astp.Func{},
		ParamHints:   map[string]ParamHint{},
	}}
	e.router = router.New()
	if err := e.registerRoutes(); err != nil {
		t.Fatalf("register routes: %v", err)
	}

	buf := &bytes.Buffer{}
	e.log.SetConsoleWriter(buf)

	e.cfg.Log.RequestEnabled = false
	status, _ := executeTestRequest(t, e, fasthttp.MethodGet, "/ok")
	if status != fasthttp.StatusOK {
		t.Fatalf("expected status %d, got %d", fasthttp.StatusOK, status)
	}
	if strings.Contains(buf.String(), "GET /ok ->") {
		t.Fatalf("expected request log to be disabled, got %q", buf.String())
	}

	buf.Reset()
	e.cfg.Log.RequestEnabled = true
	status, _ = executeTestRequest(t, e, fasthttp.MethodGet, "/ok")
	if status != fasthttp.StatusOK {
		t.Fatalf("expected status %d, got %d", fasthttp.StatusOK, status)
	}
	if !strings.Contains(buf.String(), "GET /ok -> 200") {
		t.Fatalf("expected request log when enabled, got %q", buf.String())
	}
}

func newRecoveryTestEngine(t *testing.T, returnStack bool) *Engine {
	t.Helper()

	logg, err := logger.New("info", "console", "", false)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	cfg := DefaultEngineConfig()
	cfg.Recovery.Enabled = true
	cfg.Recovery.ReturnStackToBody = returnStack

	e := &Engine{
		cfg:             &cfg,
		log:             logg,
		router:          router.New(),
		globalContainer: inject.New(),
		responseManager: response.NewManager(nil, nil),
		controllers:     make(map[string]reflect.Value),
		middlewares:     make(map[string]middleware.Middleware),
	}
	e.routes = []routeDef{{
		Method:       fasthttp.MethodGet,
		Path:         "/panic",
		HandlerValue: reflect.ValueOf(func() { panic("boom") }),
		AstMethod:    &astp.Func{},
		ParamHints:   map[string]ParamHint{},
	}}
	if err := e.registerRoutes(); err != nil {
		t.Fatalf("register routes: %v", err)
	}
	return e
}

func executeTestRequest(t *testing.T, e *Engine, method string, path string) (int, []byte) {
	t.Helper()

	var req fasthttp.Request
	req.Header.SetMethod(method)
	req.SetRequestURI(path)

	var ctx fasthttp.RequestCtx
	ctx.Init(&req, nil, nil)
	e.router.Handler(&ctx)

	return ctx.Response.StatusCode(), append([]byte(nil), ctx.Response.Body()...)
}

func decodeJSONBody(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode response body: %v; body=%s", err, string(body))
	}
	return payload
}
