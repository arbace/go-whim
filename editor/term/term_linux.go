package term

import (
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

// XTABS (TABDLY) is not in package syscall; Linux's value.
const xtabs = 0x1800

// tcgets and tcsets are tcgetattr's and tcsetattr(TCSANOW)'s ioctl requests.
const tcgets, tcsets = syscall.TCGETS, syscall.TCSETS

// wakePipe is the wake-up pipe, non-blocking and close-on-exec.
func wakePipe() (r, w int, ok bool) {
	var fds [2]int
	if syscall.Pipe2(fds[:], syscall.O_NONBLOCK|syscall.O_CLOEXEC) != nil {
		return -1, -1, false
	}
	return fds[0], fds[1], true
}

// sel is select(2) on reading.  Linux's select leaves the time remaining in
// tv, so an EINTR from the Go runtime's own signals resumes the same wait.
func sel(nfd int, r *syscall.FdSet, tv *syscall.Timeval) (int, error) {
	return syscall.Select(nfd, r, nil, nil, tv)
}

// Suspend stops the process group with SIGTSTP at its default action.  Go's
// signal.Reset would leave the runtime's handler installed, which swallows
// SIGTSTP, so the kernel action is set to SIG_DFL with rt_sigaction directly
// and the runtime's own restored after.
//
// DEVIATION: the C is one thread, which takes the signal and stops as kill
// returns.  A Go process is many, and the kernel hands the signal to its first
// thread, which takes it when it next runs: with the runtime's handler put
// back at once, that thread found it and did not stop -- the Go editor's
// :suspend never stopped (0 of 100 runs, the C 100).  So the handler waits
// until the signal is no longer pending: taken, and the process stopped and
// continued (or, in an orphaned process group, discarded by the kernel).
// rt_sigpending reports only what the asking thread blocks, so this goroutine
// holds its thread and blocks SIGTSTP on it for the kill and the wait: another
// thread takes it, and until one has, it is pending here.  At most a second,
// a bound no run has reached.  Braaam's host (Term.suspend) does the same.
func (h *Host) Suspend() {
	var old, dfl [4]uint64 // struct kernel_sigaction: handler, flags, restorer, mask

	_, _, e := syscall.RawSyscall6(syscall.SYS_RT_SIGACTION, uintptr(syscall.SIGTSTP), 0, uintptr(unsafe.Pointer(&old)), 8, 0, 0)
	if e != 0 {
		// DEVIATION (fallback only): stop with SIGSTOP instead
		syscall.Kill(0, syscall.SIGSTOP)
		return
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	const tstp = uint64(1) << (syscall.SIGTSTP - 1)
	set, mask := tstp, uint64(0)
	syscall.RawSyscall6(syscall.SYS_RT_SIGPROCMASK, 0 /* SIG_BLOCK */, uintptr(unsafe.Pointer(&set)), uintptr(unsafe.Pointer(&mask)), 8, 0, 0)
	dfl = old
	dfl[0] = 0 // SIG_DFL
	syscall.RawSyscall6(syscall.SYS_RT_SIGACTION, uintptr(syscall.SIGTSTP), uintptr(unsafe.Pointer(&dfl)), 0, 8, 0, 0)
	syscall.Kill(0, syscall.SIGTSTP)
	for end := time.Now().Add(time.Second); time.Now().Before(end); {
		var pending uint64
		if _, _, e := syscall.RawSyscall(syscall.SYS_RT_SIGPENDING, uintptr(unsafe.Pointer(&pending)), 8, 0); e != 0 || pending&tstp == 0 {
			break
		}
	}
	syscall.RawSyscall6(syscall.SYS_RT_SIGACTION, uintptr(syscall.SIGTSTP), uintptr(unsafe.Pointer(&old)), 0, 8, 0, 0)
	syscall.RawSyscall6(syscall.SYS_RT_SIGPROCMASK, 2 /* SIG_SETMASK */, uintptr(unsafe.Pointer(&mask)), 0, 8, 0, 0)
}
