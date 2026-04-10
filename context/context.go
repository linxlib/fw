package context

import (
	"bufio"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/linxlib/fw/v2/inject"
	"github.com/linxlib/fw/v2/response"
	"github.com/valyala/fasthttp"
)

type Context interface {
	Raw() *fasthttp.RequestCtx
	Method() string
	Scheme() string
	Host() string
	Path() string
	URI() string
	Header(name string) string
	Headers() map[string]string
	Param(name string) string
	Params() map[string]string
	Query(name string) string
	Queries() map[string]string
	Body() []byte
	BindJSON(out any) error
	Respond(statusCode int, code int, message string, data any) error
	RawResponse() ResponseBuilder
	StatusOK(message ...string) ResponseBuilder
	StatusParamError(message ...string) ResponseBuilder
	StatusServerError(message ...string) ResponseBuilder
	Container() inject.Injector
	SetContainer(inject.Injector)
	SetRouteParams(map[string]string)
	TraceID() string

	// Set stores a key-value pair in the request-scoped context.
	// Typically used by middleware to pass data (e.g. user info) to handlers or later middleware.
	Set(key string, value any)
	// Get retrieves a value by key from the request-scoped context.
	// Returns the value and whether the key was found.
	Get(key string) (any, bool)
	// MustGet retrieves a value by key, panics if the key does not exist.
	MustGet(key string) any
	// GetString is a convenience method that returns the value as a string.
	// Returns empty string if the key is missing or the value is not a string.
	GetString(key string) string
	// GetInt is a convenience method that returns the value as an int.
	// Returns 0 if the key is missing or the value is not an int.
	GetInt(key string) int

	// SSE starts a Server-Sent Events stream.
	// It sets the appropriate headers (Content-Type, Cache-Control, Connection) and
	// returns an SSEWriter that can be used to send events to the client.
	// The provided callback receives the writer; when the callback returns, the stream ends.
	SSE(fn func(w *SSEWriter)) error
}

type ResponseBuilder interface {
	Message(message string) ResponseBuilder
	Code(code int) ResponseBuilder
	Data(data any) error
	Send() error
}

type FWContext struct {
	raw         *fasthttp.RequestCtx
	params      map[string]string
	store       map[string]any
	container   inject.Injector
	responder   *response.Manager
	traceIDFunc func(*fasthttp.RequestCtx) string
}

func New(raw *fasthttp.RequestCtx, resp *response.Manager) *FWContext {
	return &FWContext{raw: raw, responder: resp, params: make(map[string]string), store: make(map[string]any)}
}

func (c *FWContext) Raw() *fasthttp.RequestCtx                           { return c.raw }
func (c *FWContext) Method() string                                      { return string(c.raw.Method()) }
func (c *FWContext) Scheme() string                                      { return string(c.raw.URI().Scheme()) }
func (c *FWContext) Host() string                                        { return string(c.raw.Host()) }
func (c *FWContext) Path() string                                        { return string(c.raw.Path()) }
func (c *FWContext) URI() string                                         { return string(c.raw.RequestURI()) }
func (c *FWContext) Header(name string) string                           { return string(c.raw.Request.Header.Peek(name)) }
func (c *FWContext) Param(name string) string                            { return c.params[name] }
func (c *FWContext) Params() map[string]string                           { return cloneStringMap(c.params) }
func (c *FWContext) Query(name string) string                            { return string(c.raw.QueryArgs().Peek(name)) }
func (c *FWContext) Queries() map[string]string                          { return collectArgs(c.raw.QueryArgs()) }
func (c *FWContext) Headers() map[string]string                          { return collectHeaders(&c.raw.Request.Header) }
func (c *FWContext) Body() []byte                                        { return append([]byte(nil), c.raw.PostBody()...) }
func (c *FWContext) Container() inject.Injector                          { return c.container }
func (c *FWContext) SetContainer(i inject.Injector)                      { c.container = i }
func (c *FWContext) SetRouteParams(p map[string]string)                  { c.params = p }
func (c *FWContext) SetTraceIDFunc(fn func(*fasthttp.RequestCtx) string) { c.traceIDFunc = fn }

func (c *FWContext) BindJSON(out any) error {
	body := c.raw.PostBody()
	if len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("parse json body: %w", err)
	}
	return nil
}

func (c *FWContext) TraceID() string {
	if c.traceIDFunc != nil {
		return c.traceIDFunc(c.raw)
	}
	return string(c.raw.Response.Header.Peek("X-Trace-Id"))
}

func (c *FWContext) Respond(statusCode int, code int, message string, data any) error {
	return c.responder.Write(c.raw, statusCode, code, message, data, c.TraceID())
}

func (c *FWContext) RawResponse() ResponseBuilder {
	return &responseBuilder{ctx: c, statusCode: fasthttp.StatusOK, raw: true}
}

func (c *FWContext) StatusOK(message ...string) ResponseBuilder {
	return c.newResponseBuilder(fasthttp.StatusOK, 0, "ok", message...)
}

func (c *FWContext) StatusParamError(message ...string) ResponseBuilder {
	return c.newResponseBuilder(fasthttp.StatusBadRequest, 40001, "invalid request", message...)
}

func (c *FWContext) StatusServerError(message ...string) ResponseBuilder {
	return c.newResponseBuilder(fasthttp.StatusInternalServerError, 50000, "internal error", message...)
}

func (c *FWContext) newResponseBuilder(statusCode int, code int, defaultMessage string, message ...string) ResponseBuilder {
	b := &responseBuilder{ctx: c, statusCode: statusCode, code: code, message: defaultMessage}
	if len(message) > 0 {
		b.message = message[0]
	}
	return b
}

func (c *FWContext) Set(key string, value any) { c.store[key] = value }

type responseBuilder struct {
	ctx        *FWContext
	statusCode int
	code       int
	message    string
	raw        bool
}

func (b *responseBuilder) Message(message string) ResponseBuilder {
	b.message = message
	return b
}

func (b *responseBuilder) Code(code int) ResponseBuilder {
	b.code = code
	return b
}

func (b *responseBuilder) Data(data any) error {
	if b.raw {
		return b.ctx.responder.WriteRaw(b.ctx.raw, b.statusCode, data)
	}
	return b.ctx.responder.Write(b.ctx.raw, b.statusCode, b.code, b.message, data, b.ctx.TraceID())
}

func (b *responseBuilder) Send() error {
	if b.raw {
		return b.ctx.responder.WriteRaw(b.ctx.raw, b.statusCode, nil)
	}
	return b.ctx.responder.Write(b.ctx.raw, b.statusCode, b.code, b.message, nil, b.ctx.TraceID())
}

func (c *FWContext) Get(key string) (any, bool) {
	v, ok := c.store[key]
	return v, ok
}

func (c *FWContext) MustGet(key string) any {
	v, ok := c.store[key]
	if !ok {
		panic(fmt.Sprintf("context: key %q does not exist", key))
	}
	return v
}

func (c *FWContext) GetString(key string) string {
	v, ok := c.store[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func (c *FWContext) GetInt(key string) int {
	v, ok := c.store[key]
	if !ok {
		return 0
	}
	i, _ := v.(int)
	return i
}

// SSE starts a Server-Sent Events stream. It sets appropriate headers and invokes fn
// with an SSEWriter. The stream is closed when fn returns.
//
// Usage:
//
//	ctx.SSE(func(w *SSEWriter) {
//	    w.WriteText("hello")
//	    w.WriteJSON(map[string]any{"count": 1})
//	    w.WriteEvent("update", "payload data")
//	})
func (c *FWContext) SSE(fn func(w *SSEWriter)) error {
	c.raw.Response.Header.Set("Content-Type", "text/event-stream")
	c.raw.Response.Header.Set("Cache-Control", "no-cache")
	c.raw.Response.Header.Set("Connection", "keep-alive")
	c.raw.Response.Header.Set("X-Accel-Buffering", "no")
	c.raw.SetStatusCode(fasthttp.StatusOK)

	c.raw.SetBodyStreamWriter(func(w *bufio.Writer) {
		sw := &SSEWriter{w: w}
		fn(sw)
	})
	return nil
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func collectArgs(args *fasthttp.Args) map[string]string {
	result := make(map[string]string)
	args.VisitAll(func(key, value []byte) {
		result[string(key)] = string(value)
	})
	return result
}

func collectHeaders(header *fasthttp.RequestHeader) map[string]string {
	result := make(map[string]string)
	header.VisitAll(func(key, value []byte) {
		result[string(key)] = string(value)
	})
	return result
}

// SSEWriter writes Server-Sent Events to an underlying buffered writer.
// Each method writes a complete SSE frame and flushes immediately.
type SSEWriter struct {
	w  *bufio.Writer
	mu sync.Mutex
}

// WriteText sends a data-only SSE event with plain text content.
//
//	data: hello world\n\n
func (s *SSEWriter) WriteText(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", text); err != nil {
		return err
	}
	return s.w.Flush()
}

// WriteJSON sends a data-only SSE event with JSON-encoded content.
//
//	data: {"key":"value"}\n\n
func (s *SSEWriter) WriteJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", b); err != nil {
		return err
	}
	return s.w.Flush()
}

// WriteEvent sends a named SSE event with text data.
//
//	event: <event>\ndata: <data>\n\n
func (s *SSEWriter) WriteEvent(event string, data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	return s.w.Flush()
}

// WriteEventJSON sends a named SSE event with JSON-encoded data.
//
//	event: <event>\ndata: <json>\n\n
func (s *SSEWriter) WriteEventJSON(event string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, b); err != nil {
		return err
	}
	return s.w.Flush()
}

// WriteID sends an SSE event with id, event name, and text data.
//
//	id: <id>\nevent: <event>\ndata: <data>\n\n
func (s *SSEWriter) WriteID(id string, event string, data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := fmt.Fprintf(s.w, "id: %s\nevent: %s\ndata: %s\n\n", id, event, data); err != nil {
		return err
	}
	return s.w.Flush()
}
