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

var _ fw.IMiddlewareGlobal = (*ResponseRewriterMiddleware)(nil)

type ResponseRewriterMiddleware struct {
	*fw.MiddlewareGlobal
}

func (s *ResponseRewriterMiddleware) Execute(ctx *fw.MiddlewareContext) fw.HandlerFunc {
	return func(context *fw.Context) {
		ctx.Next(context)
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

const responseRewriterName = "ResponseRewriter"

func NewResponseRewriteMiddleware() fw.IMiddlewareGlobal {

	return &ResponseRewriterMiddleware{
		MiddlewareGlobal: fw.NewMiddlewareGlobal(responseRewriterName),
	}
}
