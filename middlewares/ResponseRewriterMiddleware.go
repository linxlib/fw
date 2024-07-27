package middlewares

import (
	"github.com/linxlib/fw"
	"github.com/linxlib/fw/internal/json"
)

type Data[T int | string] struct {
	Code    T
	Message string
	Data    interface{}
}

type ResponseRewriterMiddleware struct {
	fw.MiddlewareGlobal
}

func (s *ResponseRewriterMiddleware) CloneAsMethod() fw.IMiddlewareMethod {
	return s.CloneAsCtl()
}

func (s *ResponseRewriterMiddleware) HandlerMethod(h fw.HandlerFunc) fw.HandlerFunc {
	return func(context *fw.Context) {
		h(context)
		result := new(Data[int])

		switch context.GetFastContext().Response.StatusCode() {
		case 200:
			result.Code = 200
			result.Message = "ok"
			result.Data = make([]map[string]any, 0)
			err := json.Unmarshal(context.GetFastContext().Response.Body(), &result.Data)
			if err != nil {
				result.Message = err.Error()
			}
			bs, _ := json.Marshal(result)
			context.GetFastContext().Response.ResetBody()
			context.Data(200, "application/json", bs)
		default:

		}

	}
}

func (s *ResponseRewriterMiddleware) CloneAsCtl() fw.IMiddlewareCtl {
	return NewResponseRewriteMiddleware()
}

func (s *ResponseRewriterMiddleware) HandlerController(base string) *fw.RouteItem {
	return &fw.RouteItem{
		Method:     "",
		Path:       "",
		IsHide:     false,
		H:          nil,
		Middleware: s,
	}
}

const responseRewriterName = "ResponseRewriter"

func NewResponseRewriteMiddleware() fw.IMiddlewareGlobal {

	return &ResponseRewriterMiddleware{
		MiddlewareGlobal: fw.NewMiddlewareGlobal(responseRewriterName),
	}
}
