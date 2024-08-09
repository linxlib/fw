package internal

import (
	"fmt"
	"github.com/gookit/color"
	"time"
)

var (
	white        = color.HiWhite.Render
	red          = color.HiRed.Render
	green        = color.HiGreen.Render
	blue         = color.HiBlue.Render
	cyan         = color.HiCyan.Render
	gray         = color.Gray.Render
	yellow       = color.HiYellow.Render
	magenta      = color.HiMagenta.Render
	lightmagenta = color.LightMagenta.Render
	info         = color.Info.Render
	note         = color.Note.Render
	err          = color.Error.Render
	danger       = color.Danger.Render
	success      = color.Success.Render
	warning      = color.Warn.Render
	question     = color.Question.Render
	primary      = color.Primary.Render
	secondary    = color.Secondary.Render

	fwFormat = "%s %s "
	tinfo    = "INFO"
	twarn    = "WARNING"
	tdebug   = "DEBUG"
	terr     = "ERROR"
	tfatal   = "FATAL"
	tok      = "√"
	tfail    = "×"
	tnone    = "SPAM"
)

func format(tag string) string {
	if tag == "" {
		return time.Now().Format("15:04:05")
	}
	return fmt.Sprintf(fwFormat, white(time.Now().Format("15:04:05")), tag)
}

func Infof(fmt string, args ...any) {
	color.Printf(format(tinfo)+white(fmt)+"\n", args...)
}
func Info(args ...any) {
	color.Print(format(white(tinfo)), white(args...)+"\n")
}
func Note(args ...any) {
	color.Print(white(args...) + "\n")
}

func Whitef(fmt string, args ...any) {
	color.Printf(format(tnone)+white(fmt)+"\n", args...)
}
func Errorf(fmt string, args ...any) {
	color.Printf(format(red(terr))+white(fmt)+"\n", args...)
}
func Error(fmt string, args ...any) {
	color.Printf(format(red(terr)), red(args...)+"\n")
}
func Redf(fmt string, args ...any) {
	color.Printf(format(tnone)+red(fmt)+"\n", args...)
}
func Debugf(fmt string, args ...any) {
	color.Printf(format(blue(tdebug))+green(fmt)+"\n", args...)
}
func Greenf(fmt string, args ...any) {
	color.Printf(format(tnone)+green(fmt)+"\n", args...)
}
func Warnf(fmt string, args ...any) {
	color.Printf(format(yellow(twarn))+yellow(fmt)+"\n", args...)
}
func Fatalf(fmt string, args ...any) {
	color.Printf(format(red(tfatal))+lightmagenta(fmt)+"\n", args...)
}
func OKf(fmt string, args ...any) {
	color.Printf(format(green(tok))+green(fmt)+"\n", args...)
}
func Failf(fmt string, args ...any) {
	color.Printf(format(red(tfail))+magenta(fmt)+"\n", args...)
}
func Yellowf(fmt string, args ...any) {
	color.Printf(format(tnone)+yellow(fmt)+"\n", args...)
}
