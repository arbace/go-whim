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
// the master's ioctls.
//
// THE MASTER IS A BLOCKING DESCRIPTOR.  os.OpenFile makes a file
// non-blocking and hands it to the runtime's poller, kqueue on macOS, which
// does not poll a tty: run on the Mac, the read of the master ended at once,
// nothing read the editor's output again, the editor stalled writing it
// before its last keys, and when the limit killed it the exit waited for
// that output to drain, for ever (ps: ?Es).  A descriptor opened by
// syscall.Open is blocking, and os.NewFile leaves a blocking one out of the
// poller: each read is a plain read(2) on a thread of its own.
func openPty() (*os.File, string, error) {
	fd, err := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, "", err
	}
	m := os.NewFile(uintptr(fd), "/dev/ptmx")
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
