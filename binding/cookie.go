package binding

import "github.com/valyala/fasthttp"

type cookieBinding struct{}

func (cookieBinding) Name() string {
	return "cookie"
}

func (cookieBinding) Bind(req *fasthttp.RequestCtx, obj interface{}) error {
	f := make(map[string][]string)
	req.Request.Header.VisitAllCookie(func(k, v []byte) {
		f[string(k)] = []string{string(v)}
	})
	if err := mapFormByTag(obj, f, "cookie"); err != nil {
		return err
	}
	return validate(obj)

}
func (cookieBinding) BindUri(m map[string][]string, obj interface{}) error {
	if err := mapFormByTag(obj, m, "cookie"); err != nil {
		return err
	}
	return validate(obj)
}
