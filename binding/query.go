package binding

import (
	"github.com/valyala/fasthttp"
)

type queryBinding struct{}

func (queryBinding) Name() string {
	return "query"
}

func (queryBinding) Bind(req *fasthttp.RequestCtx, obj any) error {
	values := req.URI().QueryArgs()
	f := make(map[string][]string)
	values.VisitAll(func(key, value []byte) {
		f[string(key)] = []string{string(value)}
	})
	if err := mapFormByTag(obj, f, "query"); err != nil {
		return err
	}
	return validate(obj)
}
