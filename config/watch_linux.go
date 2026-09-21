//go:build linux

package config

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// startFileWatcher 使用 inotify 监听配置目录, 在配置内容可能变化时向
// c.watchCh 发送一个合并后的信号, 由轮询循环即时触发增量重载.
//
// inotify 仅 Linux 支持; 本文件只在该平台编译, 其余平台见 watch_other.go
// (空操作, 完全依赖轮询兜底). 若 inotify 初始化失败或没有可监听目录,
// 静默回退到纯轮询, 不影响功能正确性.
func startFileWatcher(c *Config) {
	dirs, names := c.watchTargets()
	if len(dirs) == 0 {
		return
	}
	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC)
	if err != nil {
		return
	}
	// 覆盖写/创建/移动等常见保存方式(含编辑器"临时文件+rename"的保存模式)
	const mask = uint32(unix.IN_CREATE | unix.IN_MODIFY |
		unix.IN_CLOSE_WRITE | unix.IN_MOVED_FROM | unix.IN_MOVED_TO | unix.IN_DELETE)
	added := 0
	for _, d := range dirs {
		if info, serr := os.Stat(d); serr != nil || !info.IsDir() {
			continue // 目录尚不存在, 交由轮询兜底
		}
		if _, werr := unix.InotifyAddWatch(fd, d, mask); werr == nil {
			added++
		}
	}
	if added == 0 {
		_ = unix.Close(fd)
		return
	}

	buf := make([]byte, 8*1024)
	go func() {
		for {
			n, rerr := unix.Read(fd, buf)
			if rerr != nil {
				// fd 被 Close 关闭或其他错误: 确认退出, 否则短暂退避后继续
				select {
				case <-c.done:
					return
				case <-time.After(200 * time.Millisecond):
					continue
				}
			}
			if parseInotifyEvents(buf[:n], names) {
				// 合并突发: 已有待处理信号时直接丢弃, reloadTick 幂等(未变则零开销)
				select {
				case c.watchCh <- struct{}{}:
				default:
				}
			}
		}
	}()
	go func() {
		<-c.done
		_ = unix.Close(fd) // 解除阻塞的 Read, 让监听 goroutine 退出
	}()
}

// watchTargets 返回需要监听的目录(去重)与应触发重载的文件基名集合
// (基础文件 + 环境叠加文件; 环境文件即使尚不存在也纳入, 便于其创建后即时生效).
func (c *Config) watchTargets() ([]string, map[string]bool) {
	dirs := make([]string, 0, len(c.files))
	names := make(map[string]bool)
	seenDir := make(map[string]bool)
	env := c.GetEnvironment()
	for _, f := range c.files {
		d := filepath.Dir(f)
		if !seenDir[d] {
			seenDir[d] = true
			dirs = append(dirs, d)
		}
		base := filepath.Base(f)
		ext := filepath.Ext(f)
		names[base] = true
		names[strings.TrimSuffix(base, ext)+"."+env+ext] = true
	}
	return dirs, names
}

// parseInotifyEvents 解析 inotify 事件缓冲区, 若任一事件命中已知文件基名则返回 true.
// inotify 事件为连续排列: 16 字节头(Wd/Mask/Cookie/Len, 各 4 字节)后跟 NUL 结尾的
// 文件名(长度 = Len), 下一个事件紧随其后.
func parseInotifyEvents(buf []byte, names map[string]bool) bool {
	const header = unix.SizeofInotifyEvent // 16
	off := 0
	for off+header <= len(buf) {
		length := int(binary.NativeEndian.Uint32(buf[off+12 : off+16])) // Len 字段(内核字段为原生字节序)
		if off+header+length > len(buf) {
			break
		}
		name := strings.TrimRight(string(buf[off+header:off+header+length]), "\x00")
		if names[name] {
			return true
		}
		off += header + length
	}
	return false
}
