package board

import (
	"math"
	"runtime"
	"runtime/goos"
	"sync/atomic"
	"unsafe"

	"github.com/arbace/go-whim/guest/abi"
)

// SMP (doc/GUEST.md, *SMP*): TamaGo's runtime runs an M on each CPU it is
// given by goos.Task, never drops one, and wakes a halted one by
// goos.Wake.  Its own boards answer those with an IPI and HLT; here there
// are no interrupts, and the monitor answers them -- abi.CPUStart starts a
// vCPU it created idle, abi.CPUPark stops this one until an abi.CPUWake or
// the monitor lets it look again, abi.CPUSelf says which this is.

// spinNs is how long an idle vCPU spins before it parks: a goroutine made
// runnable meanwhile is seen without an exit.
const spinNs = 20_000

// stopWaitNs is the longest idle wait taken for a stop of the world's.
const stopWaitNs = 200_000

// tls is each AP's thread-local slot, by its number: the runtime's g, at
// FS's base less 8 on amd64 (the ELF's TLS, as the runtime's settls lays
// it; whim_apentry sets FS); arm64 keeps g in a register.  The boot
// vCPU's is the runtime's own, m0's.
//
//lint:ignore U1000 the assembly's
var tls [abi.MaxCPUs][2]uint64

// started counts the APs CPUStart has started.
var started uint64

// apEntry is whim_apentry's address: kept by the linker, as the monitor
// starts an AP there (its ELF symbol).
//
//lint:ignore U1000 the assembly's
var apEntry uintptr

func init() {
	if ncpu <= 1 {
		return
	}
	goos.ProcID = procID
	goos.Task = task
	goos.Wake = wake
	runtime.GOMAXPROCS(int(ncpu))
}

// task is goos.Task: an M started on the next idle vCPU, on its g0's own
// stack (as Linux's clone is given g0's stack, which TamaGo's runtime has
// allocated, 16 KiB), so that g0's bounds are the stack's; sp, the top of
// TamaGo's own 8 MiB for it, is unused, and the monitor takes its pages
// back.  The runtime asks once for each P past the first, so for each AP
// once.
//
//go:nosplit
func task(sp, _, gp, fn unsafe.Pointer) {
	id := atomic.AddUint64(&started, 1)
	if id >= ncpu {
		throw("task: no idle vCPU")
	}
	hi := *(*uintptr)(unsafe.Add(gp, 8)) // g.stack.hi
	if Call(abi.CPUStart, int64(id), int64(hi), int64(uintptr(gp)), int64(uintptr(fn)), int64(uintptr(sp))).Ret != 0 {
		throw("task: CPUStart refused")
	}
}

// procID is goos.ProcID: which vCPU this is, asked once per M.
func procID() uint64 { return uint64(Call(abi.CPUSelf, 0, 0, 0, 0, 0).Ret) }

// wake is goos.Wake: the M on vCPU procid parked in semasleep is let go.
//
//go:nosplit
func wake(procid uint64) { Call(abi.CPUWake, int64(procid), 0, 0, 0, 0) }

// idleSMP is goos.Idle on several vCPUs: a spin of spinNs, then a park
// until the deadline, a wake, or the monitor's limit (vmm/smp.go).  The
// scheduler calls it again while it finds nothing.
func idleSMP(until int64) {
	now := nanotime()
	end := now + spinNs
	limited := until > 0 && until != math.MaxInt64
	if limited && until-now <= stopWaitNs {
		// A wait this short is a stop of the world's (stopTheWorldWithSema
		// and forEachP wait 100 µs at a time): it waits for every idle
		// vCPU to look again, which TamaGo never asks one to do, so the
		// parked are woken.  A timer this near wakes them for nothing.
		Call(abi.CPUWake, -1, 0, 0, 0, 0)
	}
	if limited && until < end {
		end = until
	}
	for nanotime() < end {
		pause()
	}
	ns := int64(-1)
	if limited {
		if ns = until - nanotime(); ns <= 0 {
			return
		}
	}
	Call(abi.CPUPark, ns, 0, 0, 0, 0)
}

//go:linkname throw runtime.throw
func throw(string)
