package app

import (
	"reflect"
	"strings"

	"github.com/linxlib/fw/v2/annotation"
	"github.com/linxlib/fw/v2/astp"
	"github.com/valyala/fasthttp"
)

func hasRawResponseAnnotation(method *astp.Func) bool {
	if method == nil {
		return false
	}
	for _, ann := range annotation.FromDoc(method.Doc) {
		if strings.EqualFold(strings.TrimSpace(ann.Name), "RawResponse") {
			return true
		}
	}
	return false
}

func extractMethodResponse(results []reflect.Value) (any, error) {
	var data any
	for _, result := range results {
		if !result.IsValid() {
			continue
		}
		if result.Type().Implements(errorType) {
			if result.IsNil() {
				continue
			}
			err, _ := result.Interface().(error)
			return nil, err
		}
		if data == nil {
			data = result.Interface()
		}
	}
	return data, nil
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

func responseWritten(raw *fasthttp.RequestCtx) bool {
	if raw == nil {
		return false
	}
	if len(raw.Response.Body()) > 0 {
		return true
	}
	return raw.Response.StatusCode() != fasthttp.StatusOK
}
