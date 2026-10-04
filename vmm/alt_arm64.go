package vmm

import "github.com/arbace/go-whim/hv"

// AltHVC is the alternative trap's SMCCC function ID (guest/rt/rt.c's
// WHIM_HVC_FN): an HVC with it in X0 is a call, the call block's address in
// X1 -- measured against the doorbell (doc/GUEST.md, milestone 5).  KVM
// answers HVCs itself unless told to forward a range
// (KVM_ARM_VM_SMCCC_FILTER); the framework returns every one.
const AltHVC = 0xc3000057

// altCall is the register holding the call block when an exit is a call
// made the alternative way.
func altCall(v hv.VCPU, s hv.Syndrome, _ uint64) (hv.Reg, bool) {
	if s.EC() != hv.ECHVC64 {
		return 0, false
	}
	fn, err := hv.VCPUGetReg(v, hv.RegX0)
	return hv.RegX1, err == nil && fn == AltHVC
}

// vmSetup has KVM forward the alternative trap's HVC; a kernel without the
// filter (before 6.4) leaves the doorbell the only way, which it always is
// for the runtime as built by default.
func vmSetup() { hv.ForwardHVC(AltHVC, 1) }
