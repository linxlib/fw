package config

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path"
	"reflect"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// loadCore 执行一次完整加载(持有写锁). 见 loadCoreUnlocked.
func (c *Config) loadCore(e *targetEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loadCoreUnlocked(e, true)
}

// loadCoreUnlocked 执行一次完整加载(调用方须持有写锁):
// 文件发现 -> 解码(缺 section 时保持现状) -> default 填充 -> env 覆盖 ->
// required 校验 -> 写回 target; commitSnapshot 为 true 时刷新哈希快照.
// 解码/校验失败时回滚 target 旧值并返回错误, 不刷新快照.
func (c *Config) loadCoreUnlocked(e *targetEntry, commitSnapshot bool) error {
	var contents [][]byte
	discovered, modTimes, err := c.discoverFiles()
	if err != nil {
		return err
	}
	for _, f := range discovered {
		b, rerr := c.readFile(f)
		if rerr != nil {
			if c.Debug || c.Verbose {
				fmt.Printf("config: failed to read %s: %v\n", f, rerr)
			}
			continue
		}
		contents = append(contents, b)
	}

	// 快照当前 target, 失败时回滚, 保证重载期间内存配置不被半更新污染.
	targetRV := reflect.ValueOf(e.target).Elem()
	snapshot := reflect.New(targetRV.Type()).Elem()
	if targetRV.Kind() == reflect.Struct {
		for i := 0; i < targetRV.NumField(); i++ {
			snapshot.Field(i).Set(targetRV.Field(i))
		}
	} else {
		snapshot.Set(targetRV)
	}

	dst := reflect.New(targetRV.Type()).Elem()
	dst.Set(snapshot)

	// default 填充
	if err := c.processDefaults(dst.Addr().Interface()); err != nil {
		return err
	}
	// YAML 解码: 文件按序叠加, 缺失的 section 不触碰目标
	for _, content := range contents {
		if err := decodeYaml(content, e.key, dst.Addr().Interface()); err != nil {
			c.restore(e, snapshot)
			return err
		}
	}
	// env 覆盖(最高优先级)
	if err := c.processEnv(dst.Addr().Interface(), e.key); err != nil {
		c.restore(e, snapshot)
		return err
	}
	// required 校验
	if err := c.processRequired(dst.Addr().Interface()); err != nil {
		c.restore(e, snapshot)
		return err
	}

	// 提交
	targetRV.Set(dst)

	if commitSnapshot {
		c.updateSnapshotUnlocked(discovered, modTimes, contents)
	}
	return nil
}

// restore 把 target 恢复为快照值(重载失败时保留旧值).
func (c *Config) restore(e *targetEntry, snapshot reflect.Value) {
	targetRV := reflect.ValueOf(e.target).Elem()
	e.copier(targetRV, snapshot)
	if !c.Silent {
		fmt.Printf("config: reload failed, keeping previous values\n")
	}
}

// discoverFiles 返回实际参与加载的文件列表(基础文件 + 环境文件)与修改时间.
// 缺失的文件直接跳过, 一个都不存在也返回 nil(仅使用默认值与环境变量).
func (c *Config) discoverFiles() ([]string, map[string]time.Time, error) {
	stat := os.Stat
	if c.FS != nil {
		stat = func(name string) (os.FileInfo, error) {
			return fs.Stat(c.FS, name)
		}
	}
	var (
		result   []string
		modTimes = make(map[string]time.Time)
	)
	for _, f := range c.files {
		if !strings.HasSuffix(f, ".yaml") {
			return nil, nil, fmt.Errorf("config: only .yaml files are supported, got %q", f)
		}
		if info, err := stat(f); err == nil && info.Mode().IsRegular() {
			result = append(result, f)
			modTimes[f] = info.ModTime()
		} else if c.Debug {
			fmt.Printf("config: file %s not found, skipped\n", f)
		}
		// 环境文件叠加: config/config.yaml -> config/config.<env>.yaml (覆盖基础文件)
		if envFile := c.envFile(f); envFile != "" {
			if info, err := stat(envFile); err == nil && info.Mode().IsRegular() {
				result = append(result, envFile)
				modTimes[envFile] = info.ModTime()
			}
		}
	}
	return result, modTimes, nil
}

// envFile 返回文件对应环境文件的候选路径(存在时), 例如
// "config/config.yaml" -> "config/config.development.yaml".
func (c *Config) envFile(file string) string {
	ext := path.Ext(file)
	candidate := strings.TrimSuffix(file, ext) + "." + c.GetEnvironment() + ext
	if c.FS != nil {
		if _, err := fs.Stat(c.FS, candidate); err != nil {
			return ""
		}
		return candidate
	}
	info, err := os.Stat(candidate)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return candidate
}

// readFile 读取文件内容.
func (c *Config) readFile(file string) ([]byte, error) {
	if c.FS != nil {
		return fs.ReadFile(c.FS, file)
	}
	return os.ReadFile(file)
}

// decodeYaml 将 YAML 数据中 key 指定的部分解码到 target.
//   - key 为空: 整个映射解码;
//   - key 为顶层 section 或点号路径: 逐级下钻, 命中则解码叶子; 未命中不触碰 target.
//
// YAML 顶层必须是映射; 内容不是映射(如裸标量)时返回错误.
func decodeYaml(data []byte, key string, target any) error {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil // 空文件视为无配置
	}
	root, err := parseMapping(data)
	if err != nil {
		return err
	}
	if root == nil {
		return nil
	}
	if key == "" {
		return root.Decode(target)
	}
	node := root
	for _, seg := range strings.Split(key, ".") {
		found := false
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == seg {
				node = node.Content[i+1]
				found = true
				break
			}
		}
		if !found {
			return nil // 路径不存在, 保持 target 现状
		}
	}
	return node.Decode(target)
}

// parseMapping 解析 YAML 顶层映射. 空文档返回 nil.
func parseMapping(data []byte) (*yaml.Node, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("config: parse yaml: %w", err)
	}
	if len(root.Content) == 0 {
		return nil, nil
	}
	top := &root
	for top.Kind == yaml.DocumentNode {
		if len(top.Content) == 0 {
			return nil, nil
		}
		top = top.Content[0]
	}
	if top.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config: top-level yaml value must be a mapping, got %v", top.Kind)
	}
	return top, nil
}

// sectionSnapshot 计算每个顶层 section 规范化内容的 sha256, 用于增量重载判定.
// 多文件叠加时后写的文件覆盖先写的同名 section.
func sectionSnapshot(contents ...[]byte) map[string][]byte {
	snap := make(map[string][]byte)
	for _, content := range contents {
		root, err := parseMapping(content)
		if err != nil || root == nil {
			continue
		}
		for i := 0; i+1 < len(root.Content); i += 2 {
			k := root.Content[i].Value
			// 子树解码为泛型值后重新序列化, 得到内容稳定的规范化表示
			var generic any
			if err := root.Content[i+1].Decode(&generic); err != nil {
				continue
			}
			data, err := yaml.Marshal(generic)
			if err != nil {
				continue
			}
			sum := sha256.Sum256(data)
			snap[k] = sum[:]
		}
	}
	return snap
}

// updateSnapshot 记录文件列表、修改时间与 section 哈希(加锁).
func (c *Config) updateSnapshot(files []string, modTimes map[string]time.Time, contents [][]byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updateSnapshotUnlocked(files, modTimes, contents)
}

// updateSnapshotUnlocked 与 updateSnapshot 相同, 调用方须持有锁.
func (c *Config) updateSnapshotUnlocked(files []string, modTimes map[string]time.Time, contents [][]byte) {
	c.fileList = append([]string(nil), files...)
	c.modTimes = modTimes
	c.sectionHashes = sectionSnapshot(contents...)
}

// fileChanged 快速判定文件是否自上次加载后变化(modtime + 文件列表对比).
func (c *Config) fileChanged(modTimes map[string]time.Time) bool {
	if len(modTimes) != len(c.modTimes) {
		return true
	}
	for f, t := range modTimes {
		prev, ok := c.modTimes[f]
		if !ok || t.After(prev) {
			return true
		}
	}
	return false
}
