package binding

import (
	"github.com/linxlib/conv"
	"github.com/valyala/fasthttp"
)

type pathBinding struct{}

func (pathBinding) Name() string {
	return "path"
}

func (pathBinding) Bind(req *fasthttp.RequestCtx, obj interface{}) error {
	f := make(map[string][]string)
	req.VisitUserValues(func(key []byte, a any) {
		k := conv.String(key)
		f[k] = append(f[k], conv.String(a))
	})
	if err := mapFormByTag(obj, f, "path"); err != nil {
		return err
	}
	return validate(obj)

}
func (pathBinding) BindUri(m map[string][]string, obj interface{}) error {
	if err := mapURI(obj, m); err != nil {
		return err
	}
	return validate(obj)
}
