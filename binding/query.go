package binding

import (
	"github.com/linxlib/conv"
	"github.com/valyala/fasthttp"
	"strings"
)

type queryBinding struct{}

func (queryBinding) Name() string {
	return "query"
}

func (queryBinding) Bind(req *fasthttp.RequestCtx, obj any) error {
	values := req.URI().QueryArgs()
	f := make(map[string][]string)
	values.VisitAll(func(key, value []byte) {
		v := conv.String(value)
		var t = []string{v}
		if strings.ContainsAny(v, ",") {
			t = strings.Split(v, ",")
		}
		f[conv.String(key)] = t
	})
	if err := mapFormByTag(obj, f, "query"); err != nil {
		return err
	}
	return validate(obj)
}
