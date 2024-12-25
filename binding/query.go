package binding

import (
	"github.com/linxlib/conv"
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
		if _, ok := f[conv.String(key)]; ok {
			f[conv.String(key)] = append(f[conv.String(key)], arrValues(value)...)
		} else {
			f[conv.String(key)] = arrValues(value)
		}

	})
	if err := mapFormByTag(obj, f, "query"); err != nil {
		return err
	}
	return validate(obj)
}
