package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/linxlib/fw/v2/internal/ansi"
)

type Logger struct {
	mu         sync.Mutex
	level      Level
	console    bool
	consoleOut io.Writer
	fileOut    io.Writer
	basePath   string
}

type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

func ParseLevel(s string) Level {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "debug":
		return DebugLevel
	case "warn":
		return WarnLevel
	case "error":
		return ErrorLevel
	default:
		return InfoLevel
	}
}

func New(level string, output string, filePath string, enableFile bool) (*Logger, error) {
	l := &Logger{level: ParseLevel(level), console: true, consoleOut: os.Stdout}
	output = strings.ToLower(strings.TrimSpace(output))
	if output == "file" {
		l.console = false
	}
	if enableFile || output == "both" || output == "file" {
		if filePath == "" {
			filePath = "logs/fw.log"
		}
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return nil, fmt.Errorf("create log dir: %w", err)
		}
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("open log file: %w", err)
		}
		l.fileOut = f
		l.basePath = filePath
	}
	return l, nil
}

func (l *Logger) Debugf(format string, args ...any) { l.log(DebugLevel, "DEBUG", format, args...) }
func (l *Logger) Infof(format string, args ...any)  { l.log(InfoLevel, "INFO", format, args...) }
func (l *Logger) Warnf(format string, args ...any)  { l.log(WarnLevel, "WARN", format, args...) }
func (l *Logger) Errorf(format string, args ...any) { l.log(ErrorLevel, "ERROR", format, args...) }

func (l *Logger) log(level Level, tag string, format string, args ...any) {
	if level < l.level {
		return
	}
	msg := fmt.Sprintf(format, args...)
	timeText := time.Now().Format("2006-01-02 15:04:05")
	plain := fmt.Sprintf("[%s] [%s] %s\n", timeText, tag, msg)

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.console {
		styled := plain
		switch level {
		case DebugLevel:
			styled = ansi.Colorize(ansi.FgLightBlue, plain)
		case InfoLevel:
			styled = ansi.Colorize(ansi.FgGreen, plain)
		case WarnLevel:
			styled = ansi.Colorize(ansi.FgYellow, plain)
		case ErrorLevel:
			styled = ansi.Colorize(ansi.FgRed, plain)
		}
		_, _ = io.WriteString(l.consoleOut, styled)
	}
	if l.fileOut != nil {
		_, _ = io.WriteString(l.fileOut, plain)
	}
}

func (l *Logger) SetConsoleWriter(w io.Writer) {
	if w == nil {
		w = io.Discard
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.consoleOut = w
}
