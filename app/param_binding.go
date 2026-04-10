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

func buildParamHints(q *astp.Query, method *astp.Func, pathParamSet map[string]struct{}) map[string]ParamHint {
	hints := make(map[string]ParamHint)
	for _, p := range method.Params {
		if p == nil || p.Name == "" {
			continue
		}
		if isContextParam(p.Type) {
			continue
		}
		source := inferBindSource(q, p, pathParamSet)
		hints[p.Name] = ParamHint{Name: p.Name, Source: source}
	}
	return hints
}

func inferBindSource(q *astp.Query, p *astp.Param, pathParamSet map[string]struct{}) BindSource {
	if p == nil || p.Type == nil {
		return BindQuery
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

func buildOpenAPIForMethod(q *astp.Query, method *astp.Func, hints map[string]ParamHint) ([]openapi.Parameter, *openapi.RequestBody) {
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
		resolved := q.ResolveParamType(p)
		schema := schemaFromTypeRef(q, p.Type)
		if resolved != nil && resolved.Kind == astp.KindStruct {
			switch hint.Source {
			case BindQuery, BindPath, BindHeader:
				for _, f := range resolved.Fields {
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
						Schema:      schemaFromTypeRef(q, f.Type),
					})
				}
			case BindBody:
				s := schemaFromStructType(q, resolved)
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
			"application/json": {Schema: *bodySchema},
		},
	}
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

func schemaFromStructType(q *astp.Query, t *astp.Type) openapi.Schema {
	props := make(map[string]openapi.Schema)
	var required []string
	if t != nil {
		for _, f := range t.Fields {
			if f == nil || f.Type == nil || f.Name == "" {
				continue
			}
			name := fieldNameForSource(f, BindBody)
			if name == "" {
				continue
			}
			s := schemaFromTypeRef(q, f.Type)
			s.Description = fieldDescription(f)
			props[name] = s
			if isFieldRequired(f, BindBody) {
				required = append(required, name)
			}
		}
	}
	sort.Strings(required)
	return openapi.Schema{Type: "object", Properties: props, Required: required}
}

func schemaFromTypeRef(q *astp.Query, ref *astp.TypeRef) openapi.Schema {
	if ref == nil {
		return openapi.Schema{Type: "string"}
	}
	switch ref.Kind {
	case astp.KindSlice:
		item := schemaFromTypeRef(q, ref.ElemType)
		return openapi.Schema{Type: "array", Items: &item}
	case astp.KindMap:
		val := schemaFromTypeRef(q, ref.ElemType)
		return openapi.Schema{Type: "object", AdditionalProperties: &val}
	case astp.KindStruct, astp.KindPointer:
		if q != nil {
			if t := q.ResolveTypeRef(ref); t != nil {
				if t.Kind == astp.KindStruct {
					s := schemaFromStructType(q, t)
					return s
				}
				if t.Kind == astp.KindEnum {
					if e := q.FindEnum(t.Name); e != nil {
						return schemaFromEnum(q, e)
					}
				}
			}
		}
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
		base = schemaFromTypeRef(q, e.Type)
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

func responseSchemaForMethod(q *astp.Query, method *astp.Func) *openapi.Schema {
	if method == nil {
		return nil
	}
	rawResponse := hasRawResponseAnnotation(method)
	idx, hasOverride := responseIndexOverride(method)
	if hasOverride {
		if idx >= 0 && idx < len(method.Results) {
			r := method.Results[idx]
			if r != nil && r.Type != nil && !isErrorTypeRef(r.Type) {
				s := schemaFromTypeRef(q, r.Type)
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
		s := schemaFromTypeRef(q, r.Type)
		if rawResponse {
			return &s
		}
		env := envelopeResponseSchema(s)
		return &env
	}
	return nil
}

func envelopeResponseSchema(data openapi.Schema) openapi.Schema {
	return openapi.Schema{
		Type: "object",
		Required: []string{
			"code",
			"message",
		},
		Properties: map[string]openapi.Schema{
			"code": {
				Type:   "integer",
				Format: "int32",
			},
			"message": {
				Type: "string",
			},
			"data": data,
			"trace_id": {
				Type: "string",
			},
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
