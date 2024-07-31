package fw

import (
	"bytes"
	"fmt"
	"github.com/gookit/color"
	"github.com/sirupsen/logrus"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"
)

type Entry struct {
	*logrus.Entry
}

func (entry *Entry) Trace(args ...interface{}) {
	entry.printLog(logrus.TraceLevel, args...)
}
func (entry *Entry) Debug(args ...interface{}) {
	entry.printLog(logrus.DebugLevel, args...)
}
func (entry *Entry) Info(args ...interface{}) {
	entry.printLog(logrus.InfoLevel, args...)
}

func (entry *Entry) Warn(args ...interface{}) {
	entry.printLog(logrus.WarnLevel, args...)
}

func (entry *Entry) Error(args ...interface{}) {
	entry.printLog(logrus.ErrorLevel, args...)
}

func (entry *Entry) Fatal(args ...interface{}) {
	entry.printLog(logrus.FatalLevel, args...)
}

func (entry *Entry) Panic(args ...interface{}) {
	entry.printLog(logrus.PanicLevel, args...)
}

func (entry *Entry) printLog(level logrus.Level, args ...interface{}) {
	datas := make([]interface{}, len(args))
	for i, v := range args {
		datas[i] = Stringify(v)
	}
	entry.Log(level, fmt.Sprint(datas...))
}

type RootFields struct {
	Timestamp string
	Func      string
	Level     logrus.Level
	Fields    interface{}
}
type Formatter struct {
	/** if enabled, the whole log will be marshaled to json (including captions)
	  work as json Formatter. */
	TransportToJson bool
	/** if enabled, log prefixed with 'timestamp, level' */
	UseDefaultCaption bool
	/** if enabled, CustomCaption will be marshaled to json */
	CustomCaptionPrettyPrint bool
	/** if has value, it attached right before message(object). custom caption can be struct, string, whatever */
	CustomCaption interface{}
	/** do PrettyPrint for message(object) */
	PrettyPrint bool
	/** if enabled, the message(object) will be colorized by predefined color code, along with logLevel */
	Colorize bool
}

/** syntatic sugar for using formatter with predefined option (for console) */
func Console() *Formatter {
	return &Formatter{UseDefaultCaption: true, PrettyPrint: true, Colorize: true}
}

/** syntatic sugar for using formatter with predefined option (for server, json) */
func Json() *Formatter {
	return &Formatter{TransportToJson: true, UseDefaultCaption: true}
}

type (
	JO map[string]interface{}
)

func (f *Formatter) Format(entry *logrus.Entry) ([]byte, error) {
	b := &bytes.Buffer{}

	root := RootFields{Fields: encode(entry.Message)}

	if f.UseDefaultCaption {
		if f.TransportToJson {
			root = RootFields{
				Fields: JO{
					"time_stamp": entry.Time.Format(time.DateTime),
					"level":      entry.Level,
					"message":    encode(entry.Message),
				},
			}
		} else {
			root = RootFields{Timestamp: entry.Time.Format(time.DateTime), Level: entry.Level,
				Fields: encode(entry.Message)}

			if f.Colorize {
				color.Fprintf(b, "%s ", white(root.Timestamp))
			} else {
				b.WriteString(root.Timestamp)
				b.WriteString(" ")
			}
		}
	}

	//if entry.HasCaller() {
	//	caller := getCaller(entry.Caller)
	//	fc := caller.Function
	//	file := fmt.Sprintf("%s:%d", caller.File, caller.Line)
	//	b.WriteString(prettierCaller(file, fc))
	//}

	if f.CustomCaption != nil {
		captionStr := ""
		if f.CustomCaptionPrettyPrint {
			captionStr = marshalIndent(f.CustomCaption)
		} else {
			captionStr = marshal(f.CustomCaption)
		}
		if f.Colorize {
			color.Fprintf(b, " [%s] ", green(captionStr))
		} else {
			b.WriteString(" [" + captionStr + "] ")
		}
	}

	var data string
	if f.PrettyPrint {
		data = marshalIndent(root.Fields)
	} else {
		data = marshal(root.Fields)
	}
	if f.Colorize {
		color.Fprintf(b, "%s", data)
	} else {
		b.WriteString(data)
	}

	b.WriteByte('\n')

	return b.Bytes(), nil
}

func prettierCaller(file string, function string) string {
	dirs := strings.Split(file, "/")
	fileDesc := strings.Join(dirs[len(dirs)-2:], "/")

	funcs := strings.Split(function, ".")
	funcDesc := strings.Join(funcs[len(funcs)-2:], ".")

	return "[" + fileDesc + ":" + funcDesc + "]"
}

func encode(message string) interface{} {
	if data := encodeForJsonString(message); data != nil {
		return data
	} else {
		return message
	}
}

func encodeForJsonString(message string) map[string]interface{} {
	inInterface := make(map[string]interface{})
	if err := unmarshal(message, &inInterface); err != nil {
		return nil
	}
	return inInterface
}

func getColorByLevel(level logrus.Level) func(a ...any) string {
	switch level {
	case logrus.TraceLevel:
		return gray
	case logrus.DebugLevel:
		return blue
	case logrus.InfoLevel:
		return green
	case logrus.WarnLevel:
		return yellow
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		return red
	default:
		return darkBlue
	}
}

var defaultFormatter = &logrus.TextFormatter{DisableColors: true}

type PathMap map[logrus.Level]string
type WriterMap map[logrus.Level]io.Writer

type LogFileHook struct {
	paths     PathMap
	writers   WriterMap
	levels    []logrus.Level
	lock      *sync.Mutex
	formatter logrus.Formatter

	defaultPath      string
	defaultWriter    io.Writer
	hasDefaultPath   bool
	hasDefaultWriter bool
}

func NewFileHook(output interface{}, formatter logrus.Formatter) *LogFileHook {
	hook := &LogFileHook{
		lock: new(sync.Mutex),
	}

	hook.SetFormatter(formatter)

	switch output.(type) {
	case string:
		hook.SetDefaultPath(output.(string))
		break
	case io.Writer:
		hook.SetDefaultWriter(output.(io.Writer))
		break
	case PathMap:
		hook.paths = output.(PathMap)
		for level := range output.(PathMap) {
			hook.levels = append(hook.levels, level)
		}
		break
	case WriterMap:
		hook.writers = output.(WriterMap)
		for level := range output.(WriterMap) {
			hook.levels = append(hook.levels, level)
		}
		break
	default:
		panic(fmt.Sprintf("unsupported level map type: %v", reflect.TypeOf(output)))
	}

	return hook
}
func (hook *LogFileHook) SetFormatter(formatter logrus.Formatter) {
	hook.lock.Lock()
	defer hook.lock.Unlock()
	if formatter == nil {
		formatter = defaultFormatter
	} else {
		switch formatter.(type) {
		case *logrus.TextFormatter:
			textFormatter := formatter.(*logrus.TextFormatter)
			textFormatter.DisableColors = true
		}
	}

	hook.formatter = formatter
}
func (hook *LogFileHook) SetDefaultPath(defaultPath string) {
	hook.lock.Lock()
	defer hook.lock.Unlock()
	hook.defaultPath = defaultPath
	hook.hasDefaultPath = true
}
func (hook *LogFileHook) SetDefaultWriter(defaultWriter io.Writer) {
	hook.lock.Lock()
	defer hook.lock.Unlock()
	hook.defaultWriter = defaultWriter
	hook.hasDefaultWriter = true
}
func (hook *LogFileHook) Fire(entry *logrus.Entry) error {
	hook.lock.Lock()
	defer hook.lock.Unlock()
	if hook.writers != nil || hook.hasDefaultWriter {
		return hook.ioWrite(entry)
	} else if hook.paths != nil || hook.hasDefaultPath {
		return hook.fileWrite(entry)
	}

	return nil
}
func (hook *LogFileHook) ioWrite(entry *logrus.Entry) error {
	var (
		writer io.Writer
		msg    []byte
		err    error
		ok     bool
	)

	if writer, ok = hook.writers[entry.Level]; !ok {
		if hook.hasDefaultWriter {
			writer = hook.defaultWriter
		} else {
			return nil
		}
	}

	// use our formatter instead of entry.String()
	msg, err = hook.formatter.Format(entry)

	if err != nil {
		log.Println("failed to generate string for entry:", err)
		return err
	}
	_, err = writer.Write(msg)
	return err
}

func (hook *LogFileHook) fileWrite(entry *logrus.Entry) error {
	var (
		fd   *os.File
		path string
		msg  []byte
		err  error
		ok   bool
	)

	if path, ok = hook.paths[entry.Level]; !ok {
		if hook.hasDefaultPath {
			path = hook.defaultPath
		} else {
			return nil
		}
	}

	dir := filepath.Dir(path)
	os.MkdirAll(dir, os.ModePerm)

	fd, err = os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		log.Println("failed to open logfile:", path, err)
		return err
	}
	defer fd.Close()

	// use our formatter instead of entry.String()
	msg, err = hook.formatter.Format(entry)

	if err != nil {
		log.Println("failed to generate string for entry:", err)
		return err
	}
	fd.Write(msg)
	return nil
}
func (hook *LogFileHook) Levels() []logrus.Level {
	return logrus.AllLevels
}
