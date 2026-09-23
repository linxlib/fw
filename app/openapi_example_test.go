package app

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/fasthttp/router"
	"github.com/linxlib/fw/v2/astp"
	"github.com/linxlib/fw/v2/inject"
	"github.com/linxlib/fw/v2/logger"
	"github.com/linxlib/fw/v2/middleware"
	"github.com/linxlib/fw/v2/openapi"
	"github.com/linxlib/fw/v2/response"
)

// newOpenAPITestEngine 构造一个能跑 collectRoutes 的引擎(基于 astp/testdata fixture).
func newOpenAPITestEngine(t *testing.T) *Engine {
	t.Helper()
	absDir, err := filepath.Abs(filepath.Join("..", "astp", "testdata"))
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	project, err := astp.NewParser().Parse(absDir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	logg, err := logger.New("info", "console", "", false)
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	cfg := DefaultEngineConfig()
	return &Engine{
		cfg:             &cfg,
		log:             logg,
		router:          router.New(),
		globalContainer: inject.New(),
		responseManager: response.NewManager(nil, nil),
		controllers:     make(map[string]reflect.Value),
		middlewares:     make(map[string]middleware.Middleware),
		project:         project,
	}
}

// findGenericMethod 按名字在 owner 的方法列表里找方法.
func findGenericMethod(t *testing.T, owner *astp.Type, name string) *astp.Func {
	t.Helper()
	for _, m := range owner.Methods {
		if m != nil && m.Name == name {
			return m
		}
	}
	t.Fatalf("method %s not found on %s", name, owner.Name)
	return nil
}

// TestGenericResponseSchemaExpands 验证泛型返回值能展开成真实实体字段, 而不是 {}.
func TestGenericResponseSchemaExpands(t *testing.T) {
	e := newOpenAPITestEngine(t)
	q := astp.NewQuery(e.project)
	owner := q.FindType("UserCtrl")
	if owner == nil {
		t.Fatal("expected to find UserCtrl in fixture")
	}

	t.Run("PageSize[*T] 展开为 Member 字段", func(t *testing.T) {
		m := findGenericMethod(t, owner, "Page")
		typeArgs := astp.RecvTypeArgs(q, owner, m)
		if len(typeArgs) == 0 {
			t.Fatal("expected a type arg binding")
		}
		// responseSchemaForMethod 返回的是整个响应(默认已包上 envelope),
		// 真正的业务数据在 data 下.
		data := dataSchema(t, responseSchemaForMethod(q, m, typeArgs))
		items, ok := data.Properties["items"]
		if !ok {
			t.Fatalf("PageSize has no items: %+v", data.Properties)
		}
		if items.Type != "array" || items.Items == nil {
			t.Fatalf("items = %+v, want array with items", items)
		}
		for _, field := range []string{"id", "name", "email"} {
			if _, ok := items.Items.Properties[field]; !ok {
				t.Errorf("Member field %q missing; got %v", field, items.Items.Properties)
			}
		}
	})

	t.Run("裸类型参数 T 展开为 Member", func(t *testing.T) {
		m := findGenericMethod(t, owner, "One")
		typeArgs := astp.RecvTypeArgs(q, owner, m)
		data := dataSchema(t, responseSchemaForMethod(q, m, typeArgs))
		if data.Type != "object" || len(data.Properties) == 0 {
			t.Fatalf("data = %+v, want an object with Member fields", data)
		}
		if _, ok := data.Properties["email"]; !ok {
			t.Errorf("Member.email missing: %+v", data.Properties)
		}
	})

	t.Run("多级嵌套 PageSize[Account[int]] 展开", func(t *testing.T) {
		m := findGenericMethod(t, owner, "PlainPage")
		typeArgs := astp.RecvTypeArgs(q, owner, m)
		data := dataSchema(t, responseSchemaForMethod(q, m, typeArgs))
		items, ok := data.Properties["items"]
		if !ok || items.Items == nil {
			t.Fatalf("items = %+v, want array with items", items)
		}
		// Account 的字段: code(int, 来自 Account[int]) 与 desc(string)
		account := items.Items.Properties
		code, ok := account["code"]
		if !ok {
			t.Fatalf("Account.code missing: %+v", account)
		}
		if code.Type != "integer" {
			t.Errorf("Account.code type = %q, want integer (from Account[int])", code.Type)
		}
		if desc, ok := account["desc"]; !ok || desc.Type != "string" {
			t.Errorf("Account.desc = %+v, want a string", desc)
		}
	})
}

// dataSchema 从响应 schema 里取出业务数据部分: 已包 envelope 时取 data,
// 未包(@RawResponse)时本身就是数据.
func dataSchema(t *testing.T, schema *openapi.Schema) openapi.Schema {
	t.Helper()
	if schema == nil {
		t.Fatal("expected a response schema")
	}
	if data, ok := schema.Properties["data"]; ok {
		return data
	}
	return *schema
}

// TestTypeDefaults 逐项断言类型缺省值表.
func TestTypeDefaults(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		s := basicSchema("string")
		if got := DefaultFromSchema(s, 0); got != "" {
			t.Errorf("default = %v, want \"\"", got)
		}
		if got := ExampleFromSchema(s, 0); got != "string" {
			t.Errorf("example = %v, want \"string\"", got)
		}
	})

	t.Run("integers", func(t *testing.T) {
		for _, name := range []string{"int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64"} {
			s := basicSchema(name)
			if got := DefaultFromSchema(s, 0); got != 0 {
				t.Errorf("%s default = %v, want 0", name, got)
			}
			if got := ExampleFromSchema(s, 0); got != 1 {
				t.Errorf("%s example = %v, want 1", name, got)
			}
		}
	})

	t.Run("floats", func(t *testing.T) {
		for _, name := range []string{"float32", "float64"} {
			s := basicSchema(name)
			if got := DefaultFromSchema(s, 0); got != 0 {
				t.Errorf("%s default = %v, want 0", name, got)
			}
			if got := ExampleFromSchema(s, 0); got != 1.5 {
				t.Errorf("%s example = %v, want 1.5", name, got)
			}
		}
	})

	t.Run("bool", func(t *testing.T) {
		s := basicSchema("bool")
		if got := DefaultFromSchema(s, 0); got != false {
			t.Errorf("default = %v, want false", got)
		}
		if got := ExampleFromSchema(s, 0); got != true {
			t.Errorf("example = %v, want true", got)
		}
	})

	t.Run("object 与 array", func(t *testing.T) {
		obj := openapi.Schema{Type: "object", Properties: map[string]openapi.Schema{
			"name": {Type: "string"},
		}}
		def, ok := DefaultFromSchema(obj, 0).(map[string]any)
		if !ok {
			t.Fatalf("object default = %v, want a map", def)
		}
		if def["name"] != "" {
			t.Errorf("object default name = %v, want \"\"", def["name"])
		}
		ex, ok := ExampleFromSchema(obj, 0).(map[string]any)
		if !ok {
			t.Fatalf("object example = %v, want a map", ex)
		}
		if ex["name"] != "string" {
			t.Errorf("object example name = %v, want \"string\"", ex["name"])
		}

		arr := openapi.Schema{Type: "array", Items: &openapi.Schema{Type: "string"}}
		if got := DefaultFromSchema(arr, 0); len(got.([]any)) != 0 {
			t.Errorf("array default = %v, want []", got)
		}
		exArr, ok := ExampleFromSchema(arr, 0).([]any)
		if !ok || len(exArr) != 1 || exArr[0] != "string" {
			t.Errorf("array example = %v, want [\"string\"]", exArr)
		}
	})

	t.Run("date-time format", func(t *testing.T) {
		s := openapi.Schema{Type: "string", Format: "date-time"}
		if got := ExampleFromSchema(s, 0); got != "2024-01-01T00:00:00Z" {
			t.Errorf("example = %v, want an RFC3339 timestamp", got)
		}
	})
}

// TestExampleRecursion 验证 example 是完整可序列化的 JSON, 且字段齐全.
func TestExampleRecursion(t *testing.T) {
	s := openapi.Schema{
		Type: "object",
		Properties: map[string]openapi.Schema{
			"id":    {Type: "integer", Format: "int64"},
			"name":  {Type: "string"},
			"email": {Type: "string"},
			"tags":  {Type: "array", Items: &openapi.Schema{Type: "string"}},
		},
	}
	got := ExampleFromSchema(s, 0)
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal example: %v", err)
	}
	body := string(b)
	for _, want := range []string{`"name":"string"`, `"id":1`, `"tags":["string"]`} {
		if !strings.Contains(body, want) {
			t.Errorf("example = %s, want it to contain %s", body, want)
		}
	}
}

// TestTagBeatsTypeDefault 验证 tag 主动标注优先于类型占位值.
func TestTagBeatsTypeDefault(t *testing.T) {
	f := &astp.Field{
		Name: "Page",
		Type: &astp.TypeRef{Name: "int", Kind: astp.KindBasic},
		Tag:  map[string]string{"default": "1", "example": "1"},
	}
	s := basicSchema("int")
	applyFieldTags(f, &s)

	if s.Default != int64(1) {
		t.Errorf("default = %v (%T), want int64(1) from the tag", s.Default, s.Default)
	}
	if s.Example != int64(1) {
		t.Errorf("example = %v (%T), want int64(1) from the tag", s.Example, s.Example)
	}
}

// TestTagValueConversions 覆盖 tag 值按类型的转换.
func TestTagValueConversions(t *testing.T) {
	if got := tagValue("42", basicSchema("int")); got != int64(42) {
		t.Errorf("int tag = %v (%T), want int64(42)", got, got)
	}
	if got := tagValue("1.5", basicSchema("float64")); got != 1.5 {
		t.Errorf("float tag = %v (%T), want 1.5", got, got)
	}
	if got := tagValue("true", basicSchema("bool")); got != true {
		t.Errorf("bool tag = %v (%T), want true", got, got)
	}
	if got := tagValue("hello", basicSchema("string")); got != "hello" {
		t.Errorf("string tag = %v, want hello", got)
	}
	// 转不了就保持字符串, 不报错
	if got := tagValue("not-a-number", basicSchema("int")); got != "not-a-number" {
		t.Errorf("bad int tag = %v, want the raw string", got)
	}
}

// TestFieldTagCaseInsensitive tag key 大小写不敏感.
func TestFieldTagCaseInsensitive(t *testing.T) {
	f := &astp.Field{
		Name: "Page",
		Type: &astp.TypeRef{Name: "int", Kind: astp.KindBasic},
		Tag:  map[string]string{"Default": "2", "EXAMPLE": "9"},
	}
	s := basicSchema("int")
	applyFieldTags(f, &s)
	if s.Default != int64(2) {
		t.Errorf("default = %v, want int64(2) from the Default tag", s.Default)
	}
	if s.Example != int64(9) {
		t.Errorf("example = %v, want int64(9) from the EXAMPLE tag", s.Example)
	}
}

// TestRequestBodyCarriesExample 验证请求体的 MediaType 带上完整 example.
func TestRequestBodyCarriesExample(t *testing.T) {
	e := newOpenAPITestEngine(t)
	q := astp.NewQuery(e.project)
	owner := q.FindType("UserCtrl")
	m := findGenericMethod(t, owner, "Create")

	hints := map[string]ParamHint{
		"entity": {Name: "entity", Source: BindBody},
	}
	typeArgs := astp.RecvTypeArgs(q, owner, m)
	_, body := buildOpenAPIForMethod(q, m, hints, typeArgs)
	if body == nil {
		t.Fatal("expected a request body")
	}
	mt, ok := body.Content["application/json"]
	if !ok {
		t.Fatalf("no application/json content: %+v", body.Content)
	}
	if mt.Example == nil {
		t.Fatal("request body example is nil")
	}
	b, err := json.Marshal(mt.Example)
	if err != nil {
		t.Fatalf("marshal example: %v", err)
	}
	if !strings.Contains(string(b), `"email"`) {
		t.Errorf("request example = %s, want it to contain the Member email field", b)
	}
}
