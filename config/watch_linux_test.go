//go:build linux

package config

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestInotifyTriggersReload 验证 Linux 下 inotify 监听能即时触发重载.
// 关键: AutoReloadInterval 设为 10s(远大于测试时限), 若 so.Port 在数秒内更新,
// 必然是 inotify 事件触发, 而非轮询. 非 Linux 平台本文件不编译, 走轮询兜底.
func TestInotifyTriggersReload(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(file, []byte(twoSections), 0o644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	c := New(&Option{
		Files:              []string{file},
		Silent:             true,
		AutoReload:         true,
		AutoReloadInterval: 10 * time.Second, // 轮询周期远大于测试时限, 用于排除轮询干扰
	})
	defer c.Close()

	var so serverOpt
	if err := c.LoadWithKey("server", &so); err != nil {
		t.Fatalf("load: %v", err)
	}
	if so.Port != 9000 {
		t.Fatalf("initial load failed: %+v", so)
	}

	content := strings.Replace(twoSections, "port: 9000", "port: 9500", 1)
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if so.Port == 9500 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if so.Port != 9500 {
		t.Fatalf("inotify reload did not take effect within deadline: %+v", so)
	}
}

// TestParseInotifyEvents 覆盖事件缓冲区解析(纯函数, 无系统调用, 任意平台可跑但
// 依赖 Linux 的 unix.SizeofInotifyEvent, 故仅 Linux 编译).
func TestParseInotifyEvents(t *testing.T) {
	// 手工构造一个事件: Wd=1, Mask=IN_CLOSE_WRITE, Cookie=0, Len=5, Name="a.b\0".
	// Len 字段按原生字节序写入, 与 parseInotifyEvents 的读取方式保持一致.
	buf := make([]byte, 16+5)
	binary.NativeEndian.PutUint32(buf[0:4], 1)
	binary.NativeEndian.PutUint32(buf[4:8], 0x8) // IN_CLOSE_WRITE
	binary.NativeEndian.PutUint32(buf[12:16], 5) // Len
	copy(buf[16:], "a.b\x00")
	if !parseInotifyEvents(buf, map[string]bool{"a.b": true}) {
		t.Fatalf("expected match for a.b")
	}
	if parseInotifyEvents(buf, map[string]bool{"x.yaml": true}) {
		t.Fatalf("unexpected match")
	}
}
