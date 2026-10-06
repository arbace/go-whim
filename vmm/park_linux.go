package vmm

import (
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// A parker is a vCPU's park on Linux: a wake token in a word, waited for
// by futex with a timeout of nanoseconds.  The host's Go timers cannot
// time a park: a wait under a millisecond in a program whose threads are
// all blocked is netpoll's epoll_wait of one millisecond, so a park of 20
// or 80 µs lasted 1.1 ms, and every stop of the Go guest's world on
// several vCPUs waited that long for an idle one (doc/LISP-SANDBOX.md,
// *The collector in the box*).  futex is in the filter already.
type parker struct{ word atomic.Uint32 }

const (
	futexWaitPrivate = 128 | 0 // FUTEX_PRIVATE_FLAG | FUTEX_WAIT
	futexWakePrivate = 128 | 1 // FUTEX_PRIVATE_FLAG | FUTEX_WAKE
)

func (p *parker) init() {}

// wake ends the park, or the next: the token kept until it is taken.
func (p *parker) wake() {
	if p.word.Swap(1) == 0 {
		syscall.Syscall6(syscall.SYS_FUTEX, uintptr(unsafe.Pointer(&p.word)), futexWakePrivate, 1, 0, 0, 0)
	}
}

// wait waits for a wake, until until (zero: no limit): true when woken.
func (p *parker) wait(until time.Time) bool {
	for {
		if p.word.Swap(0) == 1 {
			return true
		}
		var ts *syscall.Timespec
		if !until.IsZero() {
			d := time.Until(until)
			if d <= 0 {
				return false
			}
			t := syscall.NsecToTimespec(int64(d))
			ts = &t
		}
		syscall.Syscall6(syscall.SYS_FUTEX, uintptr(unsafe.Pointer(&p.word)), futexWaitPrivate, 0, uintptr(unsafe.Pointer(ts)), 0, 0)
	}
}
