// Package abi is the guest's hypercall ABI as Go constants (doc/GUEST.md,
// *The hypercall ABI*): the doorbell's address, the call numbers and the
// call block's layout -- what guest/rt/rt.c spells in C for the C guest,
// and what the Go guest (guest/tamago, built with TamaGo) imports.  It
// imports nothing, so that a GOOS=tamago build can use it; vmm's test holds
// the monitor's own numbers to it.
package abi

// Doorbell is the guest-physical address no memory backs: a guest stores
// its call block's address there, one 64-bit store (from RAX on amd64, X0
// on arm64), and exits to the monitor.
const Doorbell = 0xf0000000

// The calls: the core's 17 host functions in doc/GUEST.md's order, the
// runtime's report of a fault, what a Go runtime asks besides (a Go
// guest's): random bytes, for its hashes' seeds; and the wait and the read
// merged, which both guests make in place of a wait.
// Alloc and Free are answered in the guest and never made.
const (
	HostInit = iota + 1
	GetWinsize
	TermStart
	TermStop
	TTYKeys
	NowMs
	Delay
	WaitForInput
	ReadInput
	Suspend
	Exit
	Message
	Alloc
	Free
	Write
	Time
	Raise
	Fault
	Random // a[0] a buffer, a[1] its length: filled with random bytes
	// WaitRead is wait_for_input and read_input merged (doc/GUEST.md, *The
	// merged wait and read*): a[0] the wait's ms, a[1] a buffer, a[2] its
	// length.  The monitor waits; when there is input, it reads into the
	// buffer at once.  ret is the wait's answer, and when it is 1, a[0] the
	// read's: the count, 0 at the end, -1 for a signal.
	WaitRead
	// The calls of a Go guest on several vCPUs (doc/GUEST.md, *SMP*): the
	// runtime's goos.Task, Idle, Wake and ProcID, which TamaGo's own boards
	// answer with an IPI and a halt, answered by the monitor.  None is a
	// host function: they are not serialised with the others.
	//
	// CPUStart starts vCPU a[0] (1 to the count less one), idle since the
	// boot, at the image's whim_apentry: a[1] its stack, a[2] the M's g0,
	// a[3] the function it calls (the runtime's mstart).  a[4] is the top
	// of the TaskStackBytes the runtime allocated for the M's stack, which
	// the board does not use (its g0's own is a[1]): the monitor gives
	// their pages back to the host.  ret is 0, or -1 when that vCPU does
	// not exist or has started already.
	CPUStart
	// CPUPark stops the calling vCPU until a CPUWake names it, a[0]
	// nanoseconds pass (a[0] < 0: no limit of the guest's), or the monitor
	// lets it look again (doc/GUEST.md, *SMP*: no longer than a
	// millisecond while another vCPU runs the guest).
	CPUPark
	// CPUWake ends vCPU a[0]'s park, or its next one when it is not parked.
	CPUWake
	// CPUSelf is the calling vCPU's number, in ret: 0 the boot vCPU.
	CPUSelf
	// The store (doc/LISP-SANDBOX.md, *The store*): the box's data, kept by
	// the monitor in a directory of the host's it is given
	// (WHIM_GUEST_STORE), so that no data need be built into an image.
	// Blobs are immutable and named by their SHA-256; refs are names a
	// guest gives a blob, the store's only mutable state.  Not host
	// functions: answered by the monitor, not serialised with the others.
	// Without a store every call answers -1.
	//
	// BlobPut stores a[1] bytes at a[0] and writes their SHA-256 to the 32
	// bytes at a[2]: ret 0, or -1.
	BlobPut
	// BlobSize is the size of the blob whose hash is the 32 bytes at a[0],
	// in ret, or -1 when the store has none.
	BlobSize
	// BlobGet reads the blob whose hash is at a[0] from offset a[3] into
	// a[2] bytes at a[1]: ret the count (0 past its end), or -1.
	BlobGet
	// RefGet writes the hash ref a[0]'s a[1] bytes name to the 32 bytes at
	// a[2]: ret 0, or -1 when there is no such ref.
	RefGet
	// RefSet points ref a[0] (a[1] bytes) at the hash at a[2] -- a blob the
	// store has -- when a[3] is 0, or when the ref's hash is the 32 bytes
	// at a[3] (compare and set; all zero: the ref must not exist): ret 0,
	// -1 for a name or hash it refuses, -2 when the comparison failed.  A
	// name is 1 to 64 of [A-Za-z0-9._-], not beginning with a dot.
	RefSet
	NCalls
)

// TaskStackBytes is the stack TamaGo's runtime allocates for each M it
// starts by goos.Task (runtime/os_tamago.go's stacksize), and clears.
const TaskStackBytes = 8 << 20

// MaxCPUs is the most vCPUs the monitor gives a guest: one page of the
// amd64 TSSs, 128 bytes apart.
const MaxCPUs = 32

// The call block, in guest RAM and aligned to its size: nr, a[5], ret,
// event -- the last two written by the monitor, event a deadly signal it
// caught during the call, or 0.
const (
	BlockNr    = 0
	BlockArgs  = 8
	BlockRet   = 48
	BlockEvent = 56
	BlockSize  = 64
)

// A Go guest's entry state on amd64 (vmm's boot of a TamaGo image): 64-bit
// ring 0, paging on and identity-mapped, interrupts off, RIP the ELF's
// entry, and in registers what the board needs before anything runs.
//
//	RDI argc        RSI argv (guest-physical, as the C guest's)
//	RDX RamStart    RCX RamSize      R8 RamStackOffset
//	R9  the time-stamp counter's rate, in kHz
//	R10 the vCPUs the guest has, 1 to MaxCPUs (arm64: X5)
//
// A vCPU CPUStart starts enters whim_apentry in the same mode, the boot
// vCPU's tables and the same system registers, its own TSS (amd64) or
// SP_EL1 (arm64) -- each its own fault stack -- and in registers:
//
//	RDI the M's g0        RSI the function to call      (arm64: X0, X1)
//	RDX its number        RSP the stack                 (arm64: X2, SP_EL0)
