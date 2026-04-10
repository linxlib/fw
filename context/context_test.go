package context

import (
	"encoding/json"
	"testing"

	"github.com/linxlib/fw/v2/response"
	"github.com/valyala/fasthttp"
)

func TestStatusOKBuilderWritesEnvelope(t *testing.T) {
	ctx := newTestContext()

	err := ctx.StatusOK().Message("custom ok").Data(map[string]any{"name": "alice"})
	if err != nil {
		t.Fatalf("write response: %v", err)
	}

	if status := ctx.Raw().Response.StatusCode(); status != fasthttp.StatusOK {
		t.Fatalf("expected status %d, got %d", fasthttp.StatusOK, status)
	}
	payload := decodeTestBody(t, ctx.Raw().Response.Body())
	if payload["code"].(float64) != 0 {
		t.Fatalf("expected code 0, got %#v", payload["code"])
	}
	if payload["message"] != "custom ok" {
		t.Fatalf("expected message %q, got %#v", "custom ok", payload["message"])
	}
	data, ok := payload["data"].(map[string]any)
	if !ok || data["name"] != "alice" {
		t.Fatalf("expected data payload, got %#v", payload["data"])
	}
}

func TestStatusOKBuilderAcceptsInlineMessage(t *testing.T) {
	ctx := newTestContext()

	err := ctx.StatusOK("healthy").Send()
	if err != nil {
		t.Fatalf("write response: %v", err)
	}

	payload := decodeTestBody(t, ctx.Raw().Response.Body())
	if payload["message"] != "healthy" {
		t.Fatalf("expected message %q, got %#v", "healthy", payload["message"])
	}
	if _, ok := payload["data"]; ok {
		t.Fatalf("expected no data field, got %#v", payload["data"])
	}
}

func TestRawResponseBuilderWritesRawPayload(t *testing.T) {
	ctx := newTestContext()

	err := ctx.RawResponse().Data("ok")
	if err != nil {
		t.Fatalf("write raw response: %v", err)
	}

	if got := string(ctx.Raw().Response.Body()); got != `"ok"` {
		t.Fatalf("expected raw JSON string body, got %s", got)
	}
}

func TestStatusParamErrorBuilderUsesDefaults(t *testing.T) {
	ctx := newTestContext()

	err := ctx.StatusParamError("param xx invalid").Send()
	if err != nil {
		t.Fatalf("write response: %v", err)
	}

	if status := ctx.Raw().Response.StatusCode(); status != fasthttp.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", fasthttp.StatusBadRequest, status)
	}
	payload := decodeTestBody(t, ctx.Raw().Response.Body())
	if payload["code"].(float64) != 40001 {
		t.Fatalf("expected code 40001, got %#v", payload["code"])
	}
	if payload["message"] != "param xx invalid" {
		t.Fatalf("expected message %q, got %#v", "param xx invalid", payload["message"])
	}
}

func TestStatusServerErrorBuilderAllowsEmptyMessage(t *testing.T) {
	ctx := newTestContext()

	err := ctx.StatusServerError().Message("").Send()
	if err != nil {
		t.Fatalf("write response: %v", err)
	}

	if status := ctx.Raw().Response.StatusCode(); status != fasthttp.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", fasthttp.StatusInternalServerError, status)
	}
	payload := decodeTestBody(t, ctx.Raw().Response.Body())
	if payload["code"].(float64) != 50000 {
		t.Fatalf("expected code 50000, got %#v", payload["code"])
	}
	if payload["message"] != "" {
		t.Fatalf("expected empty message, got %#v", payload["message"])
	}
}

func newTestContext() *FWContext {
	var req fasthttp.Request
	req.Header.SetMethod(fasthttp.MethodGet)
	req.SetRequestURI("/test")

	var raw fasthttp.RequestCtx
	raw.Init(&req, nil, nil)

	return New(&raw, response.NewManager(nil, nil))
}

func decodeTestBody(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode response body: %v; body=%s", err, string(body))
	}
	return payload
}
