package ansi

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// spinnerFrames 是转圈时循环显示的字符. 用 ASCII 而非盲文点阵, 避免在
// 代码页受限的 Windows 控制台上变成问号.
var spinnerFrames = []rune{'|', '/', '-', '\\'}

// defaultSpinnerInterval 是重绘间隔. 100ms 足够看起来在转, 又不至于刷屏.
const defaultSpinnerInterval = 100 * time.Millisecond

// Spinner 是一个最小的终端转圈指示器: 后台 goroutine 按 interval 重绘当前文本,
// 结束时输出一行终态(✓/✗).
//
// 非终端环境下自动退化为「只打印开始与结束两行」, 不启动 goroutine, 也不产生
// 任何控制序列, 因此重定向到文件或管道时输出依然干净.
//
// Start/UpdateText/Stop 均可重复调用且并发安全: Start 只启动一次, Stop 只终结一次.
type Spinner struct {
	w        io.Writer
	interval time.Duration
	animated bool

	mu      sync.Mutex
	text    string
	stop    chan struct{}
	done    chan struct{}
	started bool
	stopped bool
}

// NewSpinner 创建一个写入 w 的指示器, 初始文本为 text.
// w 不是终端时 animated 为 false, 行为退化为静态两行输出.
func NewSpinner(w io.Writer, text string) *Spinner {
	return &Spinner{
		w:        w,
		text:     text,
		interval: defaultSpinnerInterval,
		animated: IsTerminal(w),
	}
}

// Start 启动转圈并打印初始文本. 非终端时只打印一行静态文本.
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.started || s.stopped {
		s.mu.Unlock()
		return
	}
	s.started = true
	text := s.text
	s.mu.Unlock()

	if !s.animated {
		fmt.Fprintf(s.w, "%s ...\n", text)
		return
	}

	s.mu.Lock()
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	s.mu.Unlock()

	go s.spin()
}

// spin 是转圈循环, 由 Start 启动, 收到 stop 信号后退出.
func (s *Spinner) spin() {
	defer close(s.done)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for i := 0; ; i++ {
		s.mu.Lock()
		text := s.text
		s.mu.Unlock()

		// \r 回到行首, \x1b[2K 清掉整行, 避免上次较长的文本残留
		fmt.Fprintf(s.w, "\r\x1b[2K%c %s", spinnerFrames[i%len(spinnerFrames)], text)

		select {
		case <-s.stop:
			return
		case <-ticker.C:
		}
	}
}

// UpdateText 更新转圈旁边显示的文本. Stop 之后调用无效果.
func (s *Spinner) UpdateText(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.text = text
}

// Stop 结束转圈并输出终态一行. success 为 true 用 ✓, 否则用 ✗.
func (s *Spinner) Stop(success bool, text string) {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	stop, done := s.stop, s.done
	s.mu.Unlock()

	// 等转圈 goroutine 退出后再写终态, 否则两条输出会交错
	if stop != nil {
		close(stop)
		<-done
	}

	mark := '✗'
	if success {
		mark = '✓'
	}
	if s.animated {
		fmt.Fprintf(s.w, "\r\x1b[2K%c %s\n", mark, text)
		return
	}
	fmt.Fprintf(s.w, "%c %s\n", mark, text)
}
