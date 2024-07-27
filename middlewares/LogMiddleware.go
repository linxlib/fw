package middlewares

import (
	"fmt"
	"github.com/linxlib/fw"
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
	logAttr = "Log"
	logName = "Log"
)

func NewLogMiddleware(logger types.ILogger) fw.IMiddlewareCtl {
	return &LogMiddleware{
		MiddlewareCtl: fw.NewMiddlewareCtl(logName, logAttr),
		logger:        logger}
}

// LogMiddleware
// for logging request info.
// can be used on Controller or Method
type LogMiddleware struct {
	fw.MiddlewareCtl
	logger types.ILogger
	// real_ip_header=CF-Connecting-IP
	realIPHeader string

	start time.Time
}

func (w *LogMiddleware) CloneAsCtl() fw.IMiddlewareCtl {
	return NewLogMiddleware(w.logger)
}

func (w *LogMiddleware) HandlerController(s string) *fw.RouteItem {
	return &fw.RouteItem{
		Method:     "",
		Path:       "",
		IsHide:     false,
		H:          nil,
		Middleware: w,
	}
}

func (w *LogMiddleware) CloneAsMethod() fw.IMiddlewareMethod {
	return w.CloneAsCtl()
}

func (w *LogMiddleware) HandlerMethod(next fw.HandlerFunc) fw.HandlerFunc {
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

		w.logger.Infof("|%3d| %13v | %15s | %-7s %#v %s\n%s",
			params.StatusCode,
			params.Latency.String(),
			params.ClientIP,
			params.Method,
			params.Path,
			byteCountSI(int64(params.BodySize)),
			params.ErrorMessage,
		)
	}
}

// ByteCountSI 字节数转带单位
func byteCountSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}
