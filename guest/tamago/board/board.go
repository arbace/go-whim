// Package board is a TamaGo board for whim's monitor (vmm/, doc/GUEST.md):
// the runtime hooks of GOOS=tamago (runtime/goos) answered as the C guest
// answers its host -- by hypercalls through the doorbell -- or in the
// guest where an exit would cost too much.
//
//	CPUInit        cpuinit (boot_ISA.s): the monitor's entry state taken
//	               (guest/abi), SSE on (amd64) or FP and
//	               SIMD (arm64), the first stack, the runtime's rt0
//	Hwinit0        nothing: the monitor has set the machine up
//	Hwinit1        goos.Exit and goos.Idle set
//	Nanotime       the counter (TSC; CNTVCT_EL0), at the rate the monitor
//	               said (amd64) or CNTFRQ_EL0 says (arm64):
//	               in the guest, since the scheduler asks it often
//	Printk         a line buffered, then abi.Message to stderr: a call
//	GetRandomData  abi.Random: a call
//	InitRNG        nothing
//	Exit           abi.Exit: a call, which never returns
//	Idle           one vCPU: a spin to the deadline, there being no
//	               interrupt to wait for; several: a spin, then abi.CPUPark
//	RamStart, RamSize, RamStackOffset
//	               the monitor's, in registers at the entry
//	Task, Wake, ProcID
//	               on several vCPUs (smp.go): abi.CPUStart, abi.CPUWake
//	               and abi.CPUSelf
//
// No interrupts, no timer: the runtime schedules its goroutines
// cooperatively on each vCPU, and a vCPU only stops in a hypercall.  An
// exception goes through the monitor's IDT to whim_vectors, which reports
// it by abi.Fault as the C guest's vectors do.
package board

import (
	"math"
	"math/bits"
	"runtime/goos"
	"unsafe"

	"github.com/arbace/go-whim/guest/abi"
)

// For the assembly (go_asm.h).
const (
	//lint:ignore U1000 the assembly's
	doorbellAddr = abi.Doorbell
	//lint:ignore U1000 the assembly's
	callFault = abi.Fault
	//lint:ignore U1000 the assembly's
	blockSize = abi.BlockSize
)

// What cpuinit keeps of the entry state.
var (
	bootArgc  uint64
	bootArgv  unsafe.Pointer // **byte, guest-physical: the guest is identity-mapped
	tickKHz   uint64         // the counter's rate: the monitor's (amd64), CNTFRQ_EL0's (arm64)
	tickBase  uint64         // the counter at the entry: nanotime's zero
	nsPerTick uint64         // nanoseconds a tick, 32.32 fixed point
	ncpu      uint64         // the vCPUs the monitor gave: 1, or more (smp.go)
	//lint:ignore U1000 the assembly's: whim_vectors, where the IDT's gates go
	vectors uintptr
)

// Block is the call block (guest/abi): what the monitor reads and writes.
type Block struct {
	Nr    uint64
	A     [5]int64
	Ret   int64
	Event uint64
}

// faultArea holds the vectors' call block, aligned to the block's size
// when used.
//
//lint:ignore U1000 the assembly's
var faultArea [2 * abi.BlockSize]byte

//go:noescape
func doorbell(cb unsafe.Pointer)
func ticks() uint64 // the time-stamp counter, CNTVCT_EL0
func pause()

// Call makes hypercall nr and returns the block as the monitor left it.
// The block is on the caller's stack, aligned to its size: each call its
// own, whichever vCPU makes it.  Pointer arguments are addresses
// (guest-physical is virtual here); it grows no stack, so an address taken
// just before it stays good.
//
//go:nosplit
func Call(nr uint64, a0, a1, a2, a3, a4 int64) Block {
	var area [2 * abi.BlockSize]byte
	p := unsafe.Pointer(&area[0])
	b := (*Block)(unsafe.Add(p, -uintptr(p)&(abi.BlockSize-1)))
	*b = Block{Nr: nr, A: [5]int64{a0, a1, a2, a3, a4}}
	doorbell(unsafe.Pointer(b))
	return *b
}

// Addr is the address of p's first byte, 0 when it has none.
//
//go:nosplit
func Addr(p []byte) int64 {
	if len(p) == 0 {
		return 0
	}
	return int64(uintptr(unsafe.Pointer(&p[0])))
}

// Args is the command line the monitor wrote into guest memory.
func Args() []string {
	if bootArgv == nil {
		return nil
	}
	ptrs := unsafe.Slice((**byte)(bootArgv), int(bootArgc))
	args := make([]string, len(ptrs))
	for i, p := range ptrs {
		n := 0
		for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
			n++
		}
		args[i] = string(unsafe.Slice(p, n))
	}
	return args
}

// Exit ends the guest with code: the console flushed, then abi.Exit, which
// the monitor never resumes.
//
//go:nosplit
func Exit(code int32) {
	flush()
	for {
		Call(abi.Exit, int64(code), 0, 0, 0, 0)
	}
}

//go:linkname hwinit0 runtime/goos.Hwinit0
func hwinit0() {}

//go:linkname hwinit1 runtime/goos.Hwinit1
func hwinit1() {
	goos.Exit = Exit
	goos.Idle = idle
}

// idle waits for the scheduler's next timer.  On one vCPU it spins: nothing
// but a timer can make a goroutine runnable (no interrupts, and the vCPU
// is stopped while a call is answered); with no timer it returns, and the
// scheduler looks again -- a guest deadlocked spins, and the monitor's
// watchdog ends it.  On several, another vCPU can, and the idle one parks
// (smp.go).
func idle(until int64) {
	if ncpu > 1 {
		idleSMP(until)
		return
	}
	if until <= 0 || until == math.MaxInt64 {
		return
	}
	for nanotime() < until {
		pause()
	}
}

//go:linkname nanotime runtime/goos.Nanotime
//go:nosplit
func nanotime() int64 {
	if nsPerTick == 0 {
		khz := tickKHz
		if khz == 0 {
			khz = 1000000 // no rate said: a guess of 1 GHz
		}
		nsPerTick = (1000000 << 32) / khz
	}
	hi, lo := bits.Mul64(ticks()-tickBase, nsPerTick)
	return int64(hi<<32 | lo>>32)
}

// The console: the runtime's own output (a panic's report), a line at a
// time to the monitor's stderr.
var (
	line  [256]byte
	lineN int
)

//go:linkname printk runtime/goos.Printk
//go:nosplit
func printk(c byte) {
	line[lineN] = c
	lineN++
	if c == '\n' || lineN == len(line) {
		flush()
	}
}

//go:nosplit
func flush() {
	if lineN > 0 {
		Call(abi.Message, int64(uintptr(unsafe.Pointer(&line[0]))), int64(lineN), 1, 0, 0)
		lineN = 0
	}
}

//go:linkname initRNG runtime/goos.InitRNG
func initRNG() {}

//go:linkname getRandomData runtime/goos.GetRandomData
//go:nosplit
func getRandomData(b []byte) {
	if len(b) > 0 {
		Call(abi.Random, Addr(b), int64(len(b)), 0, 0, 0)
	}
}
