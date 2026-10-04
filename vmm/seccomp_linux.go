package vmm

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// The monitor's system-call filter (doc/GUEST.md, *The VMM*): once the VM
// is built, every thread of the process -- the Go runtime's too, and those
// it starts later, which inherit it -- may make only the calls the runtime,
// the terminal host and KVM_RUN need, and of ioctl only KVM_RUN and the
// terminal's three requests.  Anything else kills the process (SIGSYS).
// The guest has no system calls of its own to filter: its calls are the
// 15 the exit loop answers.
//
// The list was found by running both suites under the filter in log mode
// (WHIM_GUEST_SECCOMP=log: allowed, and logged by the kernel's audit) and
// reading what it logged; then held to them in kill mode.

const (
	seccompSetModeFilter  = 1
	seccompFlagTSYNC      = 1
	seccompRetKillProcess = 0x80000000
	seccompRetLog         = 0x7ffc0000
	seccompRetAllow       = 0x7fff0000
	prSetNoNewPrivs       = 38

	bpfLdWAbs = 0x20 // BPF_LD | BPF_W | BPF_ABS
	bpfJeqK   = 0x15 // BPF_JMP | BPF_JEQ | BPF_K
	bpfRetK   = 0x06 // BPF_RET | BPF_K

	offNr   = 0  // offsetof(struct seccomp_data, nr)
	offArch = 4  // .arch
	offArg1 = 24 // .args[1], its low word (little-endian)
)

// the ioctl requests allowed: KVM_RUN, the terminal's, and the ISA's
// register access in the exit loop (kvmIoctls)
var ioctlAllowed = append([]uint32{0xae80, 0x5401 /* TCGETS */, 0x5402 /* TCSETS */, 0x5413 /* TIOCGWINSZ */}, kvmIoctls...)

type sockFilter struct {
	code   uint16
	jt, jf uint8
	k      uint32
}

type sockFprog struct {
	len    uint16
	filter *sockFilter
}

// filter is the program: the arch checked, each allowed call let through,
// ioctl by its request, everything else deny.
func filter(deny uint32) []sockFilter {
	p := []sockFilter{
		{bpfLdWAbs, 0, 0, offArch},
		{bpfJeqK, 1, 0, auditArch},
		{bpfRetK, 0, 0, seccompRetKillProcess},
		{bpfLdWAbs, 0, 0, offNr},
	}
	for _, nr := range allowed {
		p = append(p, sockFilter{bpfJeqK, 0, 1, nr}, sockFilter{bpfRetK, 0, 0, seccompRetAllow})
	}
	p = append(p, sockFilter{bpfJeqK, 0, uint8(2*len(ioctlAllowed) + 1), sysIoctl}, sockFilter{bpfLdWAbs, 0, 0, offArg1})
	for _, req := range ioctlAllowed {
		p = append(p, sockFilter{bpfJeqK, 0, 1, req}, sockFilter{bpfRetK, 0, 0, seccompRetAllow})
	}
	return append(p, sockFilter{bpfRetK, 0, 0, deny})
}

// Seccomp puts the filter on every thread of the process.
// WHIM_GUEST_SECCOMP=log logs what it would have denied instead.
func Seccomp() error {
	deny := uint32(seccompRetKillProcess)
	if os.Getenv("WHIM_GUEST_SECCOMP") == "log" {
		deny = seccompRetLog
	}
	p := filter(deny)
	prog := sockFprog{uint16(len(p)), &p[0]}
	if _, _, e := syscall.RawSyscall6(syscall.SYS_PRCTL, prSetNoNewPrivs, 1, 0, 0, 0, 0); e != 0 {
		return fmt.Errorf("PR_SET_NO_NEW_PRIVS: %w", e)
	}
	if _, _, e := syscall.RawSyscall(sysSeccomp, seccompSetModeFilter, seccompFlagTSYNC, uintptr(unsafe.Pointer(&prog))); e != 0 {
		return fmt.Errorf("seccomp: %w", e)
	}
	return nil
}
