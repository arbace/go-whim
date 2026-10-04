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
//	Idle           a spin to the deadline: there is no interrupt to wait for
//	RamStart, RamSize, RamStackOffset
//	               the monitor's, in registers at the entry
//
// One vCPU, no interrupts, no timer: the runtime schedules its goroutines
// cooperatively on it, and the vCPU only stops in a hypercall.  An
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

// area holds the call block and faultArea the vectors' one, each aligned
// to the block's size when used.
var (
	area [2 * abi.BlockSize]byte
	//lint:ignore U1000 the assembly's
	faultArea [2 * abi.BlockSize]byte
)

func doorbell(cb uintptr)
func ticks() uint64 // the time-stamp counter, CNTVCT_EL0
func pause()

//go:nosplit
func block() *Block {
	p := unsafe.Pointer(&area[0])
	return (*Block)(unsafe.Add(p, -uintptr(p)&(abi.BlockSize-1)))
}

// Call makes hypercall nr and returns the block as the monitor left it.
// Pointer arguments are addresses (guest-physical is virtual here); it
// grows no stack, so an address taken just before it stays good.
//
//go:nosplit
func Call(nr uint64, a0, a1, a2, a3, a4 int64) Block {
	b := block()
	*b = Block{Nr: nr, A: [5]int64{a0, a1, a2, a3, a4}}
	doorbell(uintptr(unsafe.Pointer(b)))
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

// idle waits for the scheduler's next timer by spinning: nothing but a
// timer can make a goroutine runnable here (no interrupts, and the vCPU
// is stopped while a call is answered).  With no timer it returns, and the
// scheduler looks again: a guest deadlocked spins, and the monitor's
// watchdog ends it.
func idle(until int64) {
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
