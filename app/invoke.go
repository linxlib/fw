package app

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/linxlib/fw/v2/astp"
	ctxpkg "github.com/linxlib/fw/v2/context"
	"github.com/linxlib/fw/v2/inject"
	"github.com/valyala/fasthttp"
)

type invokeSource struct {
	Method *astp.Func
	Hints  map[string]ParamHint
}

func buildRequestContainer(ctx *ctxpkg.FWContext, global inject.Injector, method *astp.Func, hints map[string]ParamHint) (inject.Injector, error) {
	container := inject.New()
	container.SetParent(global)
	container.Map(ctx)
	container.Map(ctx.Raw())
	container.MapTo(ctx, (*ctxpkg.Context)(nil))

	raw := ctx.Raw()
	paramMap := make(map[string]string)
	for _, p := range method.Params {
		if p == nil || p.Name == "" {
			continue
		}
		if v, ok := raw.UserValue(p.Name).(string); ok {
			paramMap[p.Name] = v
			continue
		}
		if v := raw.UserValue(p.Name); v != nil {
			paramMap[p.Name] = fmt.Sprintf("%v", v)
			continue
		}
		if q := raw.QueryArgs().Peek(p.Name); len(q) > 0 {
			paramMap[p.Name] = string(q)
			continue
		}
		if h := raw.Request.Header.Peek(p.Name); len(h) > 0 {
			paramMap[p.Name] = string(h)
		}
	}
	container.Map(paramMap)
	container.RegisterResolver(defaultArgResolverWithHints(hints))
	return container, nil
}

func invokeMethod(container inject.Injector, method *astp.Func, fn reflect.Value, hints map[string]ParamHint) ([]reflect.Value, error) {
	names := make([]string, 0, len(method.Params))
	for _, p := range method.Params {
		if p == nil {
			names = append(names, "")
			continue
		}
		names = append(names, p.Name)
	}
	return container.InvokeWith(fn.Interface(), names, invokeSource{Method: method, Hints: hints})
}

func defaultArgResolverWithHints(hints map[string]ParamHint) inject.ArgResolver {
	return func(arg inject.ArgContext, container inject.Injector) (reflect.Value, bool, error) {
		ctxVal := container.Get(reflect.TypeOf((*ctxpkg.FWContext)(nil)))
		if !ctxVal.IsValid() {
			return reflect.Value{}, false, nil
		}
		ctx, ok := ctxVal.Interface().(*ctxpkg.FWContext)
		if !ok {
			return reflect.Value{}, false, nil
		}
		raw := ctx.Raw()

		source := BindAuto
		if hint, ok := hints[arg.Name]; ok {
			source = hint.Source
		}

		if arg.Type.Kind() == reflect.Struct {
			v := reflect.New(arg.Type).Elem()
			if err := fillStructBySource(v, raw, source); err != nil {
				return reflect.Value{}, false, err
			}
			container.Set(arg.Type, v)
			return v, true, nil
		}
		if arg.Type.Kind() == reflect.Ptr && arg.Type.Elem().Kind() == reflect.Struct {
			v := reflect.New(arg.Type.Elem())
			if err := fillStructBySource(v.Elem(), raw, source); err != nil {
				return reflect.Value{}, false, err
			}
			container.Set(arg.Type, v)
			return v, true, nil
		}

		if arg.Name != "" {
			if text, ok := readNamedValueBySource(raw, arg.Name, source); ok {
				parsed, err := parsePrimitive(text, arg.Type)
				if err != nil {
					return reflect.Value{}, false, err
				}
				if parsed.IsValid() {
					container.Set(arg.Type, parsed)
					return parsed, true, nil
				}
			}
		}

		return reflect.Value{}, false, nil
	}
}

func fillStructBySource(v reflect.Value, raw *fasthttp.RequestCtx, source BindSource) error {
	if source == BindBody || source == BindAuto {
		body := raw.PostBody()
		if len(body) > 0 {
			if err := json.Unmarshal(body, v.Addr().Interface()); err != nil {
				return err
			}
		}
	}
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if !f.CanSet() {
			continue
		}
		sf := t.Field(i)
		for _, key := range bindNamesForField(sf, source) {
			if text, ok := readNamedValueBySource(raw, key, source); ok {
				pv, err := parsePrimitive(text, f.Type())
				if err != nil {
					return err
				}
				if pv.IsValid() {
					f.Set(pv)
					break
				}
			}
		}
	}
	return nil
}

func bindNamesForField(sf reflect.StructField, source BindSource) []string {
	var names []string
	appendIf := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" || v == "-" {
			return
		}
		v = strings.Split(v, ",")[0]
		if v == "" || v == "-" {
			return
		}
		for _, existing := range names {
			if existing == v {
				return
			}
		}
		names = append(names, v)
	}
	if source == BindAuto || source == BindQuery {
		appendIf(sf.Tag.Get("query"))
	}
	if source == BindAuto || source == BindPath {
		appendIf(sf.Tag.Get("path"))
	}
	if source == BindAuto || source == BindHeader {
		appendIf(sf.Tag.Get("header"))
	}
	appendIf(sf.Tag.Get("json"))
	appendIf(lowerFirst(sf.Name))
	appendIf(sf.Name)
	return names
}

func readNamedValueBySource(raw *fasthttp.RequestCtx, name string, source BindSource) (string, bool) {
	if source == BindAuto || source == BindPath {
		if uv := raw.UserValue(name); uv != nil {
			return fmt.Sprintf("%v", uv), true
		}
	}
	if source == BindAuto || source == BindQuery {
		if q := raw.QueryArgs().Peek(name); len(q) > 0 {
			return string(q), true
		}
	}
	if source == BindAuto || source == BindHeader {
		if h := raw.Request.Header.Peek(name); len(h) > 0 {
			return string(h), true
		}
	}
	if source == BindAuto {
		if q := raw.QueryArgs().Peek(lowerFirst(name)); len(q) > 0 {
			return string(q), true
		}
		if h := raw.Request.Header.Peek(lowerFirst(name)); len(h) > 0 {
			return string(h), true
		}
	}
	return "", false
}

func parsePrimitive(input string, t reflect.Type) (reflect.Value, error) {
	if t.Kind() == reflect.Ptr {
		v, err := parsePrimitive(input, t.Elem())
		if err != nil {
			return reflect.Value{}, err
		}
		ptr := reflect.New(t.Elem())
		ptr.Elem().Set(v)
		return ptr, nil
	}
	input = string(input)
	switch t.Kind() {
	case reflect.String:
		return reflect.ValueOf(input).Convert(t), nil
	case reflect.Bool:
		b, err := strconv.ParseBool(input)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(b).Convert(t), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(input, 10, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		v := reflect.New(t).Elem()
		v.SetInt(n)
		return v, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(input, 10, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		v := reflect.New(t).Elem()
		v.SetUint(n)
		return v, nil
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(input, 64)
		if err != nil {
			return reflect.Value{}, err
		}
		v := reflect.New(t).Elem()
		v.SetFloat(f)
		return v, nil
	default:
		return reflect.Value{}, nil
	}
}
