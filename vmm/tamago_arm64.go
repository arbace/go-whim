package vmm

import (
	"github.com/arbace/go-whim/hv"
)

// setupGo is a Go guest's entry state past the C guest's (guest/abi): X2
// RamStart (in place of the C guest's heap), X3 RamSize, X4 the offset of
// the runtime's first stack under the top.  The counter's rate needs no
// register: the board reads CNTFRQ_EL0, which KVM/arm64 lets EL1 read, as
// it does CNTVCT_EL0 (doc/GUEST.md, *The second guest*, arm64).
func setupGo(v hv.VCPU, l *layout) error {
	regs := []struct {
		r   hv.Reg
		val uint64
	}{
		{hv.RegX2, l.ramStart},
		{hv.RegX3, l.size - l.ramStart},
		{hv.RegX4, l.size - l.stackTop},
	}
	for _, r := range regs {
		if err := hv.VCPUSetReg(v, r.r, r.val); err != nil {
			return err
		}
	}
	return nil
}
