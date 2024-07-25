package types

type ILogger interface {
	Infof(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}
