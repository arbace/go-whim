//go:build darwin && arm64 && hvf

// The macOS backend: Hypervisor.framework through cgo, one Go function over
// each C function.  NOT BUILT AND NOT RUN: it is written so that the mapping
// from this package to the framework is explicit and can be reviewed before
// there is a Mac to build it on (doc/GUEST.md, milestone 4), and the build
// tag `hvf` keeps it out of every build until then.  The binary that links
// it needs the com.apple.security.hypervisor entitlement (an ad-hoc
// codesign with an entitlements plist).

package hv

/*
#cgo LDFLAGS: -framework Hypervisor
#include <Hypervisor/Hypervisor.h>

static hv_return_t vm_create_ipa(uint32_t ipa_bits) {
	hv_vm_config_t c = NULL;
	if (ipa_bits != 0) {
		c = hv_vm_config_create();
		hv_vm_config_set_ipa_size(c, ipa_bits);
	}
	hv_return_t r = hv_vm_create(c);
	if (c != NULL) os_release(c);
	return r;
}
*/
import "C"

import (
	"runtime"
	"unsafe"
)

func ret(r C.hv_return_t) error {
	if r == C.HV_SUCCESS {
		return nil
	}
	return Return(uint32(r))
}

// vcpus keeps each vCPU's exit record as the framework hands it out (memory
// it owns) beside the Go copy this package hands out.
var exits = map[VCPU]struct {
	c   *C.hv_vcpu_exit_t
	rec *VCPUExit
}{}

// VMCreate is hv_vm_create (with hv_vm_config_set_ipa_size when asked).
func VMCreate(config *VMConfig) error {
	bits := uint32(0)
	if config != nil {
		bits = config.IPASize
	}
	return ret(C.vm_create_ipa(C.uint32_t(bits)))
}

// VMDestroy is hv_vm_destroy.
func VMDestroy() error { return ret(C.hv_vm_destroy()) }

// VMMap is hv_vm_map.  mem must be 16 KiB-aligned on Apple silicon.
func VMMap(mem []byte, ipa IPA, flags MemoryFlags) error {
	return ret(C.hv_vm_map(unsafe.Pointer(&mem[0]), C.hv_ipa_t(ipa), C.size_t(len(mem)), C.hv_memory_flags_t(flags)))
}

// VMUnmap is hv_vm_unmap.
func VMUnmap(ipa IPA, size uint64) error {
	return ret(C.hv_vm_unmap(C.hv_ipa_t(ipa), C.size_t(size)))
}

// VCPUCreate is hv_vcpu_create, on the calling thread (locked).
func VCPUCreate() (VCPU, *VCPUExit, error) {
	var v C.hv_vcpu_t
	var e *C.hv_vcpu_exit_t
	if err := ret(C.hv_vcpu_create(&v, &e, nil)); err != nil {
		return 0, nil, err
	}
	g := &VCPUExit{}
	exits[VCPU(v)] = struct {
		c   *C.hv_vcpu_exit_t
		rec *VCPUExit
	}{e, g}
	return VCPU(v), g, nil
}

// VCPUDestroy is hv_vcpu_destroy.
func VCPUDestroy(v VCPU) error {
	delete(exits, v)
	return ret(C.hv_vcpu_destroy(C.hv_vcpu_t(v)))
}

// VCPURun is hv_vcpu_run, the framework's exit record copied into the one
// handed out.
func VCPURun(v VCPU) error {
	if err := ret(C.hv_vcpu_run(C.hv_vcpu_t(v))); err != nil {
		return err
	}
	e := exits[v]
	*e.rec = VCPUExit{
		Reason: ExitReason(e.c.reason),
		Exception: VCPUExitException{
			Syndrome:        uint64(e.c.exception.syndrome),
			VirtualAddress:  uint64(e.c.exception.virtual_address),
			PhysicalAddress: IPA(e.c.exception.physical_address),
		},
	}
	return nil
}

// VCPUsExit is hv_vcpus_exit.
func VCPUsExit(vcpus ...VCPU) error {
	cv := make([]C.hv_vcpu_t, len(vcpus))
	for i, v := range vcpus {
		cv[i] = C.hv_vcpu_t(v)
	}
	return ret(C.hv_vcpus_exit(&cv[0], C.uint32_t(len(cv))))
}

// VCPUGetReg is hv_vcpu_get_reg.
func VCPUGetReg(v VCPU, r Reg) (uint64, error) {
	var val C.uint64_t
	err := ret(C.hv_vcpu_get_reg(C.hv_vcpu_t(v), C.hv_reg_t(r), &val))
	return uint64(val), err
}

// VCPUSetReg is hv_vcpu_set_reg.
func VCPUSetReg(v VCPU, r Reg, val uint64) error {
	return ret(C.hv_vcpu_set_reg(C.hv_vcpu_t(v), C.hv_reg_t(r), C.uint64_t(val)))
}

// VCPUGetSysReg is hv_vcpu_get_sys_reg.
func VCPUGetSysReg(v VCPU, r SysReg) (uint64, error) {
	var val C.uint64_t
	err := ret(C.hv_vcpu_get_sys_reg(C.hv_vcpu_t(v), C.hv_sys_reg_t(r), &val))
	return uint64(val), err
}

// VCPUSetSysReg is hv_vcpu_set_sys_reg.
func VCPUSetSysReg(v VCPU, r SysReg, val uint64) error {
	return ret(C.hv_vcpu_set_sys_reg(C.hv_vcpu_t(v), C.hv_sys_reg_t(r), C.uint64_t(val)))
}

// Stats has no framework counterpart: every hv_vcpu_run is one exit.
func Stats(VCPU) (runs, spurious uint64) { return 0, 0 }

// LockThread: the framework requires a vCPU to live on the thread that
// created it.
func LockThread() { runtime.LockOSThread() }
