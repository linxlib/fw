package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pterm/pterm"
)

type Logger struct {
	mu       sync.Mutex
	level    Level
	console  bool
	fileOut  io.Writer
	basePath string
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
	l := &Logger{level: ParseLevel(level), console: true}
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
		switch level {
		case DebugLevel:
			pterm.NewStyle(pterm.FgLightBlue).Print(plain)
		case InfoLevel:
			pterm.NewStyle(pterm.FgGreen).Print(plain)
		case WarnLevel:
			pterm.NewStyle(pterm.FgYellow).Print(plain)
		case ErrorLevel:
			pterm.NewStyle(pterm.FgRed).Print(plain)
		}
	}
	if l.fileOut != nil {
		_, _ = io.WriteString(l.fileOut, plain)
	}
}
