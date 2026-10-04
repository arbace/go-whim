package suite

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// tcgets and tcsets are tcgetattr's and tcsetattr(TCSANOW)'s requests.
const tcgets, tcsets = syscall.TCGETS, syscall.TCSETS

// openPty is a pseudo-terminal: its master, and its slave by name.
func openPty() (*os.File, string, error) {
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, "", err
	}
	var unlock int32
	if err := ioctl(m, syscall.TIOCSPTLCK, unsafe.Pointer(&unlock)); err != nil {
		m.Close()
		return nil, "", err
	}
	var n uint32
	if err := ioctl(m, syscall.TIOCGPTN, unsafe.Pointer(&n)); err != nil {
		m.Close()
		return nil, "", err
	}
	return m, fmt.Sprintf("/dev/pts/%d", n), nil
}
