//go:build !linux

package config

// startFileWatcher 在非 Linux 平台为空操作.
//
// inotify 仅 Linux 支持; Windows 没有等价的原生机制(可用 ReadDirectoryChangesW,
// 但此处为避免跨平台行为差异而统一走轮询), macOS 同理. 因此这些平台完全依赖
// startReload 中的 AutoReloadInterval 轮询兜底, 行为与引入本机制前一致.
func startFileWatcher(c *Config) {}
