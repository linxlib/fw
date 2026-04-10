package response

import (
	"encoding/json"

	"github.com/valyala/fasthttp"
)

type Envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	TraceID string `json:"trace_id,omitempty"`
}

type Formatter interface {
	Format(code int, message string, data any, traceID string) any
}

type Encoder interface {
	Encode(ctx *fasthttp.RequestCtx, statusCode int, payload any) error
}

type DefaultFormatter struct{}

func (DefaultFormatter) Format(code int, message string, data any, traceID string) any {
	return Envelope{Code: code, Message: message, Data: data, TraceID: traceID}
}

type JSONEncoder struct{}

func (JSONEncoder) Encode(ctx *fasthttp.RequestCtx, statusCode int, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ctx.SetStatusCode(statusCode)
	ctx.Response.Header.SetContentType("application/json")
	ctx.SetBody(body)
	return nil
}

type Manager struct {
	formatter Formatter
	encoder   Encoder
}

func NewManager(formatter Formatter, encoder Encoder) *Manager {
	if formatter == nil {
		formatter = DefaultFormatter{}
	}
	if encoder == nil {
		encoder = JSONEncoder{}
	}
	return &Manager{formatter: formatter, encoder: encoder}
}

func (m *Manager) Write(ctx *fasthttp.RequestCtx, statusCode int, code int, message string, data any, traceID string) error {
	payload := m.formatter.Format(code, message, data, traceID)
	return m.encoder.Encode(ctx, statusCode, payload)
}

func (m *Manager) WriteRaw(ctx *fasthttp.RequestCtx, statusCode int, payload any) error {
	return m.encoder.Encode(ctx, statusCode, payload)
}
