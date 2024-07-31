package binding

import (
	"github.com/linxlib/conv"
	"strings"
)

func arrValues[T []byte | any](v T) []string {
	value := conv.String(v)
	var t = []string{value}
	if strings.ContainsAny(value, ",") {
		t = strings.Split(value, ",")
	}
	return t
}
