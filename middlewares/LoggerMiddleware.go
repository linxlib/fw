package middlewares

import (
	"github.com/linxlib/fw"
	"github.com/linxlib/fw/types"
)

const (
	loggerName = "Logger"
)

func NewLoggerMiddleware(logger types.ILogger) fw.IMiddlewareGlobal {
	return &LoggerMiddleware{
		MiddlewareGlobal: fw.NewMiddlewareGlobal(loggerName),
		logger:           logger}
}

type LoggerMiddleware struct {
	fw.MiddlewareGlobal
	logger types.ILogger
}

func (w *LoggerMiddleware) CloneAsCtl() fw.IMiddlewareCtl {
	return NewLoggerMiddleware(w.logger)
}

func (w *LoggerMiddleware) HandlerController(s string) *fw.RouteItem {
	return &fw.RouteItem{
		Method:     "",
		Path:       "",
		IsHide:     false,
		H:          nil,
		Middleware: w,
	}
}

func (w *LoggerMiddleware) CloneAsMethod() fw.IMiddlewareMethod {
	return w.CloneAsCtl()
}

func (w *LoggerMiddleware) HandlerMethod(next fw.HandlerFunc) fw.HandlerFunc {
	return func(context *fw.Context) {

	}
}
