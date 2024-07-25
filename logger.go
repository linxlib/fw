package fw

import "github.com/linxlib/fw/internal"

type Logger struct {
}

func (l Logger) Infof(format string, args ...interface{}) {
	internal.Infof(format, args...)
}

func (l Logger) Errorf(format string, args ...interface{}) {
	internal.Errorf(format, args...)
}
