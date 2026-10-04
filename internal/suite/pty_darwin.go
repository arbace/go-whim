package suite

import (
	"bytes"
	"os"
	"syscall"
	"unsafe"
)

// tcgets and tcsets are tcgetattr's and tcsetattr(TCSANOW)'s requests.
const tcgets, tcsets = syscall.TIOCGETA, syscall.TIOCSETA

// openPty is a pseudo-terminal: its master, and its slave by name --
// posix_openpt, grantpt, unlockpt and ptsname as macOS's libc does them, by
// the master's ioctls.  NOT RUN here: built for darwin as a compile check.
func openPty() (*os.File, string, error) {
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, "", err
	}
	if err := ioctl(m, syscall.TIOCPTYGRANT, nil); err != nil {
		m.Close()
		return nil, "", err
	}
	if err := ioctl(m, syscall.TIOCPTYUNLK, nil); err != nil {
		m.Close()
		return nil, "", err
	}
	var name [128]byte
	if err := ioctl(m, syscall.TIOCPTYGNAME, unsafe.Pointer(&name)); err != nil {
		m.Close()
		return nil, "", err
	}
	n := bytes.IndexByte(name[:], 0)
	if n < 0 {
		n = len(name)
	}
	return m, string(name[:n]), nil
}
