package types

import "github.com/gookit/color"

type ILogger interface {
	Info(args []Arg)
}

type Logger struct {
}

type Arg struct {
	Key   string
	Value color.Color
}

func (l *Logger) Info(args []Arg) {
	start := color.HiGreen.Sprint("INFO") + color.HiWhite.Sprint(":")
	for _, content := range args {
		start += " " + content.Value.Sprint(content.Key)
	}
	color.Println(start)
}
