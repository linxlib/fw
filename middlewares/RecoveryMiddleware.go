package middlewares

import (
	"bytes"
	"fmt"
	"github.com/linxlib/fw"
	"io"
	"log"
	"os"
	"runtime"
	"time"
)

// RecoveryMiddleware globally recover from panic
type RecoveryMiddleware struct {
	fw.MiddlewareGlobal
	writer io.Writer
	// if true, panic info and request info will show as web page (false when production)
	// else output something to response
	niceWeb bool
	// output to console
	console bool
	// defines whether show with color
	color bool
}

func (s *RecoveryMiddleware) CloneAsMethod() fw.IMiddlewareMethod {
	return s.CloneAsCtl()
}

func (s *RecoveryMiddleware) HandlerMethod(h fw.HandlerFunc) fw.HandlerFunc {
	return func(context *fw.Context) {
		defer func() {
			if err := recover(); err != nil {
				//var brokenPipe bool
				//if ne, ok := err.(*net.OpError); ok {
				//	if se, ok := ne.Err.(*os.SyscallError); ok {
				//		if strings.Contains(strings.ToLower(se.Error()), "broken pipe") || strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
				//			brokenPipe = true
				//		}
				//	}
				//}
				stack := stack(3)
				if s.writer != nil {
					//DUMP http request、headers etc.
					s.writer.Write([]byte("Panic:\n"))
					s.writer.Write(stack)

				} else {
					log.Println(string(stack))
				}

			}
		}()
		h(context)
	}
}

func (s *RecoveryMiddleware) CloneAsCtl() fw.IMiddlewareCtl {
	return NewRecoveryMiddleware()
}

func (s *RecoveryMiddleware) HandlerController(base string) *fw.RouteItem {

	return &fw.RouteItem{
		Method:     "",
		Path:       "",
		IsHide:     false,
		H:          nil,
		Middleware: s,
	}
}

const recoveryName = "Recovery"

func NewRecoveryMiddleware() fw.IMiddlewareGlobal {

	return &RecoveryMiddleware{
		MiddlewareGlobal: fw.NewMiddlewareGlobal(recoveryName),
		writer:           os.Stdout,
	}
}

var (
	dunno     = []byte("???")
	centerDot = []byte("·")
	dot       = []byte(".")
	slash     = []byte("/")
)

// stack returns a nicely formatted stack frame, skipping skip frames.
func stack(skip int) []byte {
	buf := new(bytes.Buffer) // the returned data
	// As we loop, we open files and read them. These variables record the currently
	// loaded file.
	var lines [][]byte
	var lastFile string
	for i := skip; ; i++ { // Skip the expected number of frames
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		// Print this much at least.  If we can't find the source, it won't show.
		fmt.Fprintf(buf, "%s:%d (0x%x)\n", file, line, pc)
		if file != lastFile {
			data, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			lines = bytes.Split(data, []byte{'\n'})
			lastFile = file
		}
		fmt.Fprintf(buf, "\t%s: %s\n", function(pc), source(lines, line))
	}
	return buf.Bytes()
}

// source returns a space-trimmed slice of the n'th line.
func source(lines [][]byte, n int) []byte {
	n-- // in stack trace, lines are 1-indexed but our array is 0-indexed
	if n < 0 || n >= len(lines) {
		return dunno
	}
	return bytes.TrimSpace(lines[n])
}

// function returns, if possible, the name of the function containing the PC.
func function(pc uintptr) []byte {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return dunno
	}
	name := []byte(fn.Name())
	// The name includes the path name to the package, which is unnecessary
	// since the file name is already included.  Plus, it has center dots.
	// That is, we see
	//	runtime/debug.*T·ptrmethod
	// and want
	//	*T.ptrmethod
	// Also the package path might contains dot (e.g. code.google.com/...),
	// so first eliminate the path prefix
	if lastSlash := bytes.LastIndex(name, slash); lastSlash >= 0 {
		name = name[lastSlash+1:]
	}
	if period := bytes.Index(name, dot); period >= 0 {
		name = name[period+1:]
	}
	name = bytes.Replace(name, centerDot, dot, -1)
	return name
}
func timeFormat(t time.Time) string {
	timeString := t.Format("2006/01/02 - 15:04:05")
	return timeString
}
