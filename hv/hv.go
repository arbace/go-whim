// Package hv is a virtual machine as Apple's Hypervisor.framework shapes one
// (its arm64 C API, <Hypervisor/Hypervisor.h>), one Go function for each C
// function used, in the same order of arguments, with the same names for
// registers, system registers and exits:
//
//	hv_vm_create(config)                 VMCreate(config)
//	hv_vm_destroy()                      VMDestroy()
//	hv_vm_map(addr, ipa, size, flags)    VMMap(mem, ipa, flags)
//	hv_vm_unmap(ipa, size)               VMUnmap(ipa, size)
//	hv_vcpu_create(&vcpu, &exit, config) VCPUCreate() (vcpu, exit, err)
//	hv_vcpu_destroy(vcpu)                VCPUDestroy(vcpu)
//	hv_vcpu_run(vcpu)                    VCPURun(vcpu)
//	hv_vcpus_exit(vcpus, count)          VCPUsExit(vcpus...)
//	hv_vcpu_get_reg / set_reg            VCPUGetReg / VCPUSetReg
//	hv_vcpu_get_sys_reg / set_sys_reg    VCPUGetSysReg / VCPUSetSysReg
//
// As in the framework there is one virtual machine per process, a vCPU is
// created, run and destroyed on one thread (the caller locks it:
// runtime.LockOSThread), and VCPURun returns with the exit record VCPUCreate
// handed out filled in -- hv_vcpu_exit_t's reason and, for an exception, its
// syndrome (ESR_EL2) and the faulting virtual and physical addresses.
//
// The Linux backend (kvm_linux*.go) is KVM through raw ioctl, no cgo.  It
// reports what the framework would: an MMIO store as an exception exit with a
// data-abort syndrome (ESR's EC 0x24, ISV set, SRT, SAS and WnR filled), the
// physical address in Exception.PhysicalAddress -- so a monitor decodes the
// framework's form on every target.  On arm64 the program counter after such
// an exit is the faulting store's, as the framework leaves it, and the
// monitor advances it.  The macOS backend is hvf_darwin.go, written as the
// cgo it would be and not built (doc/GUEST.md).
//
// amd64 sits behind the same shape with x86 names (regs_amd64.go): RAX..R15,
// RIP and RFLAGS as registers; CR0, CR3, CR4, EFER, the descriptor tables and
// each segment's selector, base, limit and access rights (VMX's encoding) as
// system registers.  Apple's x86 API is not the model.  On amd64 a store's
// exit leaves RIP past the store, since KVM has completed it.
package hv

import "fmt"

// Return is hv_return_t.  A function here returns nil for HV_SUCCESS and a
// Return otherwise.
type Return uint32

// The hv_return_t values (Hypervisor/hv_error.h).
const (
	Success      Return = 0
	Error        Return = 0xfae94001
	Busy         Return = 0xfae94002
	BadArgument  Return = 0xfae94003
	IllegalGuest Return = 0xfae94004
	NoResources  Return = 0xfae94005
	NoDevice     Return = 0xfae94006
	Denied       Return = 0xfae94007
	Unsupported  Return = 0xfae9400f
)

func (r Return) Error() string {
	switch r {
	case Error:
		return "HV_ERROR"
	case Busy:
		return "HV_BUSY"
	case BadArgument:
		return "HV_BAD_ARGUMENT"
	case IllegalGuest:
		return "HV_ILLEGAL_GUEST_STATE"
	case NoResources:
		return "HV_NO_RESOURCES"
	case NoDevice:
		return "HV_NO_DEVICE"
	case Denied:
		return "HV_DENIED"
	case Unsupported:
		return "HV_UNSUPPORTED"
	}
	return fmt.Sprintf("hv_return_t %#x", uint32(r))
}

// errnoError is a backend's failure with its cause kept: a Return for the
// framework's caller, the system's error for a person.
type errnoError struct {
	r    Return
	op   string
	errn error
}

func (e *errnoError) Error() string { return fmt.Sprintf("%s: %s: %v", e.r, e.op, e.errn) }
func (e *errnoError) Unwrap() error { return e.r }

// IPA is hv_ipa_t, a guest-physical address.
type IPA uint64

// MemoryFlags is hv_memory_flags_t.
type MemoryFlags uint64

// HV_MEMORY_READ, _WRITE, _EXEC.
const (
	MemoryRead  MemoryFlags = 1 << 0
	MemoryWrite MemoryFlags = 1 << 1
	MemoryExec  MemoryFlags = 1 << 2
)

// VMConfig is hv_vm_config_t: nil for the defaults.  IPASize is
// hv_vm_config_set_ipa_size's bits (0: the default, 36 on the framework).
type VMConfig struct {
	IPASize uint32
}

// VCPU is hv_vcpu_t.
type VCPU uint64

// ExitReason is hv_exit_reason_t.
type ExitReason uint32

// The hv_exit_reason_t values.
const (
	ExitReasonCanceled        ExitReason = 0
	ExitReasonException       ExitReason = 1
	ExitReasonVTimerActivated ExitReason = 2
	ExitReasonUnknown         ExitReason = 3
)

func (r ExitReason) String() string {
	switch r {
	case ExitReasonCanceled:
		return "HV_EXIT_REASON_CANCELED"
	case ExitReasonException:
		return "HV_EXIT_REASON_EXCEPTION"
	case ExitReasonVTimerActivated:
		return "HV_EXIT_REASON_VTIMER_ACTIVATED"
	}
	return "HV_EXIT_REASON_UNKNOWN"
}

// VCPUExitException is hv_vcpu_exit_exception_t.
type VCPUExitException struct {
	Syndrome        uint64 // ESR_EL2 as the exception set it
	VirtualAddress  uint64 // FAR_EL2 (0 where the backend has none)
	PhysicalAddress IPA    // HPFAR_EL2's, the faulting IPA
}

// VCPUExit is hv_vcpu_exit_t.  Detail is not the framework's: a backend's
// own account of an exit it reports as Unknown (KVM's exit reason and what
// it said), for a diagnostic.
type VCPUExit struct {
	Reason    ExitReason
	Exception VCPUExitException
	Detail    string
}

// The ESR_EL2 fields a monitor decodes (Arm ARM D23.2.41), for a data abort.
const (
	ECDataAbortLower = 0x24 // EC: a data abort from a lower exception level
	ECHVC64          = 0x16 // EC: HVC from AArch64
)

// Syndrome is an ESR_EL2 value, decoded.
type Syndrome uint64

// EC is the exception class, ESR[31:26].
func (s Syndrome) EC() uint32 { return uint32(s>>26) & 0x3f }

// IL is whether the trapped instruction was 32 bits, ESR[25].
func (s Syndrome) IL() bool { return s>>25&1 != 0 }

// ISV is whether a data abort's instruction syndrome (SAS, SRT, WnR) is valid,
// ISS[24].
func (s Syndrome) ISV() bool { return s>>24&1 != 0 }

// SAS is the access size, 1 << SAS bytes, ISS[23:22].
func (s Syndrome) SAS() uint32 { return uint32(s>>22) & 3 }

// SRT is the register the store read or the load wrote, ISS[20:16].
func (s Syndrome) SRT() uint32 { return uint32(s>>16) & 0x1f }

// WnR is whether the access was a write, ISS[6].
func (s Syndrome) WnR() bool { return s>>6&1 != 0 }

// DataAbort is the syndrome of a data abort from a lower level with a valid
// instruction syndrome: what the framework reports for a store to an
// unmapped IPA, and what the KVM backend makes of an MMIO exit.
func DataAbort(size int, srt uint32, write bool) Syndrome {
	sas := uint64(0)
	for 1<<sas < size {
		sas++
	}
	s := uint64(ECDataAbortLower)<<26 | 1<<25 | 1<<24 | sas<<22 | uint64(srt&0x1f)<<16
	if write {
		s |= 1 << 6
	}
	return Syndrome(s)
}
