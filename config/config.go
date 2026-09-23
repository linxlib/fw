// Package config 是 fw 专用的配置库.
//
// 数据源仅两种: YAML 配置文件(默认读取 config 目录下的 config.yaml, 只接受 .yaml 后缀)
// 与 环境变量. 支持三种加载形态:
//
//  1. 显式 key: c.LoadWithKey("server", &opt) / c.LoadWithKey("server.port", &port),
//     key 可以是顶层 section, 也可以是点号路径, target 可以是任意可寻址的值
//     (结构体指针, 父结构体的某个字段, 甚至普通变量);
//  2. tag 驱动: c.LoadByTags(&opt) 按 struct 字段上的 inject:"<section>" tag 注入;
//  3. 全量: c.Load(&opt) 等价 LoadWithKey("", &opt).
//
// 开启 AutoReload 后, 每个轮询周期只对内容发生变化的顶层 section 重新解码并
// 写回其关联的内存配置, 其余 section 的内存对象零扰动, 回调也只收到变化的 key.
// 检测到需要重载时会在标准输出打印 "config: reload detected, changed sections: [...]",
// 列出本次发生变化的顶层 section(Silent 模式下不输出).
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"sync"
	"time"
)

// Option 配置库选项.
type Option struct {
	// Files 配置文件列表, 按列表顺序叠加加载(后读的文件覆盖先读的).
	// 缺省为 ["config/config.yaml"]. 仅接受 .yaml 后缀, 显式传入 .yml/.json 会报错.
	Files []string
	// Environment 运行环境, 为空时依次取 CONFIG_ENV, go test 场景为 test, 否则 development.
	// 若存在 config/config.<env>.yaml 会叠加在其后的 config/config.yaml 之上.
	Environment string
	// ENVPrefix 环境变量前缀, 缺省 "FW". 字段无 env tag 时按 FW_<KEYPATH>_<FIELD> 派生候选变量名.
	ENVPrefix string
	Debug     bool
	Verbose   bool
	Silent    bool
	// AutoReload 开启按 AutoReloadInterval 的轮询增量重载.
	AutoReload bool
	// AutoReloadInterval 轮询间隔, 缺省 1s.
	AutoReloadInterval time.Duration
	// AutoReloadCallback 在某个 key 对应的内存配置被更新后, 针对每个受影响的
	// (key, target) 调用一次; key 只会是本次实际发生变化的顶层 section.
	AutoReloadCallback func(key string, config any)
	// FS 可选的文件系统, 便于 embed.FS / 测试场景, 缺省使用 os.
	FS fs.FS
}

// targetEntry 是注册表中的一个已注册目标.
type targetEntry struct {
	key     string // 注册时的 key: "" 全量, "server" 顶层 section, "server.port" 点号路径
	section string // 归类用的顶层 section; "" 表示全量目标(任何 section 变化都会重载)
	target  any    // 可寻址值
	copier  func(dst, src reflect.Value)
}

// Config 持有配置来源与所有已注册目标, 线程安全.
type Config struct {
	*Option
	files []string // Option.Files 的缺省填充结果

	mu            sync.RWMutex
	targets       []*targetEntry
	sectionHashes map[string][]byte // 顶层 key -> 规范化子树的 sha256
	modTimes      map[string]time.Time
	fileList      []string // 上次发现的文件列表(环境文件叠加后)

	reloadOnce sync.Once
	closeOnce  sync.Once
	done       chan struct{}
	watchCh    chan struct{} // 文件监听信号(缓冲 1, 合并突发); 非 Linux 平台无人写入

	// 供测试与外部观测: 最近一次 reloadTick 中实际被重写的全局 key 数量.
	lastReloadedKeys int
}

// New 初始化一个 Config.
func New(opts *Option) *Config {
	if opts == nil {
		opts = &Option{}
	}
	if opts.ENVPrefix == "" {
		opts.ENVPrefix = "FW"
	}
	if len(opts.Files) == 0 {
		opts.Files = []string{"config/config.yaml"}
	}
	if opts.AutoReload && opts.AutoReloadInterval == 0 {
		opts.AutoReloadInterval = time.Second
	}
	return &Config{
		Option:        opts,
		files:         opts.Files,
		sectionHashes: make(map[string][]byte),
		modTimes:      make(map[string]time.Time),
		done:          make(chan struct{}),
		watchCh:       make(chan struct{}, 1),
	}
}

// Close 停止自动重载轮询. 可重复调用.
func (c *Config) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
	})
}

// GetEnvironment 返回当前环境.
func (c *Config) GetEnvironment() string {
	if c.Environment != "" {
		return c.Environment
	}
	if env := envLookup("CONFIG_ENV"); env != "" {
		return env
	}
	if isTestBinary() {
		return "test"
	}
	return "development"
}

var (
	envLookup    = lookupEnv
	isTestBinary = detectTestBinary
)

// LoadWithKey 将 YAML 中 key 指定的 section 解码到 target, 并登记 target 以便自动重载.
//
// key 的取值:
//   - "": 整个 YAML 映射解码到 target(全量);
//   - "server": 顶层 section;
//   - "server.port": 点号路径, 叶子值解码到 target, target 可以是任意可寻址类型.
//
// target 必须是指针, 且指向可寻址的值(结构体、结构体字段或普通变量均可).
// YAML 缺失对应 section 时保持 target 现有值(含 default tag 填充的值)不变.
func (c *Config) LoadWithKey(key string, target any) error {
	if target == nil {
		return errors.New("config: target must not be nil")
	}
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Ptr || !rv.CanInterface() {
		return fmt.Errorf("config: target must be a non-nil pointer, got %T", target)
	}
	if rv.IsNil() {
		return fmt.Errorf("config: target pointer must not be nil: %T", target)
	}
	e := &targetEntry{key: key, section: topLevelKey(key), target: target}
	e.copier = makeCopier(rv.Elem().Type())

	c.mu.Lock()
	c.targets = append(c.targets, e)
	c.mu.Unlock()
	// 开启自动重载时, 首个 LoadWithKey 触发轮询启动(仅一次)
	c.startReload()
	return c.loadCore(e)
}

// Load 等价 LoadWithKey("", target).
func (c *Config) Load(target any) error {
	return c.LoadWithKey("", target)
}

// LoadByTags 按 target(struct 指针) 字段上的 inject tag 注入配置:
//
//   - inject:"server"      : 从顶层 section server 注入该字段;
//   - inject:"server.port" : 点号路径注入;
//   - inject:""            : 用字段名作为 section key;
//   - inject:"-" / "_"     : 跳过该字段.
//
// 每个被注入的字段都会按 (key, 字段地址) 登记, 自动重载时随对应 section 更新;
// 字段为指针类型且当前为 nil 时会先分配实例.
func (c *Config) LoadByTags(target any) error {
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Ptr {
		return fmt.Errorf("config: LoadByTags target must be a pointer, got %T", target)
	}
	v := rv.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("config: LoadByTags target must point to a struct, got %s", v.Kind())
	}
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		sf := t.Field(i)
		if !f.CanAddr() {
			continue
		}
		tag, ok := sf.Tag.Lookup("inject")
		if !ok {
			continue
		}
		switch tag {
		case "-", "_":
			continue
		case "":
			tag = sf.Name
		}
		if f.Kind() == reflect.Ptr {
			if f.Type().Elem().Kind() == reflect.Struct && f.IsNil() {
				f.Set(reflect.New(f.Type().Elem()))
			}
		}
		var targetPtr any
		if f.Kind() == reflect.Ptr {
			targetPtr = f.Interface()
		} else {
			targetPtr = f.Addr().Interface()
		}
		if err := c.LoadWithKey(tag, targetPtr); err != nil {
			return err
		}
	}
	return nil
}

// topLevelKey 返回 key 归类的顶层 section: "" -> "", "server" -> "server", "server.port" -> "server".
func topLevelKey(key string) string {
	if key == "" {
		return ""
	}
	if i := strings.IndexByte(key, '.'); i >= 0 {
		return key[:i]
	}
	return key
}

// makeCopier 生成一个把 src 的值副本写入 dst 的函数, 用于重载失败时回滚.
// 结构体为逐字段浅拷贝, 其他类型直接 Set(切片/映射为引用复制, 对回滚场景足够).
func makeCopier(t reflect.Type) func(dst, src reflect.Value) {
	return func(dst, src reflect.Value) {
		if src.Kind() == reflect.Struct {
			dst.Set(reflect.New(t).Elem())
			for i := 0; i < src.NumField(); i++ {
				dst.Field(i).Set(src.Field(i))
			}
			return
		}
		dst.Set(src)
	}
}

// LastReloadedKeys 返回最近一次重载周期中被重写的全局 key 数量(未开启重载时恒为 0).
// 主要用于测试与观测增量重载的实际影响面.
func (c *Config) LastReloadedKeys() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastReloadedKeys
}

// Load 使用缺省文件 config/config.yaml 全量加载.
func Load(target any) error {
	return New(nil).Load(target)
}

// LoadWithKey 使用缺省文件 config/config.yaml 加载指定 key 的 section.
func LoadWithKey(key string, target any) error {
	return New(nil).LoadWithKey(key, target)
}

// ENV 返回当前运行环境(等价 New(nil).GetEnvironment()).
func ENV() string {
	return New(nil).GetEnvironment()
}
