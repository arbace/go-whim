package vmm

import "github.com/arbace/go-whim/hv"

// AltPort is the alternative trap's port (guest/rt/rt.c's WHIM_PORT): an
// `out` there is a call, the call block's address in EAX -- measured against
// the doorbell (doc/GUEST.md, milestone 5).  KVM reports it as
// KVM_EXIT_IO, which hv reports as an exception of its own class.
const AltPort = 0x5157

// altCall is the register holding the call block when an exit is a call
// made the alternative way.
func altCall(_ hv.VCPU, s hv.Syndrome, addr uint64) (hv.Reg, bool) {
	return hv.RegRAX, s.EC() == hv.ECPortIO && addr == AltPort
}

// vmSetup is what the ISA's VM needs before its vCPU: nothing on amd64.
func vmSetup() {}
