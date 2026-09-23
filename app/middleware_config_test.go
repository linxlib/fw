package app

import (
	"os"
	"path/filepath"
	"testing"
)

const middlewareConfigYAML = `middlewares:
  Authorization:
    API-Key: "secret"
    TTL: 30
`

// TestMiddlewareConfigCaseInsensitive 保证中间件配置段的大小写不敏感取值真实生效.
//
// 回归背景: EngineConfig.Middlewares 原先直接由 yaml.v3 解码, config.Section 未实现
// UnmarshalYAML, 于是段内 key 保留配置文件里的大小写, m.Config.Get("api-key") 取不到
// YAML 中写作 API-Key 的项; 同时 Engine.middlewareConfig 只按小写精确命中 map key,
// YAML 里写作 Authorization 时整段都取不到. 两者都违背 config.Section 文档承诺的
// "key 与 Section 内的 key 都按小写去空白比较".
func TestMiddlewareConfigCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(file, []byte(middlewareConfigYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	e, err := New(file)
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}

	sec := e.middlewareConfig("Authorization")
	if got := sec.Get("api-key"); got != "secret" {
		t.Errorf("Get(api-key) = %q, want %q", got, "secret")
	}
	if got := sec.Get("API-Key"); got != "secret" {
		t.Errorf("Get(API-Key) = %q, want %q", got, "secret")
	}
	if got := sec.Get("ttl"); got != "30" {
		t.Errorf("Get(ttl) = %q, want %q", got, "30")
	}
	if !sec.Has("api-key") {
		t.Errorf("Has(api-key) = false, want true")
	}

	// 查询用的中间件名大小写不影响命中.
	for _, name := range []string{"authorization", "Authorization", "AUTHORIZATION"} {
		s := e.middlewareConfig(name)
		if got := s.Get("api-key"); got != "secret" {
			t.Errorf("middlewareConfig(%q).Get(api-key) = %q, want %q", name, got, "secret")
		}
	}

	// 未配置的中间件仍返回空 Section, 调用方无需判空.
	empty := e.middlewareConfig("Unknown")
	if got := empty.Get("api-key"); got != "" {
		t.Errorf("Get on missing section = %q, want %q", got, "")
	}
	if empty.Has("api-key") {
		t.Errorf("Has on missing section = true, want false")
	}
}

// TestMiddlewareConfigLowercaseKeysUnchanged 保证既有的全小写写法行为不变.
func TestMiddlewareConfigLowercaseKeysUnchanged(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	const yamlText = "middlewares:\n  authorization:\n    api-key: \"secret\"\n    ttl: 30\n"
	if err := os.WriteFile(file, []byte(yamlText), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	e, err := New(file)
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	sec := e.middlewareConfig("authorization")
	if got := sec.Get("api-key"); got != "secret" {
		t.Errorf("Get(api-key) = %q, want %q", got, "secret")
	}
	if got := sec.Get("ttl"); got != "30" {
		t.Errorf("Get(ttl) = %q, want %q", got, "30")
	}
}
