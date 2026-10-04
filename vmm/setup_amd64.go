package vmm

import (
	"debug/elf"
	"encoding/binary"

	"github.com/arbace/go-whim/hv"
)

// The amd64 guest starts in 64-bit ring 0 at its first instruction: no
// firmware, no real mode.  The monitor writes a GDT (a 64-bit code segment,
// a data segment and the TSS), the TSS (IST1 the fault stack), an IDT of 32
// interrupt gates into whim_vectors, and 4-level identity page tables; then
// sets the control registers, EFER and the segments as a CPU would hold
// them after loading that GDT.
const (
	elfMachine = elf.EM_X86_64
	gdtAddr    = sysBase
	tssAddr    = sysBase + 0x1000
	idtAddr    = sysBase + 0x2000
	selCode    = 0x08
	selData    = 0x10
	selTSS     = 0x18
	nVectors   = 32
	vectorSize = 16

	cr0PE, cr0MP, cr0ET, cr0NE, cr0WP, cr0PG = 1 << 0, 1 << 1, 1 << 4, 1 << 5, 1 << 16, 1 << 31
	cr4PAE                                   = 1 << 5
	eferLME, eferLMA, eferNXE                = 1 << 8, 1 << 10, 1 << 11

	ptP, ptRW, ptPS = 1 << 0, 1 << 1, 1 << 7
	ptNX            = 1 << 63
)

var pageFormat = ptFormat{
	levels: 4,
	table:  func(pa uint64) uint64 { return pa | ptP | ptRW },
	leaf: func(pa uint64, p perm, big bool) uint64 {
		e := pa | ptP
		if p&pWrite != 0 {
			e |= ptRW
		}
		if p&pExec == 0 {
			e |= ptNX
		}
		if big {
			e |= ptPS
		}
		return e
	},
}

// tssStride is the distance between two vCPUs' TSSs (doc/GUEST.md,
// *SMP*): each its own IST1, the fault stack of its own.
const tssStride = 0x80

// setup writes the system tables and sets the vCPU's registers.
func setup(v hv.VCPU, mem []byte, l *layout) error {
	le := binary.LittleEndian
	le.PutUint64(mem[gdtAddr+0x00:], 0)
	le.PutUint64(mem[gdtAddr+selCode:], 0x00af9b000000ffff) // P, DPL 0, code exec/read, L, G
	le.PutUint64(mem[gdtAddr+selData:], 0x00cf93000000ffff) // P, DPL 0, data read/write, D/B, G
	tss := uint64(tssAddr)
	le.PutUint64(mem[gdtAddr+selTSS:], tssLimit|(tss&0xffffff)<<16|0x8b<<40|(tss>>24&0xff)<<56)
	le.PutUint64(mem[gdtAddr+selTSS+8:], tss>>32)
	for i := range max(l.cpus, 1) {
		t := tssAddr + uint64(i)*tssStride
		le.PutUint64(mem[t+0x24:], l.faultTopOf(i)) // IST1
		le.PutUint16(mem[t+0x66:], tssLimit+1)      // no I/O bitmap
	}
	for i := range nVectors {
		h := l.vectors + uint64(i)*vectorSize
		e := idtAddr + uint64(i)*16
		le.PutUint64(mem[e:], h&0xffff|selCode<<16|1<<32|0x8e<<40|(h>>16&0xffff)<<48) // IST1, interrupt gate, P
		le.PutUint64(mem[e+8:], h>>32)
	}
	root, err := l.buildTables(mem, pageFormat)
	if err != nil {
		return err
	}
	if err := setupSys(v, l, root, 0); err != nil {
		return err
	}
	regs := []struct {
		r   hv.Reg
		val uint64
	}{
		{hv.RegRIP, l.entry},
		{hv.RegRSP, l.stackTop},
		{hv.RegRFLAGS, 0x2},
		{hv.RegRDI, l.argc},
		{hv.RegRSI, l.argv},
		{hv.RegRDX, l.heap},
	}
	for _, r := range regs {
		if err := hv.VCPUSetReg(v, r.r, r.val); err != nil {
			return err
		}
	}
	return nil
}

const tssLimit = 0x67

// setupSys sets vCPU i's system registers: every vCPU's the same but TR,
// which names its own TSS (the descriptor is the GDT's one, loaded by the
// monitor and never by the guest, so its busy bit says nothing).
func setupSys(v hv.VCPU, l *layout, root uint64, i int) error {
	sys := []struct {
		r   hv.SysReg
		val uint64
	}{
		{hv.SysRegCR0, cr0PE | cr0MP | cr0ET | cr0NE | cr0WP | cr0PG},
		{hv.SysRegCR3, root},
		{hv.SysRegCR4, cr4PAE},
		{hv.SysRegEFER, eferLME | eferLMA | eferNXE},
		{hv.SysRegGDTRBase, gdtAddr},
		{hv.SysRegGDTRLimit, selTSS + 16 - 1},
		{hv.SysRegIDTRBase, idtAddr},
		{hv.SysRegIDTRLimit, nVectors*16 - 1},
		{hv.SegTR.Selector(), selTSS},
		{hv.SegTR.Base(), tssAddr + uint64(i)*tssStride},
		{hv.SegTR.Limit(), tssLimit},
		{hv.SegTR.AR(), 0x8b},
	}
	segs := []struct {
		s   hv.Segment
		sel uint64
		ar  uint64
	}{
		{hv.SegCS, selCode, 0xa09b},
		{hv.SegDS, selData, 0xc093},
		{hv.SegES, selData, 0xc093},
		{hv.SegFS, selData, 0xc093},
		{hv.SegGS, selData, 0xc093},
		{hv.SegSS, selData, 0xc093},
	}
	for _, s := range segs {
		sys = append(sys, []struct {
			r   hv.SysReg
			val uint64
		}{{s.s.Selector(), s.sel}, {s.s.Base(), 0}, {s.s.Limit(), 0xffffffff}, {s.s.AR(), s.ar}}...)
	}
	for _, s := range sys {
		if err := hv.VCPUSetSysReg(v, s.r, s.val); err != nil {
			return err
		}
	}
	return nil
}

// setupAP is vCPU i's state until it starts: the boot vCPU's system
// registers, its own TSS.
func setupAP(v hv.VCPU, l *layout, i int) error { return setupSys(v, l, l.root, i) }

// startAP points vCPU v at the image's whim_apentry for abi.CPUStart: a[1]
// the stack, a[2] the M's g0 in RDI, a[3] the function in RSI, its number
// a[0] in RDX.
func startAP(v hv.VCPU, l *layout, a [5]int64) error {
	for _, r := range []struct {
		r   hv.Reg
		val uint64
	}{
		{hv.RegRIP, l.apEntry},
		{hv.RegRSP, uint64(a[1])},
		{hv.RegRFLAGS, 0x2},
		{hv.RegRDI, uint64(a[2])},
		{hv.RegRSI, uint64(a[3])},
		{hv.RegRDX, uint64(a[0])},
	} {
		if err := hv.VCPUSetReg(v, r.r, r.val); err != nil {
			return err
		}
	}
	return nil
}

// advance is what the monitor does to the vCPU after a doorbell's exit:
// nothing on amd64, where KVM has completed the store and RIP is past it.
func advance(hv.VCPU, hv.Syndrome) error { return nil }

// pc is the vCPU's instruction pointer, for a diagnostic.
func pc(v hv.VCPU) uint64 {
	r, _ := hv.VCPUGetReg(v, hv.RegRIP)
	return r
}
