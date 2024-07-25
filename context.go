package fw

import (
	"errors"
	"github.com/linxlib/fw/render"
	"github.com/linxlib/inject"
	"github.com/valyala/fasthttp"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Context struct {
	ctx  *fasthttp.RequestCtx
	inj  inject.Injector
	mu   sync.RWMutex
	Keys map[string]any

	Errors     errorMsgs
	ErrHandler func(*Context, error)
}

func newContext(ctx *fasthttp.RequestCtx, parent ...inject.Injector) *Context {
	cc := &Context{
		ctx: ctx,
		inj: inject.New(),
	}
	if len(parent) > 0 {
		cc.inj.SetParent(parent[0])
	}
	err := cc.inj.Apply(cc)
	if err != nil {
		panic(err)
	}
	return cc
}
func (c *Context) Map(i ...interface{}) inject.TypeMapper {
	return c.inj.Map(i...)
}
func (c *Context) MapTo(i interface{}, j interface{}) inject.TypeMapper {
	return c.inj.MapTo(i, j)
}
func (c *Context) Apply(ctl interface{}) error {
	return c.inj.Apply(ctl)
}
func (c *Context) Provide(i interface{}) error {
	return c.inj.Provide(i)
}

func (c *Context) RemoteIP() string {
	return c.ctx.RemoteIP().String()
}

func (c *Context) GetFastContext() *fasthttp.RequestCtx {
	return c.ctx
}

func (c *Context) Injector() inject.Injector {
	return c.inj
}

func (c *Context) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Keys == nil {
		c.Keys = make(map[string]any)
	}
	c.Keys[key] = value
}
func (c *Context) Get(key string) (value any, exists bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists = c.Keys[key]
	return
}
func (c *Context) MustGet(key string) any {
	if value, exists := c.Get(key); exists {
		return value
	}
	panic("Key \"" + key + "\" does not exist")
}
func (c *Context) GetString(key string) (s string) {
	if val, ok := c.Get(key); ok && val != nil {
		s, _ = val.(string)
	}
	return
}
func (c *Context) GetBool(key string) (b bool) {
	if val, ok := c.Get(key); ok && val != nil {
		b, _ = val.(bool)
	}
	return
}
func (c *Context) GetInt(key string) (i int) {
	if val, ok := c.Get(key); ok && val != nil {
		i, _ = val.(int)
	}
	return
}
func (c *Context) GetInt64(key string) (i64 int64) {
	if val, ok := c.Get(key); ok && val != nil {
		i64, _ = val.(int64)
	}
	return
}
func (c *Context) GetUint(key string) (ui uint) {
	if val, ok := c.Get(key); ok && val != nil {
		ui, _ = val.(uint)
	}
	return
}
func (c *Context) GetUint64(key string) (ui64 uint64) {
	if val, ok := c.Get(key); ok && val != nil {
		ui64, _ = val.(uint64)
	}
	return
}
func (c *Context) GetFloat64(key string) (f64 float64) {
	if val, ok := c.Get(key); ok && val != nil {
		f64, _ = val.(float64)
	}
	return
}
func (c *Context) GetTime(key string) (t time.Time) {
	if val, ok := c.Get(key); ok && val != nil {
		t, _ = val.(time.Time)
	}
	return
}
func (c *Context) GetDuration(key string) (d time.Duration) {
	if val, ok := c.Get(key); ok && val != nil {
		d, _ = val.(time.Duration)
	}
	return
}
func (c *Context) GetStringSlice(key string) (ss []string) {
	if val, ok := c.Get(key); ok && val != nil {
		ss, _ = val.([]string)
	}
	return
}
func (c *Context) GetStringMap(key string) (sm map[string]any) {
	if val, ok := c.Get(key); ok && val != nil {
		sm, _ = val.(map[string]any)
	}
	return
}
func (c *Context) GetStringMapString(key string) (sms map[string]string) {
	if val, ok := c.Get(key); ok && val != nil {
		sms, _ = val.(map[string]string)
	}
	return
}
func (c *Context) GetStringMapStringSlice(key string) (smss map[string][]string) {
	if val, ok := c.Get(key); ok && val != nil {
		smss, _ = val.(map[string][]string)
	}
	return
}

func (c *Context) Param(key string) string {
	return c.ctx.UserValue([]byte(key)).(string)
}

// TODO: 完善Context所提供的方法
func (c *Context) Status(code int) {
	c.ctx.SetStatusCode(code)
}
func bodyAllowedForStatus(status int) bool {
	switch {
	case status >= 100 && status <= 199:
		return false
	case status == http.StatusNoContent:
		return false
	case status == http.StatusNotModified:
		return false
	}
	return true
}
func (c *Context) Render(code int, r render.IRender) {
	c.Status(code)

	if !bodyAllowedForStatus(code) {
		r.WriteContentType(c.ctx)
		return
	}

	if err := r.Render(c.ctx); err != nil {
		// Pushing error to c.Errors
		_ = c.Error(err)
		//c.Abort()
	}
}
func (c *Context) Error(err error) *Error {
	if err == nil {
		panic("err is nil")
	}

	var parsedError *Error
	ok := errors.As(err, &parsedError)
	if !ok {
		parsedError = &Error{
			Err:  err,
			Type: ErrorTypePrivate,
		}
	}

	c.Errors = append(c.Errors, parsedError)
	return parsedError
}

func (c *Context) JSON(code int, obj any) {
	c.Render(code, render.JSON{Data: obj})
}
func (c *Context) AsciiJSON(code int, obj any) {
	c.Render(code, render.AsciiJSON{Data: obj})
}
func (c *Context) PureJSON(code int, obj any) {
	c.Render(code, render.PureJSON{Data: obj})
}
func (c *Context) XML(code int, obj any) {
	c.Render(code, render.XML{Data: obj})
}
func (c *Context) String(code int, format string, values ...any) {
	c.Render(code, render.String{Format: format, Data: values})
}
func (c *Context) Redirect(code int, location string) {
	c.Render(-1, render.Redirect{
		Code:     code,
		Location: location,
	})
}
func (c *Context) Data(code int, contentType string, data []byte) {
	c.Render(code, render.Data{
		ContentType: contentType,
		Data:        data,
	})
}
func (c *Context) DataFromReader(code int, contentLength int64, contentType string, reader io.Reader, extraHeaders map[string]string) {
	c.Render(code, render.Reader{
		Headers:       extraHeaders,
		ContentType:   contentType,
		ContentLength: contentLength,
		Reader:        reader,
	})
}
func (c *Context) File(filepath string) {
	fasthttp.ServeFile(c.ctx, filepath)
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

func (c *Context) FileAttachment(filepath, filename string) {
	if isASCII(filename) {
		c.ctx.Response.Header.Set("Content-Disposition", `attachment; filename="`+escapeQuotes(filename)+`"`)
	} else {
		c.ctx.Response.Header.Set("Content-Disposition", `attachment; filename*=UTF-8''`+url.QueryEscape(filename))
	}
	fasthttp.ServeFile(c.ctx, filepath)
}
