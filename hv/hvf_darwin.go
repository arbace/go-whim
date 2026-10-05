//go:build darwin && arm64

// The macOS backend: Hypervisor.framework's arm64 C API called through
// purego (github.com/ebitengine/purego) -- the framework loaded by Dlopen,
// each hv_* function bound to a Go function of its C signature by
// RegisterLibFunc -- with no cgo, so `GOOS=darwin GOARCH=arm64
// CGO_ENABLED=0 go build` compiles it anywhere (doc/GUEST.md, *Decided*).
// Run on the M2 Max (milestone 4: doc/GUEST.md, *Running on the Mac*).  The binary
// that runs it needs the com.apple.security.hypervisor entitlement (an
// ad-hoc codesign with an entitlements plist).
//
// The mapping is one Go function for one C function; the framework does the
// rest -- PC left on a store that exits, the exit record in memory it owns
// (copied after each run into the one this package hands out), a vCPU bound
// to the thread that created it -- but for one thing KVM does and the
// framework is not documented to: KVM/arm64 keeps a VM's vCPUs on one
// virtual counter (CNTVOFF_EL2 the same for all), and here each vCPU has a
// vTimer offset of its own, so VCPUCreate gives every vCPU the VM's first's
// (TestCounterVCPUsARM64).  A guest on several vCPUs reads CNTVCT_EL0 on
// each and takes it for one clock: one vCPU's behind another's would run
// its time backwards, and the guest's nanotime, a difference from the boot
// vCPU's reading, would wrap.
package hv

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

const framework = "/System/Library/Frameworks/Hypervisor.framework/Hypervisor"

// cExit is hv_vcpu_exit_t: reason (hv_exit_reason_t, 32 bits), then
// hv_vcpu_exit_exception_t -- syndrome, virtual_address, physical_address,
// 64 bits each.
type cExit struct {
	reason   uint32
	_        uint32
	syndrome uint64
	virtual  uint64
	physical uint64
}

// The framework's functions, bound once.  hv_return_t is a 32-bit
// mach_error_t; hv_vcpu_t a uint64; the config objects pointers.
var (
	hvVMConfigCreate     func() uintptr
	hvVMConfigSetIPASize func(config uintptr, bits uint32) int32
	hvVMCreate           func(config uintptr) int32
	hvVMDestroy          func() int32
	hvVMMap              func(addr unsafe.Pointer, ipa uint64, size uintptr, flags uint64) int32
	hvVMUnmap            func(ipa uint64, size uintptr) int32
	hvVCPUCreate         func(vcpu *uint64, exit **cExit, config uintptr) int32
	hvVCPUDestroy        func(vcpu uint64) int32
	hvVCPURun            func(vcpu uint64) int32
	hvVCPUsExit          func(vcpus *uint64, count uint32) int32
	hvVCPUGetReg         func(vcpu uint64, reg uint32, value *uint64) int32
	hvVCPUSetReg         func(vcpu uint64, reg uint32, value uint64) int32
	hvVCPUGetSysReg      func(vcpu uint64, reg uint16, value *uint64) int32
	hvVCPUSetSysReg      func(vcpu uint64, reg uint16, value uint64) int32
	hvVCPUGetVTimerOff   func(vcpu uint64, offset *uint64) int32
	hvVCPUSetVTimerOff   func(vcpu uint64, offset uint64) int32

	bind    sync.Once
	bindErr error
	exitsMu sync.Mutex
	exits   = map[VCPU]vcpuExits{}
	// vmCounter is the VM's vTimer offset, its first vCPU's: every other
	// vCPU is given it (under exitsMu; VMCreate forgets it).
	vmCounter    uint64
	vmCounterSet bool
)

type vcpuExits struct {
	c   *cExit    // the framework's
	rec *VCPUExit // handed out
}

func load() error {
	bind.Do(func() {
		lib, err := purego.Dlopen(framework, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			bindErr = fmt.Errorf("%w: %s: %v", NoDevice, framework, err)
			return
		}
		for name, fn := range map[string]any{
			"hv_vm_config_create":       &hvVMConfigCreate,
			"hv_vm_config_set_ipa_size": &hvVMConfigSetIPASize,
			"hv_vm_create":              &hvVMCreate,
			"hv_vm_destroy":             &hvVMDestroy,
			"hv_vm_map":                 &hvVMMap,
			"hv_vm_unmap":               &hvVMUnmap,
			"hv_vcpu_create":            &hvVCPUCreate,
			"hv_vcpu_destroy":           &hvVCPUDestroy,
			"hv_vcpu_run":               &hvVCPURun,
			"hv_vcpus_exit":             &hvVCPUsExit,
			"hv_vcpu_get_reg":           &hvVCPUGetReg,
			"hv_vcpu_set_reg":           &hvVCPUSetReg,
			"hv_vcpu_get_sys_reg":       &hvVCPUGetSysReg,
			"hv_vcpu_set_sys_reg":       &hvVCPUSetSysReg,
			"hv_vcpu_get_vtimer_offset": &hvVCPUGetVTimerOff,
			"hv_vcpu_set_vtimer_offset": &hvVCPUSetVTimerOff,
		} {
			purego.RegisterLibFunc(fn, lib, name)
		}
	})
	return bindErr
}

func ret(r int32) error {
	if r == 0 {
		return nil
	}
	return Return(uint32(r))
}

// VMCreate is hv_vm_create, with hv_vm_config_set_ipa_size when asked (the
// config object is not released: one, for the process's one VM).
func VMCreate(config *VMConfig) error {
	if err := load(); err != nil {
		return err
	}
	exitsMu.Lock()
	vmCounterSet = false
	exitsMu.Unlock()
	c := uintptr(0)
	if config != nil && config.IPASize != 0 {
		c = hvVMConfigCreate()
		if err := ret(hvVMConfigSetIPASize(c, config.IPASize)); err != nil {
			return err
		}
	}
	return ret(hvVMCreate(c))
}

// VMDestroy is hv_vm_destroy.
func VMDestroy() error { return ret(hvVMDestroy()) }

// VMMap is hv_vm_map.  mem must be 16 KiB-aligned on Apple silicon.
func VMMap(mem []byte, ipa IPA, flags MemoryFlags) error {
	return ret(hvVMMap(unsafe.Pointer(&mem[0]), uint64(ipa), uintptr(len(mem)), uint64(flags)))
}

// VMUnmap is hv_vm_unmap.
func VMUnmap(ipa IPA, size uint64) error { return ret(hvVMUnmap(uint64(ipa), uintptr(size))) }

// VCPUCreate is hv_vcpu_create, on the calling thread (locked: LockThread),
// the vCPU put on the VM's virtual counter: the first's vTimer offset
// read, every later one's set to it (hv_vcpu_*_vtimer_offset, on the
// vCPU's own thread).
func VCPUCreate() (VCPU, *VCPUExit, error) {
	var v uint64
	var e *cExit
	if err := ret(hvVCPUCreate(&v, &e, 0)); err != nil {
		return 0, nil, err
	}
	exitsMu.Lock()
	defer exitsMu.Unlock()
	var err error
	if vmCounterSet {
		err = ret(hvVCPUSetVTimerOff(v, vmCounter))
	} else if err = ret(hvVCPUGetVTimerOff(v, &vmCounter)); err == nil {
		vmCounterSet = true
	}
	if err != nil {
		hvVCPUDestroy(v)
		return 0, nil, fmt.Errorf("the vCPU's vTimer offset: %w", err)
	}
	rec := &VCPUExit{}
	exits[VCPU(v)] = vcpuExits{e, rec}
	return VCPU(v), rec, nil
}

// VCPUDestroy is hv_vcpu_destroy.
func VCPUDestroy(v VCPU) error {
	exitsMu.Lock()
	delete(exits, v)
	exitsMu.Unlock()
	return ret(hvVCPUDestroy(uint64(v)))
}

// VCPURun is hv_vcpu_run, the framework's exit record copied into the one
// handed out.
func VCPURun(v VCPU) error {
	if err := ret(hvVCPURun(uint64(v))); err != nil {
		return err
	}
	exitsMu.Lock()
	e := exits[v]
	exitsMu.Unlock()
	*e.rec = VCPUExit{
		Reason: ExitReason(e.c.reason),
		Exception: VCPUExitException{
			Syndrome:        e.c.syndrome,
			VirtualAddress:  e.c.virtual,
			PhysicalAddress: IPA(e.c.physical),
		},
	}
	return nil
}

// VCPUsExit is hv_vcpus_exit.
func VCPUsExit(vcpus ...VCPU) error {
	if len(vcpus) == 0 {
		return nil
	}
	cv := make([]uint64, len(vcpus))
	for i, v := range vcpus {
		cv[i] = uint64(v)
	}
	return ret(hvVCPUsExit(&cv[0], uint32(len(cv))))
}

// VCPUGetReg is hv_vcpu_get_reg.
func VCPUGetReg(v VCPU, r Reg) (uint64, error) {
	var val uint64
	err := ret(hvVCPUGetReg(uint64(v), uint32(r), &val))
	return val, err
}

// VCPUSetReg is hv_vcpu_set_reg.
func VCPUSetReg(v VCPU, r Reg, val uint64) error {
	return ret(hvVCPUSetReg(uint64(v), uint32(r), val))
}

// VCPUGetSysReg is hv_vcpu_get_sys_reg.
func VCPUGetSysReg(v VCPU, r SysReg) (uint64, error) {
	var val uint64
	err := ret(hvVCPUGetSysReg(uint64(v), uint16(r), &val))
	return val, err
}

// VCPUSetSysReg is hv_vcpu_set_sys_reg.
func VCPUSetSysReg(v VCPU, r SysReg, val uint64) error {
	return ret(hvVCPUSetSysReg(uint64(v), uint16(r), val))
}

// Stats has no framework counterpart: every hv_vcpu_run is one exit.
func Stats(VCPU) (runs, spurious uint64) { return 0, 0 }

// LockThread: the framework requires a vCPU to live on the thread that
// created it, for its life.
func LockThread() { runtime.LockOSThread() }

// ForwardHVC has nothing to do on the framework: every HVC exits to the
// monitor.
func ForwardHVC(base, n uint32) error { return nil }
