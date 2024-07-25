package binding

import "github.com/valyala/fasthttp"

type uriBinding struct{}

func (uriBinding) Name() string {
	return "uri"
}
func (uriBinding) Bind(req *fasthttp.RequestCtx, obj interface{}) error {
	f := make(map[string][]string)
	req.VisitUserValues(func(key []byte, a any) {
		f[string(key)] = []string{a.(string)}
	})
	if err := mapURI(obj, f); err != nil {
		return err
	}
	return validate(obj)
}

func (uriBinding) BindUri(m map[string][]string, obj any) error {
	if err := mapURI(obj, m); err != nil {
		return err
	}
	return validate(obj)
}
