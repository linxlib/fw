// Package ansi 提供最小化的终端输出 helpers: ANSI 颜色码与转圈指示器.
//
// 存在的原因: fw 只需要「给文字上色」和「构建时转圈」这两点能力, 不值得为此引入
// pterm 及其约 11 个传递依赖(atomicgo/*, containerd/console, xo/terminfo,
// gookit/color, lithammer/fuzzysearch, mattn/go-runewidth, clipperhouse/uax29,
// golang.org/x/term ...). app 与 logger 是框架的公开包, 这些依赖会因此进入每一个
// import fw/app 的使用方 go.mod.
//
// 颜色行为遵循常见约定, 与原先 pterm 底层(gookit/color)的探测意图一致:
//   - NO_COLOR 存在且非空时禁用(https://no-color.org/);
//   - 标准输出不是终端(管道/重定向/CI)时禁用.
//
// 因此把输出重定向到文件或管道时不会混入转义序列, 调用方无需自行判断.
package ansi

import (
	"io"
	"os"
	"sync"
)

// reset 用于结束 SGR 属性, 防止颜色渗透到后续输出.
const reset = "\x1b[0m"

// 常用 SGR 前景色码(不含 CSI 前缀与结尾的 'm').
const (
	FgRed         = "31"
	FgGreen       = "32"
	FgYellow      = "33"
	FgLightBlue   = "94"
	FgLightCyan   = "96"
	FgLightYellow = "93"
)

var (
	colorOnce sync.Once
	colorOK   bool
)

// IsTerminal 报告 w 是否是字符设备(终端). 非 *os.File 一律视为非终端.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// Enabled 报告当前是否应该输出颜色. 结果按进程缓存一次: 终端形态与 NO_COLOR
// 在进程生命周期内不会改变, 重复探测只是浪费.
func Enabled() bool {
	colorOnce.Do(func() {
		if v, ok := os.LookupEnv("NO_COLOR"); ok && v != "" {
			return // NO_COLOR 规范: 非空即禁用
		}
		colorOK = IsTerminal(os.Stdout)
	})
	return colorOK
}

// Colorize 用 sgr 包裹 s. 颜色被禁用时原样返回 s, 所以调用方无需分支处理.
//
//	ansi.Colorize(ansi.FgGreen, method)
func Colorize(sgr, s string) string {
	return colorize(Enabled(), sgr, s)
}

// colorize 是 Colorize 的可测内核: enabled 由调用方给定, 不触碰全局状态.
func colorize(enabled bool, sgr, s string) string {
	if !enabled {
		return s
	}
	return "\x1b[" + sgr + "m" + s + reset
}
