package config

import (
	"bytes"
	"fmt"
	"sort"
	"time"
)

// startReload 启动自动重载轮询(仅第一次调用生效).
func (c *Config) startReload() {
	if !c.AutoReload {
		return
	}
	c.reloadOnce.Do(func() {
		// 平台相关: Linux 用 inotify 监听配置目录, 变化时经 watchCh 即时触发重载;
		// 非 Linux(含 Windows)为空操作, 完全依赖下方轮询兜底.
		startFileWatcher(c)
		go func() {
			timer := time.NewTimer(c.AutoReloadInterval)
			for {
				select {
				case <-c.done:
					timer.Stop()
					return
				case <-c.watchCh:
					// 文件监听信号: 立即增量重载一次; 轮询定时器保持原节奏作为兜底
					// (此处不重置, 避免与"已触发未取"的 timer.C 值叠加产生多余 tick)
					_, _, _ = c.reloadTick()
				case <-timer.C:
					_, _, _ = c.reloadTick()
					timer.Reset(c.AutoReloadInterval)
				}
			}
		}()
	})
}

// Reload 立即执行一次增量重载, 返回本次发生变化的顶层 section 列表(可能为空).
// 与轮询共用同一套逻辑, 供外部按需触发与测试使用.
func (c *Config) Reload() ([]string, error) {
	changed, _, err := c.reloadTick()
	return changed, err
}

// reloadTick 执行一个重载周期:
//  1. 文件 modtime 未变且文件列表未变 -> 直接跳过(不做任何解码);
//  2. 解析并计算各顶层 section 的 sha256, 与上次快照对比;
//  3. 仅对 有变化/新增/删除 的 section 重新解码并写回其已注册目标
//     (全量目标 "" 在任何 section 变化时都会被刷新, 回调携带全部变化的 key);
//  4. 单个 target 重载失败时回滚该 target 旧值, 其余 target 不受影响;
//  5. 成功后提交快照, 并在锁外针对每个 (变化key, 受影响target) 调用回调.
//
// 返回 (变化的 section 列表, 是否发生变化, 首个错误).
func (c *Config) reloadTick() ([]string, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	discovered, modTimes, err := c.discoverFiles()
	if err != nil {
		return nil, false, err
	}
	if !c.fileChanged(modTimes) && len(discovered) > 0 {
		return nil, false, nil // 文件未变, 零开销跳过
	}

	var contents [][]byte
	for _, f := range discovered {
		b, rerr := c.readFile(f)
		if rerr != nil {
			return nil, false, rerr
		}
		contents = append(contents, b)
	}
	snap := sectionSnapshot(contents...)

	// 计算发生变化的顶层 section: 内容变化的 + 新增的 + 被删除的
	changed := make(map[string]bool)
	for k, h := range snap {
		prev, ok := c.sectionHashes[k]
		if !ok || !bytes.Equal(prev, h) {
			changed[k] = true
		}
	}
	for k := range c.sectionHashes {
		if _, ok := snap[k]; !ok {
			changed[k] = true // section 被整体删除, 相关 target 需要回落到默认值
		}
	}
	if len(changed) == 0 {
		// 文件内容等价(仅格式/顺序变化), 刷新快照后跳过
		c.updateSnapshotUnlocked(discovered, modTimes, contents)
		return nil, false, nil
	}

	// 收集受影响的已注册目标: section 匹配, 或全量目标("")
	var affected []*targetEntry
	for _, e := range c.targets {
		if e.section == "" || changed[e.section] {
			affected = append(affected, e)
		}
	}

	// 逐个重载(快照-提交, 失败回滚旧值), 收集回调
	type cb struct {
		key    string
		target any
	}
	var callbacks []cb
	var firstErr error
	for _, e := range affected {
		if err := c.loadCoreUnlocked(e, false); err != nil {
			if !c.Silent {
				fmt.Printf("config: reload target for key %q failed: %v\n", e.key, err)
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if e.section != "" {
			callbacks = append(callbacks, cb{key: e.section, target: e.target})
		} else {
			// 全量目标: 回调携带所有发生变化的 section
			for k := range changed {
				callbacks = append(callbacks, cb{key: k, target: e.target})
			}
		}
	}

	// 提交快照与观测计数
	c.updateSnapshotUnlocked(discovered, modTimes, contents)
	c.lastReloadedKeys = len(affected)

	changedKeys := make([]string, 0, len(changed))
	for k := range changed {
		changedKeys = append(changedKeys, k)
	}
	sort.Strings(changedKeys)

	// 锁外触发回调, 避免用户代码持锁执行
	c.mu.Unlock()
	for _, c2 := range callbacks {
		if c.AutoReloadCallback != nil {
			c.AutoReloadCallback(c2.key, c2.target)
		}
	}
	c.mu.Lock()
	return changedKeys, true, firstErr
}
