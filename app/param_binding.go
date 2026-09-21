package app

import (
	"sort"
	"strconv"
	"strings"

	"github.com/linxlib/fw/v2/astp"
	"github.com/linxlib/fw/v2/openapi"
)

type BindSource string

const (
	BindAuto   BindSource = "auto"
	BindQuery  BindSource = "query"
	BindPath   BindSource = "path"
	BindHeader BindSource = "header"
	BindBody   BindSource = "body"
)

type ParamHint struct {
	Name   string
	Source BindSource
}

func buildParamHints(q *astp.Query, method *astp.Func, pathParamSet map[string]struct{}, typeArgs astp.TypeArgBinding) map[string]ParamHint {
	typeParams := typeParamNames(method)
	hints := make(map[string]ParamHint)
	for _, p := range method.Params {
		if p == nil || p.Name == "" {
			continue
		}
		if isContextParam(p.Type) {
			continue
		}
		source := inferBindSource(q, p, pathParamSet, typeParams)
		hints[p.Name] = ParamHint{Name: p.Name, Source: source}
	}
	return hints
}

// typeParamNames 返回一个函数/方法签名内可见的类型参数名集合:
// 方法自身声明的类型参数(Go 1.27 泛型方法) ∪ 接收器上的结构体类型实参.
//
// 泛型基础控制器的方法正是靠后者拿到实体类型占位, 例如
//
//	func (c *BaseController[T]) Create(ctx context.Context, entity *T) error
//
// 这里的 T 来自接收器 BaseController[T], 具体类型(如 User)只在实例化时可知.
func typeParamNames(method *astp.Func) map[string]bool {
	if method == nil {
		return nil
	}

	var names []string
	if method.Generic != nil {
		for _, gp := range method.Generic.Params {
			if gp != nil && gp.Name != "" {
				names = append(names, gp.Name)
			}
		}
	}
	if method.Recv != nil && method.Recv.Generic != nil {
		for _, arg := range method.Recv.Generic.Args {
			if arg != nil && arg.Name != "" {
				names = append(names, arg.Name)
			}
		}
	}
	if len(names) == 0 {
		return nil
	}

	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[n] = true
	}
	return out
}

func inferBindSource(q *astp.Query, p *astp.Param, pathParamSet map[string]struct{}, typeParams map[string]bool) BindSource {
	if p == nil || p.Type == nil {
		return BindQuery
	}

	// 泛型占位参数优先判定: 形如 entity *T / value T, T 是方法自身或接收器的类型参数.
	// 静态阶段无法解析到具体类型, 但指针/结构形参在 HTTP 语义上是请求体,
	// 标量占位则按 query 处理(调用点才知道真实类型).
	if len(typeParams) > 0 && p.Type.Name != "" && typeParams[p.Type.Name] {
		switch p.Type.Kind {
		case astp.KindPointer, astp.KindStruct, astp.KindTypeParam:
			return BindBody
		default:
			return BindQuery
		}
	}

	typeName := p.Type.Name
	resolved := q.ResolveParamType(p)
	if resolved != nil {
		if ann := annotationSource(resolved.Doc); ann != "" {
			return ann
		}
		typeName = resolved.Name
	}
	lowerName := strings.ToLower(typeName)
	switch {
	case strings.HasSuffix(lowerName, "query"):
		return BindQuery
	case strings.HasSuffix(lowerName, "path"):
		return BindPath
	case strings.HasSuffix(lowerName, "header"):
		return BindHeader
	case strings.HasSuffix(lowerName, "body"):
		return BindBody
	}
	if _, ok := pathParamSet[p.Name]; ok {
		return BindPath
	}
	if resolved != nil && resolved.Kind == astp.KindStruct {
		return BindBody
	}
	return BindQuery
}

func annotationSource(doc *astp.CommentGroup) BindSource {
	if doc == nil {
		return ""
	}
	for _, ann := range doc.ParsedAnnotations {
		if ann == nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(ann.Name)) {
		case "query":
			return BindQuery
		case "path":
			return BindPath
		case "header":
			return BindHeader
		case "body":
			return BindBody
		}
	}
	for _, name := range doc.Annotations {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "query":
			return BindQuery
		case "path":
			return BindPath
		case "header":
			return BindHeader
		case "body":
			return BindBody
		}
	}
	return ""
}

func isContextParam(t *astp.TypeRef) bool {
	if t == nil {
		return false
	}
	name := strings.TrimSpace(t.Name)
	if name == "Context" {
		return true
	}
	lp := strings.ToLower(strings.TrimSpace(t.PkgPath))
	if lp != "" && strings.Contains(lp, "context") {
		return true
	}
	return false
}

func buildOpenAPIForMethod(q *astp.Query, method *astp.Func, hints map[string]ParamHint, typeArgs astp.TypeArgBinding) ([]openapi.Parameter, *openapi.RequestBody) {
	var params []openapi.Parameter
	var bodySchema *openapi.Schema
	for _, p := range method.Params {
		if p == nil || p.Name == "" || isContextParam(p.Type) {
			continue
		}
		hint, ok := hints[p.Name]
		if !ok {
			continue
		}

		// 泛型占位参数(如 entity *T)在静态阶段解析不到具体类型, 先按实参绑定实例化.
		pType := p.Type
		if len(typeArgs) > 0 {
			pType = astp.InstantiateTypeRef(p.Type, typeArgs)
		}
		resolved := q.ResolveTypeRef(pType)
		schema := schemaFromTypeRef(q, pType, typeArgs)

		if resolved != nil && resolved.Kind == astp.KindStruct {
			// 进入被实例化类型时, 用它自身的形参与传入实参重建作用域.
			inner := innerBinding(q, resolved, pType)
			fields := resolved.Fields
			if len(inner) > 0 {
				fields = astp.InstantiateFields(resolved.Fields, inner)
			}
			switch hint.Source {
			case BindQuery, BindPath, BindHeader:
				for _, f := range fields {
					if f == nil || f.Type == nil {
						continue
					}
					name := fieldNameForSource(f, hint.Source)
					if name == "" {
						continue
					}
					params = append(params, openapi.Parameter{
						Name:        name,
						In:          string(hint.Source),
						Required:    hint.Source == BindPath || isFieldRequired(f, hint.Source),
						Description: fieldDescription(f),
						Schema:      schemaFromField(q, f, inner),
					})
				}
			case BindBody:
				s := schemaFromStructType(q, resolved, inner)
				bodySchema = &s
			}
			continue
		}

		switch hint.Source {
		case BindBody:
			bodySchema = &schema
		default:
			params = append(params, openapi.Parameter{
				Name:        p.Name,
				In:          string(hint.Source),
				Required:    hint.Source == BindPath,
				Description: paramDescription(p),
				Schema:      schema,
			})
		}
	}
	if bodySchema == nil {
		return params, nil
	}
	return params, &openapi.RequestBody{
		Required: true,
		Content: map[string]openapi.MediaType{
			"application/json": {
				Schema:  *bodySchema,
				Example: ExampleFromSchema(*bodySchema, 0),
			},
		},
	}
}

// innerBinding 进入一个被实例化的泛型类型时, 用它自身的形参与 ref 上的实参重建绑定.
//
// 这是「同名不同作用域」的关键: Base.T 与 PageSize.T 是两个独立的类型参数,
// 不重建就会把外层的绑定误用到内层作用域.
func innerBinding(q *astp.Query, t *astp.Type, ref *astp.TypeRef) astp.TypeArgBinding {
	if q == nil || t == nil || ref == nil || ref.Generic == nil || len(ref.Generic.Args) == 0 {
		return nil
	}
	return astp.RebindTypeArgs(t, ref.Generic.Args)
}

// schemaFromField 由字段生成 schema, 并按需在字段类型内部重建泛型作用域.
// 同时应用字段上的 example / default tag, 让 query/path/header 参数也带上文档标注.
func schemaFromField(q *astp.Query, f *astp.Field, bind astp.TypeArgBinding) openapi.Schema {
	if f == nil || f.Type == nil {
		return openapi.Schema{Type: "string"}
	}
	s := schemaFromTypeRef(q, f.Type, bind)
	applyFieldTags(f, &s)
	return s
}

func fieldNameForSource(f *astp.Field, source BindSource) string {
	if f == nil {
		return ""
	}
	if f.Tag != nil {
		key := string(source)
		if v := strings.TrimSpace(f.Tag[key]); v != "" && v != "-" {
			return strings.Split(v, ",")[0]
		}
		if v := strings.TrimSpace(f.Tag["json"]); v != "" && v != "-" {
			return strings.Split(v, ",")[0]
		}
	}

	return lowerFirst(f.Name)
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func schemaFromStructType(q *astp.Query, t *astp.Type, bind astp.TypeArgBinding) openapi.Schema {
	props := make(map[string]openapi.Schema)
	var required []string
	if t != nil {
		fields := t.Fields
		if len(bind) > 0 {
			fields = astp.InstantiateFields(t.Fields, bind)
		}
		for _, f := range fields {
			if f == nil || f.Type == nil || f.Name == "" {
				continue
			}
			name := fieldNameForSource(f, BindBody)
			if name == "" {
				continue
			}
			// 字段自身(或其嵌套位置)是被实例化的泛型类型时, 用该类型自身的形参与
			// 实际实参重建一层作用域. 需要下钻查找: 泛型实参可能在顶层(PageSize[*T]),
			// 也可能藏在切片元素([]T)或 map 值(map[string]PageSize[T])里.
			fieldBind := bind
			if inner := nestedBinding(q, f.Type, bind); len(inner) > 0 {
				fieldBind = inner
			}
			s := schemaFromTypeRef(q, f.Type, fieldBind)
			s.Description = fieldDescription(f)
			applyFieldTags(f, &s)
			props[name] = s
			if isFieldRequired(f, BindBody) {
				required = append(required, name)
			}
		}
	}
	sort.Strings(required)
	return openapi.Schema{Type: "object", Properties: props, Required: required}
}

func schemaFromTypeRef(q *astp.Query, ref *astp.TypeRef, bind astp.TypeArgBinding) openapi.Schema {
	if ref == nil {
		return openapi.Schema{Type: "string"}
	}
	switch ref.Kind {
	case astp.KindSlice:
		item := schemaFromTypeRef(q, ref.ElemType, bind)
		return openapi.Schema{Type: "array", Items: &item}
	case astp.KindMap:
		val := schemaFromTypeRef(q, ref.ElemType, bind)
		return openapi.Schema{Type: "object", AdditionalProperties: &val}
	case astp.KindStruct, astp.KindPointer:
		if q != nil {
			if t := q.ResolveTypeRef(ref); t != nil {
				if t.Kind == astp.KindStruct {
					// 被实例化的泛型类型: 用它自身的形参与 ref 上的实参重建作用域.
					inner := innerBinding(q, t, ref)
					if len(inner) == 0 {
						inner = bind
					}
					return schemaFromStructType(q, t, inner)
				}
				if t.Kind == astp.KindEnum {
					if e := q.FindEnum(t.Name); e != nil {
						return schemaFromEnum(q, e)
					}
				}
			}
		}
		// 解析不到具体类型(跨包类型, 或未推断出的泛型占位): 保持对象占位.
		return openapi.Schema{Type: "object"}
	default:
		if q != nil {
			if e := q.FindEnum(ref.Name); e != nil {
				return schemaFromEnum(q, e)
			}
		}
		return basicSchema(ref.Name)
	}
}

func schemaFromEnum(q *astp.Query, e *astp.Enum) openapi.Schema {
	base := openapi.Schema{Type: "string"}
	if e != nil && e.Type != nil {
		base = schemaFromTypeRef(q, e.Type, nil)
	}
	if e == nil {
		return base
	}
	vals := make([]any, 0, len(e.Values))
	for _, v := range e.Values {
		if v == nil {
			continue
		}
		if strings.TrimSpace(v.Value) != "" {
			vals = append(vals, strings.TrimSpace(v.Value))
			continue
		}
		vals = append(vals, v.Name)
	}
	base.Enum = vals
	return base
}

func responseSchemaForMethod(q *astp.Query, method *astp.Func, typeArgs astp.TypeArgBinding) *openapi.Schema {
	if method == nil {
		return nil
	}
	rawResponse := hasRawResponseAnnotation(method)
	idx, hasOverride := responseIndexOverride(method)
	if hasOverride {
		if idx >= 0 && idx < len(method.Results) {
			r := method.Results[idx]
			if r != nil && r.Type != nil && !isErrorTypeRef(r.Type) {
				s := schemaFromTypeRef(q, instantiateResult(r.Type, typeArgs), typeArgs)
				if rawResponse {
					return &s
				}
				env := envelopeResponseSchema(s)
				return &env
			}
		}
		return nil
	}

	for _, r := range method.Results {
		if r == nil || r.Type == nil {
			continue
		}
		if isErrorTypeRef(r.Type) {
			continue
		}
		s := schemaFromTypeRef(q, instantiateResult(r.Type, typeArgs), typeArgs)
		if rawResponse {
			return &s
		}
		env := envelopeResponseSchema(s)
		return &env
	}
	return nil
}

func envelopeResponseSchema(data openapi.Schema) openapi.Schema {
	code := openapi.Schema{Type: "integer", Format: "int32", Default: 0, Example: 0}
	message := openapi.Schema{Type: "string", Default: "", Example: "ok"}
	trace := openapi.Schema{Type: "string", Default: "", Example: "trace-id"}
	data.Default = DefaultFromSchema(data, 0)
	data.Example = ExampleFromSchema(data, 0)
	return openapi.Schema{
		Type: "object",
		Required: []string{
			"code",
			"message",
		},
		Properties: map[string]openapi.Schema{
			"code":     code,
			"message":  message,
			"data":     data,
			"trace_id": trace,
		},
		Default: map[string]any{
			"code":     0,
			"message":  "ok",
			"data":     data.Default,
			"trace_id": "",
		},
		Example: map[string]any{
			"code":     0,
			"message":  "ok",
			"data":     data.Example,
			"trace_id": "trace-id",
		},
	}
}

func responseIndexOverride(method *astp.Func) (int, bool) {
	if method == nil || method.Doc == nil {
		return 0, false
	}
	for _, ann := range method.Doc.ParsedAnnotations {
		if ann == nil {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(ann.Name), "Response") {
			continue
		}
		if idx, ok := parseResponseIndex(ann); ok {
			return idx, true
		}
	}
	return 0, false
}

func parseResponseIndex(ann *astp.Annotation) (int, bool) {
	if ann == nil {
		return 0, false
	}
	if v, ok := ann.KV["index"]; ok {
		if idx, ok := parseIndexValue(v); ok {
			return idx, true
		}
	}
	if v, ok := ann.KV["result"]; ok {
		if idx, ok := parseIndexValue(v); ok {
			return idx, true
		}
	}
	if len(ann.Args) > 0 {
		if idx, ok := parseIndexValue(ann.Args[0]); ok {
			return idx, true
		}
	}
	return 0, false
}

func parseIndexValue(v string) (int, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}
	if n < 0 {
		return 0, false
	}
	if n == 0 {
		return 0, true
	}
	return n - 1, true
}

func isErrorTypeRef(t *astp.TypeRef) bool {
	if t == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(t.Name), "error")
}

func entityDescription(name string, doc *astp.CommentGroup) string {
	if doc == nil {
		return ""
	}
	prefix := strings.TrimSpace(name)
	for _, c := range doc.List {
		if c == nil {
			continue
		}
		line := strings.TrimSpace(c.Text)
		if line == "" || strings.HasPrefix(line, "@") {
			continue
		}
		if prefix != "" && strings.HasPrefix(line, prefix+" ") {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
		if prefix != "" && line == prefix {
			continue
		}
		return line
	}
	return ""
}

func basicSchema(name string) openapi.Schema {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "bool":
		return openapi.Schema{Type: "boolean"}
	case "int", "int8", "int16", "int32", "uint", "uint8", "uint16", "uint32":
		return openapi.Schema{Type: "integer", Format: "int32"}
	case "int64", "uint64":
		return openapi.Schema{Type: "integer", Format: "int64"}
	case "float32":
		return openapi.Schema{Type: "number", Format: "float"}
	case "float64":
		return openapi.Schema{Type: "number", Format: "double"}
	case "string":
		return openapi.Schema{Type: "string"}
	case "time", "time.time":
		return openapi.Schema{Type: "string", Format: "date-time"}
	default:
		return openapi.Schema{Type: "string"}
	}
}

func fieldDescription(f *astp.Field) string {
	if f == nil || f.Doc == nil {
		return ""
	}
	var lines []string
	for _, c := range f.Doc.List {
		if c == nil {
			continue
		}
		text := strings.TrimSpace(c.Text)
		if text == "" || strings.HasPrefix(text, "@") {
			continue
		}
		lines = append(lines, text)
	}
	return strings.Join(lines, " ")
}

func paramDescription(p *astp.Param) string {
	if p == nil || p.Type == nil {
		return ""
	}
	return ""
}

func isFieldRequired(f *astp.Field, source BindSource) bool {
	if f == nil || f.Type == nil {
		return false
	}
	if isPointerTypeRef(f.Type) {
		return false
	}
	if f.Tag != nil {
		if hasRequiredTag(f.Tag["validate"]) {
			return true
		}
		if hasRequiredTag(f.Tag["binding"]) {
			return true
		}
		if hasRequiredTag(f.Tag["required"]) {
			return true
		}
		if source == BindBody {
			if v := strings.TrimSpace(f.Tag["json"]); v == "-" {
				return false
			}
			if strings.Contains(strings.ToLower(f.Tag["json"]), "omitempty") {
				return false
			}
		}
	}
	return source == BindPath
}

func hasRequiredTag(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return false
	}
	if v == "true" || v == "required" {
		return true
	}
	for _, part := range strings.Split(v, ",") {
		if strings.TrimSpace(part) == "required" {
			return true
		}
	}
	return false
}

func isPointerTypeRef(ref *astp.TypeRef) bool {
	if ref == nil {
		return false
	}
	if ref.Kind == astp.KindPointer {
		return true
	}
	if ref.ElemType != nil && ref.Kind == astp.KindPointer {
		return true
	}
	return false
}

// nestedBinding 在字段类型里下钻查找「被实例化的泛型类型」, 并返回重建后的绑定.
//
// 泛型实参的位置不固定: 可能在顶层(PageSize[*T]), 也可能藏在切片元素([]T)、
// map 值(map[string]PageSize[T])里. 这里按 Generic.Args / ElemType / KeyType 的顺序
// 找到第一个带实参且本模块内可解析的类型, 用它自身的形参重新配对.
//
// 找不到时返回 nil, 调用方沿用外层绑定.
func nestedBinding(q *astp.Query, ref *astp.TypeRef, outer astp.TypeArgBinding) astp.TypeArgBinding {
	if q == nil || ref == nil {
		return nil
	}

	// 深度优先, 带访问集合避免自引用泛型死循环.
	seen := map[*astp.TypeRef]bool{}
	var walk func(*astp.TypeRef) astp.TypeArgBinding
	walk = func(r *astp.TypeRef) astp.TypeArgBinding {
		if r == nil || seen[r] {
			return nil
		}
		seen[r] = true

		if r.Generic != nil && len(r.Generic.Args) > 0 && r.Name != "" {
			if t := q.FindType(r.Name); t != nil {
				if inner := astp.RebindTypeArgs(t, r.Generic.Args); len(inner) > 0 {
					return inner
				}
			}
		}
		for _, arg := range genericArgs(r) {
			if inner := walk(arg); inner != nil {
				return inner
			}
		}
		if inner := walk(r.ElemType); inner != nil {
			return inner
		}
		if inner := walk(r.KeyType); inner != nil {
			return inner
		}
		return nil
	}

	return walk(ref)
}

// genericArgs 返回 ref 上的泛型实参列表.
func genericArgs(ref *astp.TypeRef) []*astp.TypeRef {
	if ref == nil || ref.Generic == nil {
		return nil
	}
	return ref.Generic.Args
}

// instantiateResult 按实参绑定实例化返回值类型, 让泛型占位(T / *T)变成真实类型.
// 没有绑定时原样返回, 保持零成本.
func instantiateResult(ref *astp.TypeRef, bind astp.TypeArgBinding) *astp.TypeRef {
	if len(bind) == 0 {
		return ref
	}
	return astp.InstantiateTypeRef(ref, bind)
}
