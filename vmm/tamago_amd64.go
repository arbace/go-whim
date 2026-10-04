package vmm

import (
	"github.com/arbace/go-whim/hv"
)

// setupGo is a Go guest's entry state past the C guest's (guest/abi): the
// RAM the runtime may use, the offset of its first stack under the top,
// and the time-stamp counter's rate, which TamaGo's nanotime scales -- read
// from KVM, since amd64 has no register that says it.  The board enables
// SSE itself, as its own first instructions.
func setupGo(v hv.VCPU, l *layout) error {
	khz, err := hv.TSCFrequency(v)
	if err != nil {
		return err
	}
	regs := []struct {
		r   hv.Reg
		val uint64
	}{
		{hv.RegRDX, l.ramStart},
		{hv.RegRCX, l.size - l.ramStart},
		{hv.RegR8, l.size - l.stackTop},
		{hv.RegR9, khz},
	}
	for _, r := range regs {
		if err := hv.VCPUSetReg(v, r.r, r.val); err != nil {
			return err
		}
	}
	return nil
}
