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
	NCalls
)

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
