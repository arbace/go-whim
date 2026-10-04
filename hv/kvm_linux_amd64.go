package hv

import (
	"syscall"
	"unsafe"
)

// The amd64 half of the KVM backend.  The general registers travel in
// kvm_run's synced area (KVM_CAP_SYNC_REGS): copied out by KVM at every exit
// and in when changed, so reading RAX or RIP after an exit costs no system
// call.  struct kvm_sregs is fetched when first asked for after a run, and
// set before the next one when changed.
const (
	kvmGetRegs          = 0x8090ae81
	kvmGetSregs         = 0x8138ae83
	kvmSetSregs         = 0x4138ae84
	kvmGetSupportedCPU  = 0xc008ae05
	kvmSetCPUID2        = 0x4008ae90
	kvmSetTSSAddr       = 0xae47
	kvmSyncX86Regs      = 1
	kvmCPUIDEntries     = 256
	tssAddr             = 0xfffbd000 // three pages under 4 GiB VMX wants for itself
	nRegs               = 18
	sizeofKvmRegs       = nRegs * 8
	sizeofKvmCpuidEntry = 40
)

// ECPortIO is not an Arm exception class: it is the class this backend
// gives an I/O-port exit (`out` on amd64), whose port is the exit's
// PhysicalAddress and whose value RAX holds (SRT 0).  KVM completes the
// instruction when the vCPU runs again.
const ECPortIO = 0x3f

type vmISA struct{}

type kvmSegment struct {
	base                                    uint64
	limit                                   uint32
	selector                                uint16
	typ, present, dpl, db, s, l, g, avl, un uint8
	_                                       uint8
}

type kvmDtable struct {
	base  uint64
	limit uint16
	_     [3]uint16
}

type kvmSregs struct {
	seg                              [8]kvmSegment // cs ds es fs gs ss tr ldt
	gdt, idt                         kvmDtable
	cr0, cr2, cr3, cr4, cr8, efer, _ uint64
	_                                [4]uint64
}

type vcpuISA struct {
	regs                  [nRegs]uint64
	regsDirty             bool
	sregs                 kvmSregs
	sregsValid, sregsDirt bool
}

func vmType(uint32) uintptr { return 0 }

func vmInit() error {
	if _, err := ioctl(vm.fd, kvmSetTSSAddr, tssAddr); err != nil {
		return fail(Error, "KVM_SET_TSS_ADDR", err)
	}
	return nil
}

func vcpuInit(c *vcpu) error {
	if caps, _ := ioctl(vm.kvm, kvmCheckExtension, kvmCapSyncRegs); caps&kvmSyncX86Regs == 0 {
		return fail(Unsupported, "KVM_CAP_SYNC_REGS", syscall.ENOSYS)
	}
	// the host's CPUID, as KVM supports it: without it the guest has no NX,
	// and EFER.NXE is refused
	buf := make([]byte, 8+kvmCPUIDEntries*sizeofKvmCpuidEntry)
	*(*uint32)(unsafe.Pointer(&buf[0])) = kvmCPUIDEntries
	if _, err := ioctl(vm.kvm, kvmGetSupportedCPU, uintptr(unsafe.Pointer(&buf[0]))); err != nil {
		return fail(Error, "KVM_GET_SUPPORTED_CPUID", err)
	}
	if _, err := ioctl(c.fd, kvmSetCPUID2, uintptr(unsafe.Pointer(&buf[0]))); err != nil {
		return fail(Error, "KVM_SET_CPUID2", err)
	}
	if _, err := ioctl(c.fd, kvmGetRegs, uintptr(unsafe.Pointer(&c.isa.regs[0]))); err != nil {
		return fail(Error, "KVM_GET_REGS", err)
	}
	c.put64(kvmRunValidRegs, kvmSyncX86Regs)
	return nil
}

func beforeRun(c *vcpu) error {
	if c.isa.sregsDirt {
		if _, err := ioctl(c.fd, kvmSetSregs, uintptr(unsafe.Pointer(&c.isa.sregs))); err != nil {
			return fail(IllegalGuest, "KVM_SET_SREGS", err)
		}
		c.isa.sregsDirt = false
	}
	if c.isa.regsDirty {
		copy(c.run[kvmRunSyncRegs:kvmRunSyncRegs+sizeofKvmRegs], unsafe.Slice((*byte)(unsafe.Pointer(&c.isa.regs[0])), sizeofKvmRegs))
		c.put64(kvmRunDirtyRegs, kvmSyncX86Regs)
		c.isa.regsDirty = false
	}
	return nil
}

func afterRun(c *vcpu, _ bool) {
	c.put64(kvmRunDirtyRegs, 0)
	copy(unsafe.Slice((*byte)(unsafe.Pointer(&c.isa.regs[0])), sizeofKvmRegs), c.run[kvmRunSyncRegs:kvmRunSyncRegs+sizeofKvmRegs])
	c.isa.sregsValid = false
}

// mmioExit is a store's syndrome: SRT 0, RAX, when RAX holds the value
// stored (the guest's ABI); without a valid instruction syndrome when not.
// RIP is past the store already.
func mmioExit(c *vcpu, n int, write bool, data uint64) Syndrome {
	s := DataAbort(n, 0, write)
	mask := ^uint64(0)
	if n < 8 {
		mask = 1<<(8*n) - 1
	}
	if c.isa.regs[RegRAX]&mask != data {
		s &^= 1 << 24
	}
	return s
}

// isaExit reports an `out` to a port as an ECPortIO exception exit.
func isaExit(c *vcpu, reason uint32) bool {
	if reason != kvmExitIO || c.run[kvmIODirection] != 1 {
		return false
	}
	n := int(c.run[kvmIOSize])
	s := DataAbort(n, 0, true)
	s = s&^(0x3f<<26) | ECPortIO<<26
	c.exit.Reason = ExitReasonException
	c.exit.Exception.Syndrome = uint64(s)
	c.exit.Exception.PhysicalAddress = IPA(c.u16(kvmIOPort))
	return true
}

// VCPUGetReg is hv_vcpu_get_reg.
func VCPUGetReg(v VCPU, r Reg) (uint64, error) {
	c, err := get(v)
	if err != nil {
		return 0, err
	}
	if r >= nRegs {
		return 0, BadArgument
	}
	return c.isa.regs[r], nil
}

// VCPUSetReg is hv_vcpu_set_reg.
func VCPUSetReg(v VCPU, r Reg, val uint64) error {
	c, err := get(v)
	if err != nil {
		return err
	}
	if r >= nRegs {
		return BadArgument
	}
	c.isa.regs[r] = val
	c.isa.regsDirty = true
	return nil
}

func (c *vcpu) sregs() (*kvmSregs, error) {
	if !c.isa.sregsValid {
		if _, err := ioctl(c.fd, kvmGetSregs, uintptr(unsafe.Pointer(&c.isa.sregs))); err != nil {
			return nil, fail(Error, "KVM_GET_SREGS", err)
		}
		c.isa.sregsValid = true
	}
	return &c.isa.sregs, nil
}

func (s *kvmSegment) ar() uint64 {
	v := uint64(s.typ&15) | uint64(s.s&1)<<4 | uint64(s.dpl&3)<<5 | uint64(s.present&1)<<7 |
		uint64(s.avl&1)<<12 | uint64(s.l&1)<<13 | uint64(s.db&1)<<14 | uint64(s.g&1)<<15 | uint64(s.un&1)<<16
	return v
}

func (s *kvmSegment) setAR(v uint64) {
	s.typ, s.s, s.dpl, s.present = uint8(v&15), uint8(v>>4&1), uint8(v>>5&3), uint8(v>>7&1)
	s.avl, s.l, s.db, s.g, s.un = uint8(v>>12&1), uint8(v>>13&1), uint8(v>>14&1), uint8(v>>15&1), uint8(v>>16&1)
}

// sysReg is where r lives in struct kvm_sregs.
func sysReg(s *kvmSregs, r SysReg, set bool, val uint64) (uint64, bool) {
	if r >= segFirst && r < segFirst+8*4 {
		seg := &s.seg[(r-segFirst)/4]
		switch (r - segFirst) % 4 {
		case segSelector:
			if set {
				seg.selector = uint16(val)
			}
			return uint64(seg.selector), true
		case segBase:
			if set {
				seg.base = val
			}
			return seg.base, true
		case segLimit:
			if set {
				seg.limit = uint32(val)
			}
			return uint64(seg.limit), true
		default:
			if set {
				seg.setAR(val)
			}
			return seg.ar(), true
		}
	}
	var p *uint64
	switch r {
	case SysRegCR0:
		p = &s.cr0
	case SysRegCR2:
		p = &s.cr2
	case SysRegCR3:
		p = &s.cr3
	case SysRegCR4:
		p = &s.cr4
	case SysRegEFER:
		p = &s.efer
	case SysRegGDTRBase:
		p = &s.gdt.base
	case SysRegIDTRBase:
		p = &s.idt.base
	case SysRegGDTRLimit:
		if set {
			s.gdt.limit = uint16(val)
		}
		return uint64(s.gdt.limit), true
	case SysRegIDTRLimit:
		if set {
			s.idt.limit = uint16(val)
		}
		return uint64(s.idt.limit), true
	default:
		return 0, false
	}
	if set {
		*p = val
	}
	return *p, true
}

// VCPUGetSysReg is hv_vcpu_get_sys_reg.
func VCPUGetSysReg(v VCPU, r SysReg) (uint64, error) {
	c, err := get(v)
	if err != nil {
		return 0, err
	}
	s, err := c.sregs()
	if err != nil {
		return 0, err
	}
	val, ok := sysReg(s, r, false, 0)
	if !ok {
		return 0, BadArgument
	}
	return val, nil
}

// VCPUSetSysReg is hv_vcpu_set_sys_reg.
func VCPUSetSysReg(v VCPU, r SysReg, val uint64) error {
	c, err := get(v)
	if err != nil {
		return err
	}
	s, err := c.sregs()
	if err != nil {
		return err
	}
	if _, ok := sysReg(s, r, true, val); !ok {
		return BadArgument
	}
	c.isa.sregsDirt = true
	return nil
}

func (c *vcpu) put64(off int, v uint64) {
	*(*uint64)(unsafe.Pointer(&c.run[off])) = v
}
