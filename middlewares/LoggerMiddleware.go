package middlewares

import (
	"github.com/linxlib/conv"
	"github.com/linxlib/fw"
	"github.com/sirupsen/logrus"
	"time"
)

const (
	loggerName = "Logger"
)

func NewLoggerMiddleware(logger *logrus.Logger) fw.IMiddlewareGlobal {
	return &LoggerMiddleware{
		MiddlewareGlobal: fw.NewMiddlewareGlobal(loggerName),
		Logger:           logger}
}

type LoggerMiddleware struct {
	*fw.MiddlewareGlobal
	Logger *logrus.Logger `inject:""`
}

func (w *LoggerMiddleware) CloneAsCtl() fw.IMiddlewareCtl {
	return NewLoggerMiddleware(w.Logger)
}

func (w *LoggerMiddleware) HandlerController(s string) []*fw.RouteItem {
	return fw.EmptyRouteItem(w)
}

func (w *LoggerMiddleware) CloneAsMethod() fw.IMiddlewareMethod {
	return w.CloneAsCtl()
}

func (w *LoggerMiddleware) HandlerMethod(next fw.HandlerFunc) fw.HandlerFunc {
	return func(context *fw.Context) {
		fctx := context.GetFastContext()
		start := time.Now()
		params := &LogParams{}
		params.BodySize = len(fctx.PostBody())
		params.Path = conv.String(fctx.Request.RequestURI())
		//// add Cloudflare CDN real ip header support
		//if w.realIPHeader != "" {
		//	params.ClientIP = string(fctx.Request.Header.Peek(w.realIPHeader))
		//}
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
