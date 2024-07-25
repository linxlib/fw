package middlewares

import (
	"github.com/linxlib/fw"
	"github.com/linxlib/fw/attribute"
	"github.com/linxlib/fw/types"
	"net/url"
	"time"
)

type LogParams struct {
	// TimeStamp shows the time after the server returns a response.
	TimeStamp time.Time
	// StatusCode is HTTP response code.
	StatusCode int
	// Latency is how much time the server cost to process a certain request.
	Latency time.Duration
	// ClientIP equals Context's ClientIP method.
	ClientIP string
	// Method is the HTTP method given to the request.
	Method string
	// Path is a path the client requests.
	Path string
	// ErrorMessage is set if error has occurred in processing the request.
	ErrorMessage string
	// BodySize is the size of the Response Body
	BodySize int
	// Keys are the keys set on the request's context.
	Keys map[string]any
}

const (
	loggerAttr = "Logger"
	loggerName = "Logger"
)

func init() {
	fw.AddMethodAttributeType(loggerAttr, attribute.TypeMiddleware)
}

func NewLoggerMiddleware(logger types.ILogger) fw.IMiddleware {
	return &LoggerMiddleware{
		MiddlewareMethod: fw.NewMiddlewareMethod(loggerName, loggerAttr),
		logger:           logger}
}

type LoggerMiddleware struct {
	fw.MiddlewareMethod
	logger types.ILogger
	// real_ip_header=CF-Connecting-IP
	realIPHeader string

	start time.Time
}

func (w *LoggerMiddleware) CloneAsMethod() fw.IMiddlewareMethod {
	return &LoggerMiddleware{
		MiddlewareMethod: fw.NewMiddlewareMethod(loggerName, loggerAttr),
		logger:           w.logger}
}

func (w *LoggerMiddleware) HandlerMethod(next fw.HandlerFunc) fw.HandlerFunc {
	var p = w.GetParam()
	values, err := url.ParseQuery(p)
	if err != nil {
		w.realIPHeader = ""
	} else {
		w.realIPHeader = values.Get("real_ip_header")
	}
	return func(context *fw.Context) {
		//log.Println("LoggerMiddleware called")
		fctx := context.GetFastContext()
		w.start = time.Now()
		params := LogParams{}
		params.BodySize = len(fctx.PostBody())
		params.Path = string(fctx.Path())
		// add Cloudflare CDN real ip header support
		if w.realIPHeader != "" {
			params.ClientIP = string(fctx.Request.Header.Peek(w.realIPHeader))
		}
		params.ClientIP = fctx.RemoteIP().String()
		params.Method = string(fctx.Method())
		next(context)
		params.TimeStamp = time.Now()
		params.Latency = params.TimeStamp.Sub(w.start)
		params.StatusCode = fctx.Response.StatusCode()
		err, exist := context.Get("fw_err")
		if exist && err != nil {
			params.ErrorMessage = "Err:" + err.(error).Error()
		}

		w.logger.Infof("|%3d| %13v | %15s | %-7s %#v\n%s",
			params.StatusCode,
			params.Latency.String(),
			params.ClientIP,
			params.Method,
			params.Path,
			params.ErrorMessage,
		)
	}
}
