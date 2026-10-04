package vmm

import (
	"debug/elf"

	"github.com/arbace/go-whim/hv"
)

// The arm64 guest starts at EL1 with the MMU on, at its first instruction:
// the monitor writes 3-level identity page tables for a 32-bit space (4 KiB
// granule, 2 MiB blocks) and sets, as the framework names them, MAIR_EL1,
// TCR_EL1, TTBR0_EL1, VBAR_EL1 (whim_vectors), SP_EL0 (the stack), SP_EL1
// (the fault stack), SCTLR_EL1 (MMU and caches on), CPSR (EL1t, DAIF
// masked) and PC.  KVM/arm64 and the framework boot it the same way.
const (
	elfMachine = elf.EM_AARCH64

	mair      = 0x04<<8 | 0xff // attr0 normal write-back, attr1 device nGnRE
	tcr       = 32 | 1<<8 | 1<<10 | 3<<12 | 1<<23 | 1<<32
	sctlrRES1 = 1<<11 | 1<<20 | 1<<22 | 1<<23 | 1<<28 | 1<<29
	sctlr     = sctlrRES1 | 1<<0 | 1<<2 | 1<<12 // M, C, I
	cpsrEL1t  = 0x4 | 0xf<<6                    // EL1 on SP_EL0, D A I F masked

	dValid, dTable, dPage = 1 << 0, 1 << 1, 1 << 1
	dAttrDevice           = 1 << 2
	dRO                   = 1 << 7
	dSHInner              = 3 << 8
	dAF                   = 1 << 10
	dPXN, dUXN            = 1 << 53, 1 << 54
)

var pageFormat = ptFormat{
	levels: 3,
	table:  func(pa uint64) uint64 { return pa | dValid | dTable },
	leaf: func(pa uint64, p perm, big bool) uint64 {
		e := pa | dValid | dAF | dUXN
		if !big {
			e |= dPage
		}
		if p&pDevice != 0 {
			e |= dAttrDevice
		} else {
			e |= dSHInner
		}
		if p&pWrite == 0 {
			e |= dRO
		}
		if p&pExec == 0 {
			e |= dPXN
		}
		return e
	},
}

// setup writes the page tables and sets the vCPU's registers.
func setup(v hv.VCPU, mem []byte, l *layout) error {
	root, err := l.buildTables(mem, pageFormat)
	if err != nil {
		return err
	}
	for _, s := range []struct {
		r   hv.SysReg
		val uint64
	}{
		{hv.SysRegMAIREL1, mair},
		{hv.SysRegTCREL1, tcr},
		{hv.SysRegTTBR0EL1, root},
		{hv.SysRegVBAREL1, l.vectors},
		{hv.SysRegSPEL0, l.stackTop},
		{hv.SysRegSPEL1, l.faultTop},
		{hv.SysRegSCTLREL1, sctlr},
	} {
		if err := hv.VCPUSetSysReg(v, s.r, s.val); err != nil {
			return err
		}
	}
	for _, r := range []struct {
		r   hv.Reg
		val uint64
	}{
		{hv.RegCPSR, cpsrEL1t},
		{hv.RegPC, l.entry},
		{hv.RegX0, l.argc},
		{hv.RegX1, l.argv},
		{hv.RegX2, l.heap},
	} {
		if err := hv.VCPUSetReg(v, r.r, r.val); err != nil {
			return err
		}
	}
	return nil
}

// advance is what the monitor does after a doorbell's exit, as on the
// framework: PC moved past the store, which the data abort left it on.  An
// HVC's exit is past the HVC already.
func advance(v hv.VCPU, s hv.Syndrome) error {
	if s.EC() != hv.ECDataAbortLower {
		return nil
	}
	pc, err := hv.VCPUGetReg(v, hv.RegPC)
	if err != nil {
		return err
	}
	return hv.VCPUSetReg(v, hv.RegPC, pc+4)
}

// pc is the vCPU's program counter, for a diagnostic.
func pc(v hv.VCPU) uint64 {
	r, _ := hv.VCPUGetReg(v, hv.RegPC)
	return r
}
