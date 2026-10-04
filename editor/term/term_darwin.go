package term

import (
	"math/bits"
	"runtime"
	"syscall"
	"time"

	"github.com/arbace/go-whim/internal/libsys"
)

// The macOS terminal host: what term_linux.go is on Linux, with macOS's
// requests and calls and the same behaviour.  NOT RUN here: built for
// darwin/arm64 and darwin/amd64 as a compile check (doc/GUEST.md, *Running
// on the Mac*).

// OXTABS, macOS's XTABS: the tab expansion the C host's XTABS clears.
const xtabs = 0x4

// tcgets and tcsets are tcgetattr's and tcsetattr(TCSANOW)'s ioctl requests.
const tcgets, tcsets = syscall.TIOCGETA, syscall.TIOCSETA

// wakePipe is the wake-up pipe, non-blocking and close-on-exec: macOS has no
// pipe2, so pipe and then the two flags, before anything can fork.
func wakePipe() (r, w int, ok bool) {
	var fds [2]int
	if syscall.Pipe(fds[:]) != nil {
		return -1, -1, false
	}
	for _, fd := range fds {
		if syscall.SetNonblock(fd, true) != nil {
			syscall.Close(fds[0])
			syscall.Close(fds[1])
			return -1, -1, false
		}
		syscall.CloseOnExec(fd)
	}
	return fds[0], fds[1], true
}

// sel is select(2) on reading as Linux's behaves: it returns the number of
// descriptors ready and leaves the time remaining in tv, so that a wait
// resumed after an EINTR or after the wake-up pipe alone woke it goes on for
// the rest.  macOS's select returns no count and leaves tv as it was.
func sel(nfd int, r *syscall.FdSet, tv *syscall.Timeval) (int, error) {
	var end time.Time
	if tv != nil {
		end = time.Now().Add(time.Duration(tv.Nano()))
	}
	err := syscall.Select(nfd, r, nil, nil, tv)
	if tv != nil {
		left := time.Until(end)
		if left < 0 {
			left = 0
		}
		*tv = syscall.NsecToTimeval(left.Nanoseconds())
	}
	if err != nil {
		return -1, err
	}
	n := 0
	for i := 0; i*nfdbits < nfd; i++ {
		n += bits.OnesCount32(uint32(r.Bits[i]))
	}
	return n, nil
}

// Suspend stops the process group with SIGTSTP at its default action, as
// term_linux.go's does and for the same reasons (read its comment): the
// action set to SIG_DFL and the runtime's restored after, through libc's
// sigaction, since the runtime installs its handlers through libc; SIGTSTP
// blocked on this goroutine's locked thread for the kill and the wait, so
// that another thread takes it; the wait bounded by a second, ended when the
// signal is no longer pending here.
func (h *Host) Suspend() {
	old, err := libsys.Action(syscall.SIGTSTP, nil)
	if err != nil {
		// DEVIATION (fallback only): stop with SIGSTOP instead
		syscall.Kill(0, syscall.SIGSTOP)
		return
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	tstp := libsys.Bit(syscall.SIGTSTP)
	mask, _ := libsys.ThreadMask(libsys.SigBlock, tstp)
	dfl := old
	dfl.Handler = libsys.SigDfl
	libsys.Action(syscall.SIGTSTP, &dfl)
	syscall.Kill(0, syscall.SIGTSTP)
	for end := time.Now().Add(time.Second); time.Now().Before(end); {
		if pending, err := libsys.Pending(); err != nil || pending&tstp == 0 {
			break
		}
	}
	libsys.Action(syscall.SIGTSTP, &old)
	libsys.ThreadMask(libsys.SigSetmask, mask)
}
