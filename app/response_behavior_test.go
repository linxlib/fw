package app

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/fasthttp/router"
	"github.com/linxlib/fw/v2/astp"
	"github.com/linxlib/fw/v2/config"
	"github.com/linxlib/fw/v2/inject"
	"github.com/linxlib/fw/v2/logger"
	"github.com/linxlib/fw/v2/middleware"
	"github.com/linxlib/fw/v2/response"
	"github.com/valyala/fasthttp"
)

func TestHandlerReturnValueUsesEnvelopeByDefault(t *testing.T) {
	e := newResponseTestEngine(t, reflect.ValueOf(func() map[string]any {
		return map[string]any{"name": "alice"}
	}), &astp.Func{Results: []*astp.Param{{Type: &astp.TypeRef{Name: "map", Kind: astp.KindMap}}}})

	status, body := executeTestRequest(t, e, fasthttp.MethodGet, "/test")
	if status != fasthttp.StatusOK {
		t.Fatalf("expected status %d, got %d", fasthttp.StatusOK, status)
	}
	payload := decodeJSONBody(t, body)
	if payload["message"] != "ok" {
		t.Fatalf("expected wrapped response message, got %#v", payload["message"])
	}
	data, ok := payload["data"].(map[string]any)
	if !ok || data["name"] != "alice" {
		t.Fatalf("expected wrapped data, got %#v", payload["data"])
	}
}

func TestHandlerReturnValueCanSkipEnvelopeWithRawResponse(t *testing.T) {
	e := newResponseTestEngine(t, reflect.ValueOf(func() map[string]any {
		return map[string]any{"name": "alice"}
	}), &astp.Func{
		Doc:     &astp.CommentGroup{ParsedAnnotations: []*astp.Annotation{{Name: "RawResponse"}}},
		Results: []*astp.Param{{Type: &astp.TypeRef{Name: "map", Kind: astp.KindMap}}},
	})

	status, body := executeTestRequest(t, e, fasthttp.MethodGet, "/test")
	if status != fasthttp.StatusOK {
		t.Fatalf("expected status %d, got %d", fasthttp.StatusOK, status)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode raw response body: %v; body=%s", err, string(body))
	}
	if payload["name"] != "alice" {
		t.Fatalf("expected raw response payload, got %#v", payload)
	}
	if _, ok := payload["code"]; ok {
		t.Fatalf("expected raw response without envelope, got %#v", payload)
	}
}

func TestHandlerReturnValueErrorStopsSuccessWrite(t *testing.T) {
	e := newResponseTestEngine(t, reflect.ValueOf(func() (map[string]any, error) {
		return nil, assertErr("boom")
	}), &astp.Func{Results: []*astp.Param{{Type: &astp.TypeRef{Name: "map", Kind: astp.KindMap}}, {Type: &astp.TypeRef{Name: "error"}}}})

	status, body := executeTestRequest(t, e, fasthttp.MethodGet, "/test")
	if status != fasthttp.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", fasthttp.StatusInternalServerError, status)
	}
	payload := decodeJSONBody(t, body)
	if payload["message"] != "internal error" {
		t.Fatalf("expected internal error response, got %#v", payload)
	}
}

func TestResponseSchemaForRawResponseSkipsEnvelope(t *testing.T) {
	method := &astp.Func{
		Doc:     &astp.CommentGroup{ParsedAnnotations: []*astp.Annotation{{Name: "RawResponse"}}},
		Results: []*astp.Param{{Type: &astp.TypeRef{Name: "string"}}},
	}
	schema := responseSchemaForMethod(nil, method)
	if schema == nil {
		t.Fatal("expected schema")
	}
	if schema.Type != "string" {
		t.Fatalf("expected raw schema type string, got %#v", schema)
	}
	if _, ok := schema.Properties["code"]; ok {
		t.Fatalf("expected raw schema without envelope, got %#v", schema)
	}
}

func TestManualResponsePreventsAutomaticRewrite(t *testing.T) {
	e := newResponseTestEngine(t, reflect.ValueOf(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusCreated)
		ctx.Response.SetBodyString(`{"created":true}`)
		ctx.Response.Header.SetContentType("application/json")
	}), &astp.Func{Params: []*astp.Param{{Name: "ctx", Type: &astp.TypeRef{Name: "RequestCtx", PkgPath: "github.com/valyala/fasthttp"}}}})

	status, body := executeTestRequest(t, e, fasthttp.MethodGet, "/test")
	if status != fasthttp.StatusCreated {
		t.Fatalf("expected status %d, got %d", fasthttp.StatusCreated, status)
	}
	if string(body) != `{"created":true}` {
		t.Fatalf("expected manual response body, got %s", string(body))
	}
}

func newResponseTestEngine(t *testing.T, handler reflect.Value, method *astp.Func) *Engine {
	t.Helper()

	logg, err := logger.New("info", "console", "", false)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	e := &Engine{
		cfg:             config.Default(),
		log:             logg,
		router:          router.New(),
		globalContainer: inject.New(),
		responseManager: response.NewManager(nil, nil),
		controllers:     make(map[string]reflect.Value),
		middlewares:     make(map[string]middleware.Middleware),
	}
	e.routes = []routeDef{{
		Method:       fasthttp.MethodGet,
		Path:         "/test",
		HandlerValue: handler,
		AstMethod:    method,
		ParamHints:   map[string]ParamHint{},
	}}
	if err := e.registerRoutes(); err != nil {
		t.Fatalf("register routes: %v", err)
	}
	return e
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
