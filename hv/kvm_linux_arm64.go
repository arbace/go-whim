package hv

import "unsafe"

// The arm64 half of the KVM backend: every register through
// KVM_GET/SET_ONE_REG, Apple's system-register encodings ORed into KVM's
// sysreg id (the same packing), the four that KVM keeps among the core
// registers (SP_EL0, SP_EL1, ELR_EL1, SPSR_EL1) mapped there.
//
// The framework leaves PC on a store that exits and the monitor advances it;
// KVM advances it itself, when the vCPU next runs.  So while an MMIO exit is
// pending, PC reads as the store's address, and a PC set is written 4 short
// of where it is to land (nothing written when it is the store's next
// instruction, the common case); a PC not set is put back on the store, so
// that it is made again, as on the framework.
const (
	kvmGetOneReg         = 0x4010aeab
	kvmSetOneReg         = 0x4010aeac
	kvmArmVCPUInit       = 0x4020aeae
	kvmArmPreferredTgt   = 0x8020aeaf
	kvmRegCore64         = 0x6030000000100000 // KVM_REG_ARM64 | KVM_REG_SIZE_U64 | KVM_REG_ARM_CORE
	kvmRegCore32         = 0x6020000000100000 // the same, 32 bits
	kvmRegSys            = 0x6030000000130000 // KVM_REG_ARM64 | KVM_REG_SIZE_U64 | KVM_REG_ARM64_SYSREG
	coreSP               = 248                // offsetof(struct kvm_regs, regs.sp): SP_EL0
	corePC               = 256
	corePSTATE           = 264
	coreSPEL1            = 272
	coreELREL1           = 280
	coreSPSR             = 288 // spsr[KVM_SPSR_EL1]
	coreFPSR             = 848 // fp_regs.fpsr
	coreFPCR             = 852
	kvmSMCCCFilterAttr   = 0 // KVM_ARM_VM_SMCCC_FILTER
	kvmSMCCCCtrlGroup    = 0 // KVM_ARM_VM_SMCCC_CTRL
	kvmSMCCCFilterFwd    = 2 // KVM_SMCCC_FILTER_FWD_TO_USER
	kvmHypercallExitSMC  = 1 << 0
	kvmVMTypeIPASizeMask = 0xff
)

type vmISA struct{}

type vcpuISA struct {
	mmio    bool   // an MMIO exit is pending completion
	faultPC uint64 // the store's address, when known
	pcKnown bool
	pcSet   bool // the monitor set PC while the MMIO was pending
	newPC   uint64
	x0      uint64 // X0 as read at the exit
}

func vmType(ipaBits uint32) uintptr { return uintptr(ipaBits & kvmVMTypeIPASizeMask) }

func vmInit() error { return nil }

type kvmVCPUInit struct {
	target   uint32
	features [7]uint32
}

func vcpuInit(c *vcpu) error {
	var init kvmVCPUInit
	if _, err := ioctlPtr(vm.fd, kvmArmPreferredTgt, unsafe.Pointer(&init)); err != nil {
		return fail(Unsupported, "KVM_ARM_PREFERRED_TARGET", err)
	}
	if _, err := ioctlPtr(c.fd, kvmArmVCPUInit, unsafe.Pointer(&init)); err != nil {
		return fail(Error, "KVM_ARM_VCPU_INIT", err)
	}
	return nil
}

type kvmOneReg struct {
	id, addr uint64
}

func (c *vcpu) getOne(id uint64) (uint64, error) {
	var v uint64
	r := kvmOneReg{id: id}
	if _, err := ioctlAt(c.fd, kvmGetOneReg, unsafe.Pointer(&r), &r.addr, unsafe.Pointer(&v)); err != nil {
		return 0, fail(BadArgument, "KVM_GET_ONE_REG", err)
	}
	return v, nil
}

func (c *vcpu) setOne(id, v uint64) error {
	r := kvmOneReg{id: id}
	if _, err := ioctlAt(c.fd, kvmSetOneReg, unsafe.Pointer(&r), &r.addr, unsafe.Pointer(&v)); err != nil {
		return fail(BadArgument, "KVM_SET_ONE_REG", err)
	}
	return nil
}

func core(off uint64) uint64 { return kvmRegCore64 | off/4 }

// regID is KVM's id for r.
func regID(r Reg) (uint64, bool) {
	switch {
	case r <= RegX30:
		return core(uint64(r) * 8), true
	case r == RegPC:
		return core(corePC), true
	case r == RegCPSR:
		return core(corePSTATE), true
	case r == RegFPSR:
		return kvmRegCore32 | coreFPSR/4, true
	case r == RegFPCR:
		return kvmRegCore32 | coreFPCR/4, true
	}
	return 0, false
}

// sysRegID is KVM's id for r: the core registers' for the four it keeps
// there, the encoding in KVM's sysreg space otherwise.
func sysRegID(r SysReg) uint64 {
	switch r {
	case SysRegSPEL0:
		return core(coreSP)
	case SysRegSPEL1:
		return core(coreSPEL1)
	case SysRegELREL1:
		return core(coreELREL1)
	case SysRegSPSREL1:
		return core(coreSPSR)
	}
	return kvmRegSys | uint64(r)
}

func (c *vcpu) storePC() (uint64, error) {
	if !c.isa.pcKnown {
		pc, err := c.getOne(core(corePC))
		if err != nil {
			return 0, err
		}
		c.isa.faultPC, c.isa.pcKnown = pc, true
	}
	return c.isa.faultPC, nil
}

func beforeRun(c *vcpu) error {
	if !c.isa.mmio {
		return nil
	}
	c.isa.mmio = false
	if c.isa.pcSet {
		pc, err := c.storePC()
		if err != nil {
			return err
		}
		if c.isa.newPC == pc+4 {
			return nil
		}
		return c.setOne(core(corePC), c.isa.newPC-4)
	}
	pc, err := c.storePC()
	if err != nil {
		return err
	}
	return c.setOne(core(corePC), pc-4)
}

func afterRun(c *vcpu, _ bool) {
	c.isa = vcpuISA{}
}

// mmioExit is a store's syndrome.  KVM does not say which register the
// store read; the guest's ABI stores from X0, which is read and checked
// against the value KVM reports: SRT 0 when it holds it, no valid
// instruction syndrome when not.
func mmioExit(c *vcpu, n int, write bool, data uint64) Syndrome {
	c.isa.mmio = true
	s := DataAbort(n, 0, write)
	x0, err := c.getOne(core(0))
	mask := ^uint64(0)
	if n < 8 {
		mask = 1<<(8*n) - 1
	}
	if err != nil || x0&mask != data {
		return s &^ (1 << 24)
	}
	c.isa.x0 = x0
	return s
}

// isaExit reports an HVC that KVM forwarded (KVM_ARM_VM_SMCCC_FILTER) as the
// framework does: an exception exit with EC 0x16, PC past the HVC.
func isaExit(c *vcpu, reason uint32) bool {
	if reason != kvmExitHypercall || c.u64(kvmHypercallFlags)&kvmHypercallExitSMC != 0 {
		return false
	}
	c.exit.Reason = ExitReasonException
	c.exit.Exception.Syndrome = uint64(ECHVC64)<<26 | 1<<25
	c.exit.Exception.PhysicalAddress = IPA(c.u64(kvmHypercallNr))
	return true
}

// VCPUGetReg is hv_vcpu_get_reg.
func VCPUGetReg(v VCPU, r Reg) (uint64, error) {
	c, err := get(v)
	if err != nil {
		return 0, err
	}
	if c.isa.mmio {
		switch r {
		case RegPC:
			if c.isa.pcSet {
				return c.isa.newPC, nil
			}
			return c.storePC()
		case RegX0:
			return c.isa.x0, nil
		}
	}
	id, ok := regID(r)
	if !ok {
		return 0, BadArgument
	}
	return c.getOne(id)
}

// VCPUSetReg is hv_vcpu_set_reg.
func VCPUSetReg(v VCPU, r Reg, val uint64) error {
	c, err := get(v)
	if err != nil {
		return err
	}
	if c.isa.mmio && r == RegPC {
		c.isa.pcSet, c.isa.newPC = true, val
		return nil
	}
	id, ok := regID(r)
	if !ok {
		return BadArgument
	}
	if c.isa.mmio && r == RegX0 {
		c.isa.x0 = val
	}
	return c.setOne(id, val)
}

// VCPUGetSysReg is hv_vcpu_get_sys_reg.
func VCPUGetSysReg(v VCPU, r SysReg) (uint64, error) {
	c, err := get(v)
	if err != nil {
		return 0, err
	}
	return c.getOne(sysRegID(r))
}

// VCPUSetSysReg is hv_vcpu_set_sys_reg.
func VCPUSetSysReg(v VCPU, r SysReg, val uint64) error {
	c, err := get(v)
	if err != nil {
		return err
	}
	return c.setOne(sysRegID(r), val)
}

type kvmDeviceAttr struct {
	flags uint32
	group uint32
	attr  uint64
	addr  uint64
}

type kvmSMCCCFilter struct {
	base, nr uint32
	action   uint8
	_        [15]uint8
}

// ForwardHVC has KVM hand the function IDs base..base+n-1 of an HVC or SMC
// to the monitor (KVM_ARM_VM_SMCCC_FILTER, Linux 6.4 on), reported as the
// framework reports an HVC.  Not the framework's: there every HVC exits.
// Called after VMCreate, before the first vCPU runs.
func ForwardHVC(base, n uint32) error {
	f := kvmSMCCCFilter{base: base, nr: n, action: kvmSMCCCFilterFwd}
	a := kvmDeviceAttr{group: kvmSMCCCCtrlGroup, attr: kvmSMCCCFilterAttr}
	if _, err := ioctlAt(vm.fd, kvmSetDeviceAttr, unsafe.Pointer(&a), &a.addr, unsafe.Pointer(&f)); err != nil {
		return fail(Unsupported, "KVM_ARM_VM_SMCCC_FILTER", err)
	}
	return nil
}
