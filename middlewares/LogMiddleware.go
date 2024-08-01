package middlewares

import (
	"fmt"
	"github.com/gookit/color"
	"github.com/linxlib/conv"
	"github.com/linxlib/fw"
	"github.com/sirupsen/logrus"
	"net/http"
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

func (p *LogParams) TimeStampWithColor() string {
	return color.HiWhite.Sprint(p.TimeStamp.Format(time.DateTime))
}
func (p *LogParams) LatencyWithColor() string {
	return color.HiWhite.Sprint(p.Latency.String())
}
func (p *LogParams) ClientIPWithColor() string {
	return color.HiWhite.Sprint(p.ClientIP)
}
func (p *LogParams) StatusCodeWithColor() string {
	code := p.StatusCode
	switch {
	case code >= http.StatusContinue && code < http.StatusOK:
		return color.White.Sprint(code)
	case code >= http.StatusOK && code < http.StatusMultipleChoices:
		return color.Green.Sprint(code)
	case code >= http.StatusMultipleChoices && code < http.StatusBadRequest:
		return color.White.Sprint(code)
	case code >= http.StatusBadRequest && code < http.StatusInternalServerError:
		return color.Yellow.Sprint(code)
	default:
		return color.Red.Sprint(code)
	}
}

func (p *LogParams) MethodWithColor() string {
	switch p.Method {
	case "GET":
		return color.Blue.Sprint(p.Method)
	case "POST":
		return color.Cyan.Sprint(p.Method)
	case "PUT":
		return color.Yellow.Sprint(p.Method)
	case "DELETE":
		return color.Red.Sprint(p.Method)
	case "PATCH":
		return color.Green.Sprint(p.Method)
	case "HEAD":
		return color.Magenta.Sprint(p.Method)
	case "OPTIONS":
		return color.White.Sprint(p.Method)
	default:
		return color.Normal.Sprint(p.Method)
	}
}

const (
	logAttr = "Log"
	logName = "Log"
)

func NewLogMiddleware(logger *logrus.Logger) fw.IMiddlewareCtl {
	return &LogMiddleware{
		MiddlewareCtl: fw.NewMiddlewareCtl(logName, logAttr),
		Logger:        logger,
	}
}

// LogMiddleware
// for logging request info.
// can be used on Controller or Method
type LogMiddleware struct {
	*fw.MiddlewareCtl
	Logger *logrus.Logger `inject:""`
	// real_ip_header=CF-Connecting-IP
	realIPHeader string
}

func (w *LogMiddleware) CloneAsCtl() fw.IMiddlewareCtl {
	return NewLogMiddleware(w.Logger)
}

func (w *LogMiddleware) HandlerController(s string) *fw.RouteItem {
	return fw.EmptyRouteItem(w)
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
		fctx := context.GetFastContext()
		start := time.Now()
		params := &LogParams{}
		params.BodySize = len(fctx.PostBody())
		params.Path = conv.String(fctx.Request.RequestURI())
		// add Cloudflare CDN real ip header support
		if w.realIPHeader != "" {
			params.ClientIP = string(fctx.Request.Header.Peek(w.realIPHeader))
		}
		params.ClientIP = fctx.RemoteIP().String()
		params.Method = conv.String(fctx.Method())
		next(context)
		params.TimeStamp = time.Now()
		params.Latency = params.TimeStamp.Sub(start)
		params.StatusCode = fctx.Response.StatusCode()
		err, exist := context.Get("fw_err")
		if exist && err != nil {
			params.ErrorMessage = "\nErr:" + err.(error).Error()
		}

		w.Logger.Printf("|%3s| %18s | %15s | %-7s %s %s%s",
			params.StatusCodeWithColor(),
			params.LatencyWithColor(),
			params.ClientIPWithColor(),
			params.MethodWithColor(),
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
