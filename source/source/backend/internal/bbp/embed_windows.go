//go:build windows && amd64

package bbp

import (
	_ "embed"
	"os/exec"
	"syscall"
)

//go:embed assets/bbpPairings-windows-amd64.exe
var embedded []byte

func embeddedBinary() []byte { return embedded }

// hideWindow keeps the engine from flashing a console window.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
