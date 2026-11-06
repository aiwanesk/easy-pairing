//go:build !(windows && amd64)

package bbp

import "os/exec"

func embeddedBinary() []byte { return nil }

func hideWindow(*exec.Cmd) {}
