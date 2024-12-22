package utils

import "testing"

func TestRunCmd(t *testing.T) {
	RunCmd("ping", "223.5.5.5")
}
