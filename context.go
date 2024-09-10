package fw

import (
	"bufio"
	"fmt"
	"github.com/gookit/goutil/errorx"
	"github.com/linxlib/conv"
	"github.com/linxlib/fw/render"
	"github.com/linxlib/inject"
	"github.com/valyala/bytebufferpool"
	"github.com/valyala/fasthttp"
	"html/template"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Context struct {
	ctx  *fasthttp.RequestCtx
	inj  inject.Injector
	mu   sync.RWMutex
	Keys map[string]any

	errs       errorx.Errors
	ErrHandler func(*Context, error)
	hasReturn  bool // 是否已经通过上下文方法写入了返回值(包括并且不限于状态码, body, header等)
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

// TODO: 这个方法应该被取消或私有, 为了配合hasReturn
func (c *Context) GetFastContext() *fasthttp.RequestCtx {
	return c.ctx
}

func (c *Context) Injector() inject.Injector {
	return c.inj
}

func (c *Context) Invoke(i interface{}) ([]reflect.Value, error) {
	return c.inj.Invoke(i)
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
	return conv.String(c.ctx.UserValue(key))
}

// TODO: 完善Context所提供的方法

func (c *Context) Status(code int) {
	c.hasReturn = true
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
func (c *Context) render(code int, r render.IRender) {
	c.Status(code)

	if !bodyAllowedForStatus(code) {
		r.WriteContentType(c.ctx)
		return
	}

	if err := r.Render(c.ctx); err != nil {
		// Pushing error to c.Errors
		c.Error(err)
		//c.Abort()
	}
}

func (c *Context) Error(err error) {
	if err == nil {
		panic("err is nil")
	}

	c.String(500, err.Error())
}

func (c *Context) JSON(code int, obj any) {
	c.render(code, render.JSON{Data: obj})
}
func (c *Context) JSONP(data any, callback ...string) {
	var cb string

	if len(callback) > 0 {
		cb = callback[0]
	} else {
		cb = "callback"
	}
	c.render(200, render.JsonpJSON{
		Callback: cb,
		Data:     data,
	})
}
func (c *Context) AsciiJSON(code int, obj any) {
	c.render(code, render.AsciiJSON{Data: obj})
}
func (c *Context) PureJSON(code int, obj any) {
	c.render(code, render.PureJSON{Data: obj})
}
func (c *Context) XML(code int, obj any) {
	c.render(code, render.XML{Data: obj})
}
func (c *Context) String(code int, format string, values ...any) {
	c.render(code, render.String{Format: format, Data: values})
}
func (c *Context) Redirect(code int, location string) {
	c.render(-1, render.Redirect{
		Code:     code,
		Location: location,
	})
}
func (c *Context) Data(code int, contentType string, data []byte) *Context {
	c.render(code, render.Data{
		ContentType: contentType,
		Data:        data,
	})
	return c
}
func (c *Context) DataFromReader(code int, contentLength int64, contentType string, reader io.Reader, extraHeaders map[string]string) {
	c.render(code, render.Reader{
		Headers:       extraHeaders,
		ContentType:   contentType,
		ContentLength: contentLength,
		Reader:        reader,
	})
}
func (c *Context) File(filepath string) *Context {
	c.hasReturn = true
	c.ctx.SendFile(filepath)
	return c
}

// Protocol returns the HTTP protocol of request: HTTP/1.1 and HTTP/2.
func (c *Context) Protocol() string {
	return conv.String(c.ctx.Request.Header.Protocol())
}
func (c *Context) HTML(code int, name string, obj any) {
}
func (c *Context) HTMLPure(code int, content string, obj any) *Context {
	tmpl, _ := template.New("html").Parse(content)
	c.render(code, render.HTML{
		Template: tmpl,
		Name:     "html",
		Data:     obj,
	})
	return c
}

// Stream sends a streaming response and returns a boolean
// indicates "Is client disconnected in middle of stream"
func (c *Context) Stream(step func(w *bufio.Writer)) {
	c.hasReturn = true
	c.SetContentType("text/event-stream")
	c.SetHeader("Cache-Control", "no-cache")
	c.SetHeader("Connection", "keep-alive")
	c.SetHeader("Access-Control-Allow-Origin", "*")
	c.SetHeader("Transfer-Encoding", "chunked")
	c.ctx.SetBodyStreamWriter(step)
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

func (c *Context) FileAttachment(filepath, filename string) *Context {
	c.hasReturn = true
	if isASCII(filename) {
		c.ctx.Response.Header.Set("Content-Disposition", `attachment; filename="`+escapeQuotes(filename)+`"`)
	} else {
		c.ctx.Response.Header.Set("Content-Disposition", `attachment; filename*=UTF-8''`+url.QueryEscape(filename))
	}
	c.ctx.SendFile(filepath)
	return c
}

// Vary adds the given header field to the Vary response header.
// This will append the header, if not already listed, otherwise leaves it listed in the current location.
func (c *Context) Vary(fields ...string) {
	c.Append(fasthttp.HeaderVary, fields...)
}

// Write appends p into response body.
func (c *Context) Write(p []byte) (int, error) {
	c.ctx.Response.AppendBody(p)
	return len(p), nil
}

// Writef appends `f` & `a` into response body writer.
func (c *Context) Writef(f string, a ...any) (int, error) {
	//nolint:wrap check // This must not be wrapped
	return fmt.Fprintf(c.ctx.Response.BodyWriter(), f, a...)
}

// WriteString appends s to response body.
func (c *Context) WriteString(s string) *Context {
	c.ctx.Response.AppendBodyString(s)
	return c
}

// SendStatus sets the HTTP status code and if the response body is empty,
// it sets the correct status message in the body.
func (c *Context) SendStatus(status int) {
	c.Status(status)

	// Only set status body when there is no response body
	if len(c.ctx.Response.Body()) == 0 {
		c.WriteString(StatusMessage(status))
		return
	}
}

// XHR returns a Boolean property, that is true, if the request's X-Requested-With header field is XMLHttpRequest,
// indicating that the request was issued by a client library (such as jQuery).
func (c *Context) XHR() bool {
	return EqualFold(conv.Bytes(c.GetHeader(fasthttp.HeaderXRequestedWith)), []byte("xmlhttprequest"))
}

// GetHeader returns the HTTP request header specified by field.
// Field names are case-insensitive
// Returned value is only valid within the handler. Do not store any references.
// Make copies or use the Immutable setting instead.
func (c *Context) GetHeader(key string, defaultValue ...string) string {
	return GetReqHeader(c, key, defaultValue...)
}

// GetReqHeader returns the HTTP request header specified by filed.
// This function is generic and can handle differnet headers type values.
func GetReqHeader[V GenericType](c *Context, key string, defaultValue ...V) V {
	var v V
	return genericParseType[V](conv.String(c.ctx.Request.Header.Peek(key)), v, defaultValue...)
}

// Append the specified value to the HTTP response header field.
// If the header is not already set, it creates the header with the specified value.
func (c *Context) Append(field string, values ...string) {
	if len(values) == 0 {
		return
	}
	h := conv.String(c.ctx.Response.Header.Peek(field))
	originalH := h
	for _, value := range values {
		if len(h) == 0 {
			h = value
		} else if h != value && !strings.HasPrefix(h, value+",") && !strings.HasSuffix(h, " "+value) &&
			!strings.Contains(h, " "+value+",") {
			h += ", " + value
		}
	}
	if originalH != h {
		c.Set(field, h)
	}
}

var localHosts = [...]string{"127.0.0.1", "::1"}

// IsLocalHost will return true if address is a localhost address.
func (*Context) isLocalHost(address string) bool {
	for _, h := range localHosts {
		if address == h {
			return true
		}
	}
	return false
}

// IsFromLocal will return true if request came from local.
func (c *Context) IsFromLocal() bool {
	return c.isLocalHost(c.ctx.RemoteIP().String())
}

// Type sets the Content-Type HTTP header to the MIME type specified by the file extension.
func (c *Context) Type(extension string, charset ...string) *Context {
	if len(charset) > 0 {
		c.ctx.Response.Header.SetContentType(GetMIME(extension) + "; charset=" + charset[0])
	} else {
		c.ctx.Response.Header.SetContentType(GetMIME(extension))
	}
	return c
}
func (c *Context) Method() string {
	return conv.String(c.ctx.Method())
}
func (c *Context) setCanonical(key, val string) {
	c.ctx.Response.Header.SetCanonical(conv.Bytes(key), conv.Bytes(val))
}

// Location sets the response Location HTTP header to the specified path parameter.
func (c *Context) Location(path string) *Context {
	c.setCanonical("Location", path)
	return c
}

// ContextString returns unique string representation of the ctx.
//
// The returned value may be useful for logging.
func (c *Context) ContextString() string {
	// Get buffer from pool
	buf := bytebufferpool.Get()

	// Start with the ID, converting it to a hex string without fmt.Sprintf
	buf.WriteByte('#')
	// Convert ID to hexadecimal
	id := strconv.FormatUint(c.ctx.ID(), 16)
	// Pad with leading zeros to ensure 16 characters
	for i := 0; i < (16 - len(id)); i++ {
		buf.WriteByte('0')
	}
	buf.WriteString(id)
	buf.WriteString(" - ")

	// Add local and remote addresses directly
	buf.WriteString(c.ctx.LocalAddr().String())
	buf.WriteString(" <-> ")
	buf.WriteString(c.ctx.RemoteAddr().String())
	buf.WriteString(" - ")

	// Add method and URI
	buf.Write(c.ctx.Request.Header.Method())
	buf.WriteByte(' ')
	buf.Write(c.ctx.URI().FullURI())

	// Allocate string
	str := buf.String()

	// Reset buffer
	buf.Reset()
	bytebufferpool.Put(buf)

	return str
}

// SetHeader sets the response's HTTP header field to the specified key, value.
func (c *Context) SetHeader(key, val string) *Context {
	c.ctx.Response.Header.Set(key, val)

	return c
}

// SendStream sets response body stream and optional body size.
func (c *Context) SendStream(stream io.Reader, size ...int) error {
	c.hasReturn = true
	if len(size) > 0 && size[0] >= 0 {
		c.ctx.Response.SetBodyStream(stream, size[0])
	} else {
		c.ctx.Response.SetBodyStream(stream, -1)
	}

	return nil
}
func (c *Context) SaveUploadFile(file *multipart.FileHeader, dst string) error {
	open, err := file.Open()
	if err != nil {
		return err
	}
	defer open.Close()
	if err = os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, open)
	if err != nil {
		return err
	}
	return nil
}

func (c *Context) PostBody() []byte {
	return c.ctx.PostBody()
}

func (c *Context) QueryArgs() *fasthttp.Args {
	return c.ctx.QueryArgs()
}
func (c *Context) VisitQueryArgs(callback func(key string, value string)) {
	c.ctx.QueryArgs().VisitAll(func(key, value []byte) {
		callback(conv.String(key), conv.String(value))
	})
}
func (c *Context) PostArgs() *fasthttp.Args {
	return c.ctx.PostArgs()
}
func (c *Context) VisitPostArgs(callback func(key string, value string)) {
	c.ctx.PostArgs().VisitAll(func(key, value []byte) {
		callback(conv.String(key), conv.String(value))
	})
}

func (c *Context) MultipartForm() (*multipart.Form, error) {
	return c.ctx.MultipartForm()
}

func (c *Context) ServeFS(fs fs.FS, path string) *Context {
	fasthttp.ServeFS(c.ctx, fs, path)
	return c
}
func (c *Context) SetContentType(value string) *Context {
	c.ctx.SetContentType(value)
	return c
}
func (c *Context) RequestURI() string {
	return conv.String(c.ctx.RequestURI())
}
func (c *Context) UserAgent() string {
	return conv.String(c.ctx.UserAgent())
}
func (c *Context) Host() string {
	return conv.String(c.ctx.Host())
}
func (c *Context) ResetBody() *Context {
	c.ctx.Response.ResetBody()
	return c
}
func (c *Context) ResponseBody() []byte {
	return c.ctx.Response.Body()
}
