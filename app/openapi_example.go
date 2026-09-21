package app

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/linxlib/fw/v2/astp"
	"github.com/linxlib/fw/v2/openapi"
)

// maxExampleDepth 限制 example/default 生成的递归深度, 防止自引用结构无限展开.
const maxExampleDepth = 6

// applyFieldTags 把 model 字段上的 example / default tag 应用到 schema 上.
//
// 优先级: tag 主动标注 > 按类型生成的占位值. tag 是显式意图, 优先.
func applyFieldTags(f *astp.Field, s *openapi.Schema) {
	if f == nil || s == nil {
		return
	}

	if raw, ok := fieldTag(f, "example"); ok {
		s.Example = tagValue(raw, *s)
	} else {
		s.Example = ExampleFromSchema(*s, 0)
	}

	if raw, ok := fieldTag(f, "default"); ok {
		s.Default = tagValue(raw, *s)
	} else {
		s.Default = DefaultFromSchema(*s, 0)
	}
}

// fieldTag 读取字段 tag 中指定 key 的值(大小写不敏感, 与 Go 惯例一致).
func fieldTag(f *astp.Field, key string) (string, bool) {
	if f == nil || f.Tag == nil {
		return "", false
	}
	for k, v := range f.Tag {
		if strings.EqualFold(strings.TrimSpace(k), key) {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}

// tagValue 把 tag 字符串按 schema 类型转成对应的 JSON 值.
//
// 数值按 int/float 解析成数字, bool 解析成布尔, JSON 字面量(对象/数组)尝试解析,
// 其余保持字符串. 转换失败时原样返回字符串, 不报错.
func tagValue(raw string, s openapi.Schema) any {
	if raw == "" {
		return raw
	}

	switch s.Type {
	case "integer":
		if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return n
		}
		if n, err := strconv.ParseFloat(raw, 64); err == nil {
			return n
		}
	case "number":
		if n, err := strconv.ParseFloat(raw, 64); err == nil {
			return n
		}
	case "boolean":
		if b, err := strconv.ParseBool(raw); err == nil {
			return b
		}
	}

	// 尝试按 JSON 字面量解析(对象/数组/带引号字符串), 失败则保持字符串.
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err == nil {
		switch v.(type) {
		case map[string]any, []any, string, float64, bool, nil:
			return v
		}
	}
	return raw
}

// ExampleFromSchema 按 schema 递归生成示例值(带深度上限).
//
// schema 自身已带 Example(即字段上的 example tag 已设置过)时优先采用,
// 否则按类型给占位示例: string -> "string", 整数 -> 1, 浮点 -> 1.5, bool -> true.
func ExampleFromSchema(s openapi.Schema, depth int) any {
	if depth > maxExampleDepth {
		return nil
	}
	if s.Example != nil {
		return s.Example
	}

	switch s.Type {
	case "object":
		if len(s.Properties) == 0 {
			// 无字段信息时给出空对象, 与 additionalProperties 形状区分.
			return map[string]any{}
		}
		out := make(map[string]any, len(s.Properties))
		for name, p := range s.Properties {
			out[name] = ExampleFromSchema(p, depth+1)
		}
		return out
	case "array":
		if s.Items == nil {
			return []any{}
		}
		return []any{ExampleFromSchema(*s.Items, depth+1)}
	case "string":
		return exampleString(s)
	case "integer":
		if len(s.Enum) > 0 {
			return s.Enum[0]
		}
		return 1
	case "number":
		if len(s.Enum) > 0 {
			return s.Enum[0]
		}
		return 1.5
	case "boolean":
		if len(s.Enum) > 0 {
			return s.Enum[0]
		}
		return true
	default:
		if s.AdditionalProperties != nil {
			return map[string]any{"key": ExampleFromSchema(*s.AdditionalProperties, depth+1)}
		}
		return nil
	}
}

// DefaultFromSchema 按 schema 递归生成默认值(带深度上限).
//
// 类型缺省值规则: string -> "", 整数 -> 0, 浮点 -> 0, bool -> false,
// 对象 -> {字段: 默认值}, 数组 -> [], map -> {}.
// schema 自身已带 Default(字段上的 default tag 已设置过)时优先采用.
func DefaultFromSchema(s openapi.Schema, depth int) any {
	if depth > maxExampleDepth {
		return nil
	}
	if s.Default != nil {
		return s.Default
	}

	switch s.Type {
	case "object":
		if len(s.Properties) == 0 {
			return map[string]any{}
		}
		out := make(map[string]any, len(s.Properties))
		for name, p := range s.Properties {
			out[name] = DefaultFromSchema(p, depth+1)
		}
		return out
	case "array":
		return []any{}
	case "string":
		return defaultString(s)
	case "integer":
		if len(s.Enum) > 0 {
			return s.Enum[0]
		}
		return 0
	case "number":
		if len(s.Enum) > 0 {
			return s.Enum[0]
		}
		return 0
	case "boolean":
		if len(s.Enum) > 0 {
			return s.Enum[0]
		}
		return false
	default:
		return nil
	}
}

// exampleString 生成字符串示例值: 有 enum 取第一个, 有 format 按 format 给, 否则 "string".
func exampleString(s openapi.Schema) any {
	if len(s.Enum) > 0 {
		return s.Enum[0]
	}
	switch strings.ToLower(strings.TrimSpace(s.Format)) {
	case "date-time":
		return "2024-01-01T00:00:00Z"
	case "date":
		return "2024-01-01"
	case "email":
		return "user@example.com"
	case "uuid":
		return "00000000-0000-0000-0000-000000000000"
	}
	return "string"
}

// defaultString 生成字符串缺省值: 有 enum 取第一个, 否则空字符串.
func defaultString(s openapi.Schema) any {
	if len(s.Enum) > 0 {
		return s.Enum[0]
	}
	return ""
}
