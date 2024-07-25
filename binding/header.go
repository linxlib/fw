package binding

import (
	"github.com/valyala/fasthttp"
	"net/textproto"
	"reflect"
)

type headerBinding struct{}

func (headerBinding) Name() string {
	return "header"
}

func (headerBinding) Bind(req *fasthttp.RequestCtx, obj any) error {
	h := make(map[string][]string)
	req.Request.Header.VisitAll(func(key, value []byte) {
		h[string(key)] = []string{string(value)}
	})
	if err := mapHeader(obj, h); err != nil {
		return err
	}

	return validate(obj)
}

func mapHeader(ptr any, h map[string][]string) error {
	return mappingByPtr(ptr, headerSource(h), "header")
}

type headerSource map[string][]string

var _ setter = headerSource(nil)

func (hs headerSource) TrySet(value reflect.Value, field reflect.StructField, tagValue string, opt setOptions) (bool, error) {
	return setByForm(value, field, hs, textproto.CanonicalMIMEHeaderKey(tagValue), opt)
}
