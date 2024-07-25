package binding

import (
	"github.com/valyala/fasthttp"
)

const defaultMemory = 32 << 20

type formBinding struct{}
type formPostBinding struct{}
type formMultipartBinding struct{}

func (formBinding) Name() string {
	return "form"
}

func (formBinding) Bind(req *fasthttp.RequestCtx, obj any) error {
	f := make(map[string][]string)
	req.QueryArgs().VisitAll(func(key []byte, value []byte) {
		f[string(key)] = []string{string(value)}
	})
	req.Request.PostArgs().VisitAll(func(key, value []byte) {
		f[string(key)] = []string{string(value)}
	})

	if err := mapForm(obj, f); err != nil {
		return err
	}
	return validate(obj)
}

func (formPostBinding) Name() string {
	return "form-urlencoded"
}

func (formPostBinding) Bind(req *fasthttp.RequestCtx, obj any) error {
	f := make(map[string][]string)
	req.Request.PostArgs().VisitAll(func(key, value []byte) {
		f[string(key)] = []string{string(value)}
	})
	if err := mapForm(obj, f); err != nil {
		return err
	}
	return validate(obj)
}

func (formMultipartBinding) Name() string {
	return "multipart/form-data"
}

func (formMultipartBinding) Bind(req *fasthttp.RequestCtx, obj any) error {
	mform, err := req.Request.MultipartForm()
	if err != nil {
		return err
	}
	if err := mapForm(obj, mform.Value); err != nil {
		return err
	}

	return validate(obj)
}
