package vmm

import (
	"runtime"
	"syscall"

	"github.com/arbace/go-whim/internal/libsys"
)

// Die ends the process by SIGSEGV at its default action, as die_linux.go's
// does: the Go runtime's handler taken off through libc's sigaction, the
// signal unblocked on this thread and sent to it (pthread_kill, tgkill's
// counterpart).  NOT RUN here: built for darwin as a compile check.
func Die() {
	runtime.LockOSThread()
	libsys.Action(syscall.SIGSEGV, &libsys.Sigaction{Handler: libsys.SigDfl})
	libsys.ThreadMask(libsys.SigUnblock, libsys.Bit(syscall.SIGSEGV))
	if libsys.RaiseThread(syscall.SIGSEGV) != nil {
		syscall.Kill(syscall.Getpid(), syscall.SIGSEGV)
	}
	select {}
}
