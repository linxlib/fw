package config

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// newTestConfig 在 t.TempDir 下创建 config.yaml 并返回指向它的 Config(Silent 模式).
func newTestConfig(t *testing.T, content string) (*Config, string) {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	c := New(&Option{
		Files:  []string{file},
		Silent: true,
	})
	return c, file
}

type serverOpt struct {
	Host    string `yaml:"host" default:"0.0.0.0"`
	Port    int    `yaml:"port" default:"8080"`
	Timeout int    `yaml:"timeout" default:"3"`
}

type redisOpt struct {
	Host string `yaml:"host" default:"127.0.0.1"`
	Port int    `yaml:"port" default:"6379"`
	DB   int    `yaml:"db" default:"0"`
}

const twoSections = `server:
  host: 10.0.0.1
  port: 9000
  timeout: 5

redis:
  host: 10.0.0.2
  port: 6380
  db: 3
`

func TestLoadWithKeyTopLevelSection(t *testing.T) {
	c, _ := newTestConfig(t, twoSections)
	var so serverOpt
	if err := c.LoadWithKey("server", &so); err != nil {
		t.Fatalf("LoadWithKey: %v", err)
	}
	if so.Host != "10.0.0.1" || so.Port != 9000 || so.Timeout != 5 {
		t.Fatalf("server section not loaded: %+v", so)
	}
	var ro redisOpt
	if err := c.LoadWithKey("redis", &ro); err != nil {
		t.Fatalf("LoadWithKey redis: %v", err)
	}
	if ro.Host != "10.0.0.2" || ro.Port != 6380 || ro.DB != 3 {
		t.Fatalf("redis section not loaded: %+v", ro)
	}
}

func TestLoadWithKeyDottedPathToPlainVar(t *testing.T) {
	c, _ := newTestConfig(t, twoSections)
	var port int
	if err := c.LoadWithKey("server.port", &port); err != nil {
		t.Fatalf("LoadWithKey dotted: %v", err)
	}
	if port != 9000 {
		t.Fatalf("dotted path not decoded: got %d, want 9000", port)
	}
	// 父结构体的某个字段也可作为 target
	var so serverOpt
	if err := c.LoadWithKey("server.port", &so.Port); err != nil {
		t.Fatalf("LoadWithKey into struct field: %v", err)
	}
	if so.Port != 9000 {
		t.Fatalf("struct field not decoded: got %d", so.Port)
	}
}

func TestLoadByTags(t *testing.T) {
	c, _ := newTestConfig(t, twoSections)

	type appOpt struct {
		Server *serverOpt `inject:"server"`
		Redis  *redisOpt  `inject:"redis"`
		Other  *serverOpt `inject:"-"`
	}
	var opt appOpt
	if err := c.LoadByTags(&opt); err != nil {
		t.Fatalf("LoadByTags: %v", err)
	}
	if opt.Server == nil || opt.Server.Port != 9000 {
		t.Fatalf("inject tag not honored: %+v", opt.Server)
	}
	if opt.Redis == nil || opt.Redis.DB != 3 {
		t.Fatalf("inject tag not honored: %+v", opt.Redis)
	}
	if opt.Other != nil {
		t.Fatalf("skipped field must stay nil, got %+v", opt.Other)
	}
}

func TestMissingSectionKeepsDefaults(t *testing.T) {
	c, _ := newTestConfig(t, twoSections)
	type onlyHost struct {
		Host string `yaml:"host" default:"fallback"`
		Port int    `yaml:"port" default:"4321"`
	}
	var o onlyHost
	if err := c.LoadWithKey("missing", &o); err != nil {
		t.Fatalf("LoadWithKey missing: %v", err)
	}
	if o.Host != "fallback" || o.Port != 4321 {
		t.Fatalf("defaults not applied for missing section: %+v", o)
	}
}

func TestRequiredField(t *testing.T) {
	c, _ := newTestConfig(t, twoSections)
	type strict struct {
		Token string `yaml:"token" required:"true"`
	}
	var o strict
	if err := c.LoadWithKey("redis", &o); err == nil {
		t.Fatalf("expected required error for absent token")
	}
}

func TestEnvOverride(t *testing.T) {
	c, _ := newTestConfig(t, twoSections)
	// 前缀派生: ENVPrefix(FW) + section(server) + 字段(PORT)
	t.Setenv("FW_SERVER_PORT", "1234")
	// 显式 env tag 优先于派生
	t.Setenv("MY_TIMEOUT", "99")

	type envOpt struct {
		Host    string `yaml:"host" env:"MY_HOST"`
		Port    int    `yaml:"port"`
		Timeout int    `yaml:"timeout"`
	}
	var o envOpt
	if err := c.LoadWithKey("server", &o); err != nil {
		t.Fatalf("LoadWithKey: %v", err)
	}
	if o.Port != 1234 {
		t.Fatalf("env prefix derivation failed: got %d, want 1234", o.Port)
	}
	// 未设置的显式 env tag 不影响 yaml 值
	if o.Host != "10.0.0.1" {
		t.Fatalf("yaml value lost: got %q", o.Host)
	}
}

func TestExplicitEnvTagWins(t *testing.T) {
	c, _ := newTestConfig(t, twoSections)
	t.Setenv("MY_HOST", "env-host")
	type envOpt struct {
		Host string `yaml:"host" env:"MY_HOST"`
	}
	var o envOpt
	if err := c.LoadWithKey("server", &o); err != nil {
		t.Fatalf("LoadWithKey: %v", err)
	}
	if o.Host != "env-host" {
		t.Fatalf("explicit env tag not applied: got %q", o.Host)
	}
}

func TestYmlSuffixRejected(t *testing.T) {
	dir := t.TempDir()
	yml := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(yml, []byte(twoSections), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	c := New(&Option{Files: []string{yml}, Silent: true})
	var so serverOpt
	err := c.LoadWithKey("server", &so)
	if err == nil {
		t.Fatalf("expected error for .yml file")
	}
	if !strings.Contains(err.Error(), ".yaml") {
		t.Fatalf("error should mention .yaml restriction: %v", err)
	}
}

// TestIncrementalReload 验证: 只改 server section 时, redis 目标不被重写,
// 回调只收到 "server".
func TestIncrementalReload(t *testing.T) {
	c, file := newTestConfig(t, twoSections)

	var so serverOpt
	var ro redisOpt
	if err := c.LoadWithKey("server", &so); err != nil {
		t.Fatalf("load server: %v", err)
	}
	if err := c.LoadWithKey("redis", &ro); err != nil {
		t.Fatalf("load redis: %v", err)
	}
	redisPtr := reflect.ValueOf(&ro).Pointer()

	var mu sync.Mutex
	var callbackKeys []string
	c.AutoReloadCallback = func(key string, config interface{}) {
		mu.Lock()
		callbackKeys = append(callbackKeys, key)
		mu.Unlock()
	}

	// 未修改文件时, Reload 不应触发任何目标重写
	if changed, err := c.Reload(); err != nil || len(changed) != 0 {
		t.Fatalf("no-op reload: changed=%v err=%v", changed, err)
	}
	if n := c.LastReloadedKeys(); n != 0 {
		t.Fatalf("no-op reload touched %d targets", n)
	}

	// 只修改 server section (redis 原样保留)
	content := strings.Replace(twoSections, "port: 9000", "port: 9100", 1)
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	// 确保 modtime 前进
	fut := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(file, fut, fut)

	changed, err := c.Reload()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reflect.DeepEqual(changed, []string{"server"}) {
		t.Fatalf("changed sections = %v, want [server]", changed)
	}
	if so.Port != 9100 {
		t.Fatalf("server target not updated: %+v", so)
	}
	// redis 目标值不变且指针未被替换(值类型无法验指针, 用值断言)
	if ro.Host != "10.0.0.2" || ro.Port != 6380 || ro.DB != 3 {
		t.Fatalf("redis target changed unexpectedly: %+v", ro)
	}
	_ = redisPtr
	if n := c.LastReloadedKeys(); n != 1 {
		t.Fatalf("expected exactly 1 target rewritten, got %d", n)
	}
	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(callbackKeys, []string{"server"}) {
		t.Fatalf("callbacks = %v, want [server]", callbackKeys)
	}
}

// TestReloadSectionRemoved 验证: section 被删除后, 对应目标回落到默认值.
func TestReloadSectionRemoved(t *testing.T) {
	c, file := newTestConfig(t, twoSections)
	var so serverOpt
	if err := c.LoadWithKey("server", &so); err != nil {
		t.Fatalf("load: %v", err)
	}
	if so.Port != 9000 {
		t.Fatalf("initial load failed: %+v", so)
	}

	content := strings.SplitN(twoSections, "redis:", 2)[0] // 去掉 redis section
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	fut := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(file, fut, fut)

	changed, err := c.Reload()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reflect.DeepEqual(changed, []string{"redis"}) {
		t.Fatalf("changed = %v, want [redis]", changed)
	}
	// server 未变
	if so.Port != 9000 {
		t.Fatalf("server target changed unexpectedly: %+v", so)
	}
}

// TestReloadPrintsDetectedNotice 验证: 检测到需要重载时在标准输出打印提示,
// 无变化与 Silent 模式下不打印.
func TestReloadPrintsDetectedNotice(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(file, []byte(twoSections), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	c := New(&Option{Files: []string{file}})
	var so serverOpt
	if err := c.LoadWithKey("server", &so); err != nil {
		t.Fatalf("load: %v", err)
	}

	// 文件无变化: 不打印任何内容
	if out, err := captureStdout(t, func() error { _, err := c.Reload(); return err }); err != nil {
		t.Fatalf("no-op reload: %v", err)
	} else if out != "" {
		t.Fatalf("no-op reload should print nothing, got %q", out)
	}

	// 修改 server section: 打印变化的 section 列表
	content := strings.Replace(twoSections, "port: 9000", "port: 9100", 1)
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	fut := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(file, fut, fut)

	out, err := captureStdout(t, func() error { _, err := c.Reload(); return err })
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	want := "config: reload detected, changed sections: [server]\n"
	if out != want {
		t.Fatalf("stdout = %q, want %q", out, want)
	}

	// Silent 模式下不再输出
	c.Silent = true
	content = strings.Replace(content, "port: 9100", "port: 9200", 1)
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	fut = time.Now().Add(3 * time.Second)
	_ = os.Chtimes(file, fut, fut)
	if out, err := captureStdout(t, func() error { _, err := c.Reload(); return err }); err != nil {
		t.Fatalf("silent reload: %v", err)
	} else if out != "" {
		t.Fatalf("silent mode should print nothing, got %q", out)
	}
}

// captureStdout 捕获 fn 执行期间写入标准输出的内容, 结束后还原 os.Stdout.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = orig
		_ = r.Close()
	}()
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	callErr := fn()
	_ = w.Close()
	return <-done, callErr
}

// TestAutoReloadPolling 验证轮询线程: 修改文件后 AutoReloadInterval 内自动生效,
// Close 后停止.
func TestAutoReloadPolling(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(file, []byte(twoSections), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	c := New(&Option{
		Files:              []string{file},
		Silent:             true,
		AutoReload:         true,
		AutoReloadInterval: 20 * time.Millisecond,
	})
	var so serverOpt
	if err := c.LoadWithKey("server", &so); err != nil {
		t.Fatalf("load: %v", err)
	}

	content := strings.Replace(twoSections, "port: 9000", "port: 9200", 1)
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	_ = os.Chtimes(file, time.Now().Add(2*time.Second), time.Now().Add(2*time.Second))
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if so.Port == 9200 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if so.Port != 9200 {
		t.Fatalf("auto reload did not take effect: %+v", so)
	}
	c.Close()
}

// TestWholeFileTarget 验证 key 为 "" 的全量目标: 任一 section 变化都会刷新.
func TestWholeFileTarget(t *testing.T) {
	c, file := newTestConfig(t, twoSections)
	type fullOpt struct {
		Server serverOpt `yaml:"server"`
		Redis  redisOpt  `yaml:"redis"`
	}
	var fo fullOpt
	if err := c.Load(&fo); err != nil {
		t.Fatalf("load: %v", err)
	}
	if fo.Server.Port != 9000 || fo.Redis.DB != 3 {
		t.Fatalf("whole-file load failed: %+v", fo)
	}
	content := strings.Replace(twoSections, "db: 3", "db: 7", 1)
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	fut := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(file, fut, fut)
	changed, err := c.Reload()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reflect.DeepEqual(changed, []string{"redis"}) {
		t.Fatalf("changed = %v, want [redis]", changed)
	}
	if fo.Redis.DB != 7 {
		t.Fatalf("whole-file target not refreshed: %+v", fo)
	}
}

// TestEnvFileOverlay 验证 config/config.<env>.yaml 的叠加覆盖.
func TestEnvFileOverlay(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "config.yaml")
	envFile := filepath.Join(dir, "config.test.yaml")
	if err := os.WriteFile(base, []byte(twoSections), 0o644); err != nil {
		t.Fatalf("write base: %v", err)
	}
	overlay := `server:
  port: 1111
`
	if err := os.WriteFile(envFile, []byte(overlay), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	// 显式指定环境, 避免依赖平台测试二进制的自动检测
	c := New(&Option{Files: []string{base}, Silent: true, Environment: "test"})
	var so serverOpt
	if err := c.LoadWithKey("server", &so); err != nil {
		t.Fatalf("load: %v", err)
	}
	if so.Host != "10.0.0.1" {
		t.Fatalf("base value lost: %+v", so)
	}
	if so.Port != 1111 {
		t.Fatalf("env overlay not applied: got %d, want 1111", so.Port)
	}
}

// TestLoadMissingFileUsesDefaults 验证文件不存在时仅用默认值与环境变量.
func TestLoadMissingFileUsesDefaults(t *testing.T) {
	c := New(&Option{Files: []string{filepath.Join(t.TempDir(), "absent.yaml")}, Silent: true})
	var so serverOpt
	if err := c.LoadWithKey("server", &so); err != nil {
		t.Fatalf("load with missing file: %v", err)
	}
	if so.Host != "0.0.0.0" || so.Port != 8080 {
		t.Fatalf("defaults not applied: %+v", so)
	}
}
