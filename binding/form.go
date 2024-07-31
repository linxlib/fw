package binding

import (
	"github.com/linxlib/conv"
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
		k := conv.String(key)
		f[k] = append(f[k], conv.String(value))
	})
	req.Request.PostArgs().VisitAll(func(key, value []byte) {
		k := conv.String(key)
		f[k] = append(f[k], conv.String(value))
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
		k := conv.String(key)
		f[k] = append(f[k], conv.String(value))
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
