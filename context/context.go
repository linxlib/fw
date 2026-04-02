package context

import (
	"encoding/json"
	"fmt"

	"github.com/linxlib/fw/inject"
	"github.com/linxlib/fw/response"
	"github.com/valyala/fasthttp"
)

type Context interface {
	Raw() *fasthttp.RequestCtx
	Param(name string) string
	Query(name string) string
	BindJSON(out any) error
	Respond(statusCode int, code int, message string, data any) error
	Container() inject.Injector
	SetContainer(inject.Injector)
	SetRouteParams(map[string]string)
	TraceID() string
}

type FWContext struct {
	raw         *fasthttp.RequestCtx
	params      map[string]string
	container   inject.Injector
	responder   *response.Manager
	traceIDFunc func(*fasthttp.RequestCtx) string
}

func New(raw *fasthttp.RequestCtx, resp *response.Manager) *FWContext {
	return &FWContext{raw: raw, responder: resp, params: make(map[string]string)}
}

func (c *FWContext) Raw() *fasthttp.RequestCtx                           { return c.raw }
func (c *FWContext) Param(name string) string                            { return c.params[name] }
func (c *FWContext) Query(name string) string                            { return string(c.raw.QueryArgs().Peek(name)) }
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
