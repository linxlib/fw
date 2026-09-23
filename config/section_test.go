package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// engineConfigLike 模仿 app.EngineConfig 的 middlewares 段: map[string]config.Section.
type engineConfigLike struct {
	Middlewares map[string]Section `yaml:"middlewares"`
}

const mixedCaseYAML = `middlewares:
  Authorization:
    API-Key: "secret"
    TTL: 30
`

// TestSectionUnmarshalNormalizesKeys 验证: 从 YAML 解码出的 Section 会归一化 key,
// 因此 Get/Has 命中配置文件中任意大小写写法的 key.
func TestSectionUnmarshalNormalizesKeys(t *testing.T) {
	var cfg engineConfigLike
	if err := yaml.Unmarshal([]byte(mixedCaseYAML), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	sec, ok := cfg.Middlewares["Authorization"]
	if !ok {
		t.Fatalf("middleware section not found, got keys %v", keysOf(cfg.Middlewares))
	}

	cases := []struct {
		key  string
		want string
	}{
		{"api-key", "secret"},
		{"API-Key", "secret"},
		{"Api-Key", "secret"},
		{"ttl", "30"},
		{"api_key", ""}, // "-" 与 "_" 不互相归一化, 与文档约定一致
		{"TTL", "30"},
		{"missing", ""},
	}
	for _, c := range cases {
		if got := sec.Get(c.key); got != c.want {
			t.Errorf("Get(%q) = %q, want %q", c.key, got, c.want)
		}
	}
	for _, key := range []string{"api-key", "API-Key", "ttl"} {
		if !sec.Has(key) {
			t.Errorf("Has(%q) = false, want true", key)
		}
	}
	if sec.Has("missing") {
		t.Errorf("Has(%q) = true, want false", "missing")
	}
}

// TestSectionUnmarshalNullNode 验证: YAML 空节点(如 "middlewares:\n  authorization:") 下,
// yaml.v3 不会调用 UnmarshalYAML, Section 保持零值 nil; Get/Has 对 nil 依然安全,
// 因此中间件无需判空. 这与 Engine.middlewareConfig 缺失配置段时的返回等价.
func TestSectionUnmarshalNullNode(t *testing.T) {
	var cfg engineConfigLike
	const yamlText = "middlewares:\n  authorization:\n"
	if err := yaml.Unmarshal([]byte(yamlText), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	sec, ok := cfg.Middlewares["authorization"]
	if !ok {
		t.Fatalf("middleware key not present, got %v", keysOf(cfg.Middlewares))
	}
	if len(sec) != 0 {
		t.Fatalf("len(Section) = %d, want 0", len(sec))
	}
	if got := sec.Get("api-key"); got != "" {
		t.Errorf("Get on nil Section = %q, want %q", got, "")
	}
	if sec.Has("api-key") {
		t.Errorf("Has on nil Section = true, want false")
	}
}

// TestSectionUnmarshalLowercaseKeysUnchanged 验证: 本来就全小写的 key 行为不变.
func TestSectionUnmarshalLowercaseKeysUnchanged(t *testing.T) {
	const yamlText = "api-key: secret\nttl: 30\n"
	var s Section
	if err := yaml.Unmarshal([]byte(yamlText), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := s.Get("api-key"); got != "secret" {
		t.Errorf("Get(api-key) = %q, want %q", got, "secret")
	}
	if got := s.Get("ttl"); got != "30" {
		t.Errorf("Get(ttl) = %q, want %q", got, "30")
	}
}

// TestNewSectionMatchesUnmarshal 验证: NewSection 与 YAML 解码路径对同一份数据的
// key 归一化结果一致, 两条入口不存在行为分歧.
func TestNewSectionMatchesUnmarshal(t *testing.T) {
	const yamlText = "API-Key: secret\nTTL: 30\n"
	var fromYAML Section
	if err := yaml.Unmarshal([]byte(yamlText), &fromYAML); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	fromMap := NewSection(map[string]any{"API-Key": "secret", "TTL": 30})
	if len(fromYAML) != len(fromMap) {
		t.Fatalf("length mismatch: yaml %d, map %d", len(fromYAML), len(fromMap))
	}
	for k, v := range fromMap {
		got, ok := fromYAML[k]
		if !ok {
			t.Errorf("key %q missing from yaml-decoded Section", k)
			continue
		}
		if got != v {
			t.Errorf("key %q: yaml %v, map %v", k, got, v)
		}
	}
}

func keysOf(m map[string]Section) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
