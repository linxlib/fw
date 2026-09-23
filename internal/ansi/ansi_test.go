package ansi

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestColorizeEnabled(t *testing.T) {
	got := colorize(true, FgGreen, "GET")
	want := "\x1b[32mGET\x1b[0m"
	if got != want {
		t.Errorf("colorize(enabled) = %q, want %q", got, want)
	}
}

func TestColorizeDisabledReturnsPlainText(t *testing.T) {
	// 颜色禁用时必须原样返回, 否则转义序列会泄漏到文件/管道/CI 日志里.
	for _, sgr := range []string{FgRed, FgGreen, FgYellow, FgLightBlue, FgLightCyan, FgLightYellow} {
		if got := colorize(false, sgr, "GET"); got != "GET" {
			t.Errorf("colorize(disabled, %s) = %q, want %q", sgr, got, "GET")
		}
	}
}

func TestColorizeKeepsResetAtEnd(t *testing.T) {
	got := colorize(true, FgRed, "a")
	if !strings.HasSuffix(got, reset) {
		t.Errorf("colorize result %q must end with reset %q, otherwise color bleeds into later output", got, reset)
	}
	if !strings.HasPrefix(got, "\x1b[") {
		t.Errorf("colorize result %q must start with CSI", got)
	}
}

func TestIsTerminal(t *testing.T) {
	// bytes.Buffer 不是 *os.File, 必须判为非终端.
	if IsTerminal(&bytes.Buffer{}) {
		t.Errorf("IsTerminal(bytes.Buffer) = true, want false")
	}
	// go test 的 stdout 通常是管道, 因此这里预期 false; 若在真实终端里跑则为 true,
	// 所以只断言函数对 nil 也不 panic.
	_ = IsTerminal(nil)
}

func TestSpinnerNonTerminalStaticOutput(t *testing.T) {
	var buf bytes.Buffer
	sp := NewSpinner(&buf, "step one")
	if sp.animated {
		t.Fatalf("animated = true for non-terminal writer, want false")
	}

	sp.Start()
	sp.UpdateText("step one (working)")
	sp.Stop(true, "step one done")

	got := buf.String()
	// 非终端: 只有开始与结束两行, 且不含任何控制序列.
	if strings.ContainsAny(got, "\x1b\r") {
		t.Errorf("non-terminal output must not contain control sequences, got %q", got)
	}
	if !strings.Contains(got, "step one ...\n") {
		t.Errorf("missing start line, got %q", got)
	}
	if !strings.Contains(got, "✓ step one done\n") {
		t.Errorf("missing success line, got %q", got)
	}
}

func TestSpinnerNonTerminalFailure(t *testing.T) {
	var buf bytes.Buffer
	sp := NewSpinner(&buf, "compile")
	sp.Start()
	sp.Stop(false, "compile failed")

	if got := buf.String(); !strings.Contains(got, "✗ compile failed\n") {
		t.Errorf("missing failure line, got %q", got)
	}
}

func TestSpinnerAnimated(t *testing.T) {
	var buf bytes.Buffer
	sp := NewSpinner(&buf, "working")
	// 强制走动画分支, 避免测试依赖真实终端; 同时把间隔调短以便快速结束.
	sp.animated = true
	sp.interval = time.Millisecond

	sp.Start()
	sp.UpdateText("working (halfway)")
	time.Sleep(10 * time.Millisecond) // 让转圈至少重绘几帧
	sp.Stop(true, "working done")

	got := buf.String()
	if !strings.Contains(got, "working") {
		t.Errorf("output missing spinner text, got %q", got)
	}
	if !strings.Contains(got, "✓ working done\n") {
		t.Errorf("missing final line, got %q", got)
	}
	// 终态必须清行, 否则转圈字符会留在输出里.
	if !strings.Contains(got, "\r\x1b[2K") {
		t.Errorf("animated output should clear the line before the final mark, got %q", got)
	}
}

func TestSpinnerStartStopIdempotent(t *testing.T) {
	var buf bytes.Buffer
	sp := NewSpinner(&buf, "once")
	sp.animated = true
	sp.interval = time.Millisecond

	sp.Start()
	sp.Start() // 重复 Start 不得再起一个 goroutine
	sp.Stop(true, "done")
	sp.Stop(true, "done again") // 重复 Stop 不得再输出一行
	sp.UpdateText("too late")   // Stop 之后更新应被忽略

	got := buf.String()
	if n := strings.Count(got, "✓"); n != 1 {
		t.Errorf("final mark appears %d times, want 1; output %q", n, got)
	}
}
