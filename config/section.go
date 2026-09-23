package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Section 是一份无 schema 的键值配置段(map[string]any), 提供大小写不敏感的取值.
//
// 适用于按名字注入的中间件/插件配置: 配置文件中没有对应段时它是一个空 Section,
// Get 返回零值, 调用方无需判空.
//
//	// config/app.yaml
//	// middlewares:
//	//   authorization:
//	//     api-key: "secret"
//
//	type AuthorizationMiddleware struct {
//	    Config *config.Section `inject:""`
//	}
//
//	func (m *AuthorizationMiddleware) Handle(...) error {
//	    if m.Config.Get("api-key") == "" { ... }
//	}
type Section map[string]any

// UnmarshalYAML 让 Section 可被 yaml.v3 直接解码, 并在解码后经 NewSection 归一化
// key. 缺少本实现时 yaml 按 map 原样解码, key 会保留配置文件里的大小写, 导致
// Get("api-key") 取不到 YAML 中写作 API-Key 的项. 注意 null 节点不会进入本方法
// (yaml.v3 会提前短路), 此时 Section 保持零值 nil, 而 Get/Has 对 nil 依然安全.
func (s *Section) UnmarshalYAML(value *yaml.Node) error {
	var raw map[string]any
	if err := value.Decode(&raw); err != nil {
		return err
	}
	*s = NewSection(raw)
	return nil
}

// NewSection 由普通 map 构造 Section, key 统一归一化为小写去空白形式.
// 传入 nil 时返回空 Section(非 nil), 方便调用方直接取值.
func NewSection(m map[string]any) Section {
	out := make(Section, len(m))
	for k, v := range m {
		out[normalizeKey(k)] = v
	}
	return out
}

// Get 返回 key 对应的值的字符串形式, 缺失或为 nil 时返回 "".
// key 与 Section 内的 key 都按小写去空白比较, 因此 "api-key" / "API-Key" 等价.
func (s Section) Get(key string) string {
	if len(s) == 0 {
		return ""
	}
	v, ok := s[normalizeKey(key)]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case bool:
		return fmt.Sprintf("%t", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// Has 判断 key 是否存在(同样大小写不敏感).
func (s Section) Has(key string) bool {
	if len(s) == 0 {
		return false
	}
	_, ok := s[normalizeKey(key)]
	return ok
}

// normalizeKey 归一化配置 key: 去空白 + 小写.
func normalizeKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}
