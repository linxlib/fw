package binding

import (
	"github.com/valyala/fasthttp"
)

type pathBinding struct{}

func (pathBinding) Name() string {
	return "path"
}

func (pathBinding) Bind(req *fasthttp.RequestCtx, obj interface{}) error {
	f := make(map[string][]string)
	req.VisitUserValues(func(key []byte, a any) {
		f[string(key)] = []string{a.(string)}
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
