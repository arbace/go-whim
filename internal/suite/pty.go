package suite

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"
)

// A terminal case: the editor on a pseudo-terminal of a given size and TERM.
type ptySpec struct {
	rows, cols int
	term       string
}

func ioctl(f *os.File, req uintptr, arg unsafe.Pointer) error {
	c, err := f.SyscallConn()
	if err != nil {
		return err
	}
	var errno syscall.Errno
	if err := c.Control(func(fd uintptr) {
		_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg))
	}); err != nil {
		return err
	}
	if errno != 0 {
		return errno
	}
	return nil
}

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

// RunPty runs bin on a pseudo-terminal and returns everything it wrote there
// and its exit status.
//
// EXACT, NOT PACED.  The slave is made raw and non-echoing before anything is
// written, every key is written before the editor starts, and only then is it
// started: it finds all its input waiting from its first read, whatever the
// load, so the output is the same on every run -- the quick suite's file, on a
// real terminal.  What the terminal adds is the paths a file never reaches:
// isatty, the window size asked of the terminal, the modes set and restored.
func RunPty(bin string, args []string, keys []byte, spec ptySpec) ([]byte, int, error) {
	m, slaveName, err := openPty()
	if err != nil {
		return nil, -1, err
	}
	defer m.Close()
	slave, err := os.OpenFile(slaveName, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, -1, err
	}
	defer slave.Close()
	ws := struct{ rows, cols, x, y uint16 }{uint16(spec.rows), uint16(spec.cols), 0, 0}
	if err := ioctl(slave, syscall.TIOCSWINSZ, unsafe.Pointer(&ws)); err != nil {
		return nil, -1, err
	}
	var t syscall.Termios
	if err := ioctl(slave, syscall.TCGETS, unsafe.Pointer(&t)); err != nil {
		return nil, -1, err
	}
	// cfmakeraw
	t.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP | syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	t.Oflag &^= syscall.OPOST
	t.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	t.Cflag &^= syscall.CSIZE | syscall.PARENB
	t.Cflag |= syscall.CS8
	if err := ioctl(slave, syscall.TCSETS, unsafe.Pointer(&t)); err != nil {
		return nil, -1, err
	}
	if _, err := m.Write(keys); err != nil {
		return nil, -1, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	env := []string{"TERM=" + spec.term, "PATH=" + os.Getenv("PATH"), "HOME=" + os.TempDir()}
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	if err := cmd.Start(); err != nil {
		return nil, -1, err
	}
	slave.Close() // the child holds it now; the master reads EOF (EIO) when it exits

	var out bytes.Buffer
	done := make(chan struct{})
	go func() {
		io.Copy(&out, m)
		close(done)
	}()
	werr := cmd.Wait()
	<-done
	if ctx.Err() != nil {
		return out.Bytes(), -1, fmt.Errorf("no exit within 10 s")
	}
	code := 0
	var ee *exec.ExitError
	if errors.As(werr, &ee) {
		code = ee.ExitCode()
	} else if werr != nil {
		return nil, -1, werr
	}
	return out.Bytes(), code, nil
}
