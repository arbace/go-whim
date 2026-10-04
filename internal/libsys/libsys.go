//go:build darwin

// Package libsys is the few signal functions of macOS's libSystem that
// package syscall does not offer and a raw system call cannot replace:
// sigaction (the kernel's form takes the trampoline libc supplies, and the Go
// runtime installs its handlers through libc), pthread_sigmask, sigpending
// and pthread_kill -- bound by purego, no cgo, as hv binds
// Hypervisor.framework.  The terminal host's Suspend and the monitor's Die
// use them for what rt_sigaction, rt_sigprocmask, rt_sigpending and tgkill
// do on Linux.  NOT RUN here: written against <signal.h>, built for darwin
// as a compile check.
package libsys

import (
	"fmt"
	"sync"
	"syscall"

	"github.com/ebitengine/purego"
)

const lib = "/usr/lib/libSystem.B.dylib"

// Sigaction is macOS's struct sigaction: the handler (SIG_DFL 0, SIG_IGN 1,
// or a function), sa_mask (sigset_t, 32 bits) and sa_flags.
type Sigaction struct {
	Handler uintptr
	Mask    uint32
	Flags   int32
}

// SIG_DFL, and pthread_sigmask's hows: macOS's values, not Linux's.
const (
	SigDfl     = 0
	SigBlock   = 1
	SigUnblock = 2
	SigSetmask = 3
)

// Bit is sig's bit in a sigset_t.
func Bit(sig syscall.Signal) uint32 { return 1 << (uint(sig) - 1) }

var (
	sigaction      func(sig int32, act, old *Sigaction) int32
	pthreadSigmask func(how int32, set, old *uint32) int32
	sigpending     func(set *uint32) int32
	pthreadSelf    func() uintptr
	pthreadKill    func(t uintptr, sig int32) int32

	bind    sync.Once
	bindErr error
)

// Load binds the functions; every other function here calls it.
func Load() error {
	bind.Do(func() {
		h, err := purego.Dlopen(lib, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			bindErr = fmt.Errorf("libsys: %s: %v", lib, err)
			return
		}
		for name, fn := range map[string]any{
			"sigaction":       &sigaction,
			"pthread_sigmask": &pthreadSigmask,
			"sigpending":      &sigpending,
			"pthread_self":    &pthreadSelf,
			"pthread_kill":    &pthreadKill,
		} {
			purego.RegisterLibFunc(fn, h, name)
		}
	})
	return bindErr
}

// Action sets sig's action to act (when not nil) and returns the one before.
func Action(sig syscall.Signal, act *Sigaction) (Sigaction, error) {
	var old Sigaction
	if err := Load(); err != nil {
		return old, err
	}
	if sigaction(int32(sig), act, &old) != 0 {
		return old, fmt.Errorf("sigaction(%d) failed", sig)
	}
	return old, nil
}

// ThreadMask is pthread_sigmask(how, set) on the calling thread (the caller
// locks it), returning the mask before.
func ThreadMask(how int, set uint32) (uint32, error) {
	var old uint32
	if err := Load(); err != nil {
		return 0, err
	}
	if e := pthreadSigmask(int32(how), &set, &old); e != 0 {
		return 0, syscall.Errno(e)
	}
	return old, nil
}

// Pending is sigpending: the signals pending for the calling thread.
func Pending() (uint32, error) {
	var set uint32
	if err := Load(); err != nil {
		return 0, err
	}
	if sigpending(&set) != 0 {
		return 0, fmt.Errorf("sigpending failed")
	}
	return set, nil
}

// RaiseThread sends sig to the calling thread: pthread_kill(pthread_self()).
func RaiseThread(sig syscall.Signal) error {
	if err := Load(); err != nil {
		return err
	}
	if e := pthreadKill(pthreadSelf(), int32(sig)); e != 0 {
		return syscall.Errno(e)
	}
	return nil
}
