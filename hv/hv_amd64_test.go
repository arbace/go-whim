package hv

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// newTinyGuest is a VM of 64 KiB with code at 0x1000 run in real mode, CS
// and DS based so that DS:0 is 0x10000, where no memory is: the smallest
// guest that makes the exits the monitor decodes.
func newTinyGuest(t *testing.T, code []byte) (VCPU, *VCPUExit) {
	t.Helper()
	if _, err := os.Stat("/dev/kvm"); err != nil {
		t.Skip("no /dev/kvm")
	}
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	if err := VMCreate(nil); err != nil {
		t.Fatal(err)
	}
	mem, err := syscall.Mmap(-1, 0, 0x10000, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_PRIVATE|syscall.MAP_ANON)
	if err != nil {
		t.Fatal(err)
	}
	copy(mem[0x1000:], code)
	if err := VMMap(mem, 0, MemoryRead|MemoryWrite|MemoryExec); err != nil {
		t.Fatal(err)
	}
	v, exit, err := VCPUCreate()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := VCPUDestroy(v); err != nil {
			t.Error(err)
		}
		if err := VMDestroy(); err != nil {
			t.Error(err)
		}
		syscall.Munmap(mem)
	})
	for _, s := range []struct {
		r   SysReg
		val uint64
	}{{SegCS.Selector(), 0}, {SegCS.Base(), 0}, {SegDS.Selector(), 0x1000}, {SegDS.Base(), 0x10000}} {
		if err := VCPUSetSysReg(v, s.r, s.val); err != nil {
			t.Fatal(err)
		}
	}
	if err := VCPUSetReg(v, RegRIP, 0x1000); err != nil {
		t.Fatal(err)
	}
	if err := VCPUSetReg(v, RegRFLAGS, 2); err != nil {
		t.Fatal(err)
	}
	return v, exit
}

// TestDoorbellAMD64: a store where no memory is exits as the framework's
// data abort -- the address, the size, the register -- with RIP past the
// store, and the guest goes on.
func TestDoorbellAMD64(t *testing.T) {
	v, exit := newTinyGuest(t, []byte{
		0x66, 0xb8, 0x78, 0x56, 0x34, 0x12, // mov eax, 0x12345678
		0x66, 0xa3, 0x00, 0x00, // mov [0], eax: DS:0 = 0x10000
		0xf4, // hlt
	})
	if err := VCPURun(v); err != nil {
		t.Fatal(err)
	}
	s := Syndrome(exit.Exception.Syndrome)
	if exit.Reason != ExitReasonException || s.EC() != ECDataAbortLower || !s.ISV() || !s.WnR() || s.SAS() != 2 {
		t.Fatalf("exit %v syndrome %#x, not a 4-byte data abort", exit.Reason, exit.Exception.Syndrome)
	}
	if exit.Exception.PhysicalAddress != 0x10000 {
		t.Fatalf("the abort at %#x, not 0x10000", exit.Exception.PhysicalAddress)
	}
	r, ok := RegForSRT(s.SRT())
	if !ok {
		t.Fatalf("SRT %d", s.SRT())
	}
	if val, _ := VCPUGetReg(v, r); val != 0x12345678 {
		t.Fatalf("the stored register holds %#x", val)
	}
	if rip, _ := VCPUGetReg(v, RegRIP); rip != 0x100a {
		t.Fatalf("RIP %#x after the store, not 0x100a", rip)
	}
	if err := VCPURun(v); err != nil {
		t.Fatal(err)
	}
	if exit.Reason != ExitReasonUnknown || !strings.Contains(exit.Detail, "HLT") {
		t.Fatalf("exit %v %q, not the halt", exit.Reason, exit.Detail)
	}
	if runs, _ := Stats(v); runs != 2 {
		t.Fatalf("%d KVM_RUNs for two exits", runs)
	}
}

// TestVCPUsExitAMD64: a guest spinning is stopped from another goroutine,
// as hv_vcpus_exit does.
func TestVCPUsExitAMD64(t *testing.T) {
	v, exit := newTinyGuest(t, []byte{0xeb, 0xfe}) // jmp .
	go func() {
		time.Sleep(50 * time.Millisecond)
		VCPUsExit(v)
	}()
	if err := VCPURun(v); err != nil {
		t.Fatal(err)
	}
	if exit.Reason != ExitReasonCanceled {
		t.Fatalf("exit %v, not canceled", exit.Reason)
	}
	if rip, _ := VCPUGetReg(v, RegRIP); rip != 0x1000 {
		t.Fatalf("RIP %#x", rip)
	}
}

// TestSysRegsAMD64: a segment's access rights, written as VMX encodes them,
// read back the same.
func TestSysRegsAMD64(t *testing.T) {
	v, _ := newTinyGuest(t, []byte{0xf4})
	if err := VCPUSetSysReg(v, SegSS.AR(), 0xc093); err != nil {
		t.Fatal(err)
	}
	if ar, _ := VCPUGetSysReg(v, SegSS.AR()); ar != 0xc093 {
		t.Fatalf("AR %#x", ar)
	}
	if _, err := VCPUGetSysReg(v, 0x7777); err != BadArgument {
		t.Fatalf("an unknown register: %v", err)
	}
	if unsafe.Sizeof(kvmSregs{}) != 312 || unsafe.Sizeof(kvmSegment{}) != 24 {
		t.Fatalf("struct kvm_sregs is %d bytes here, kvm_segment %d", unsafe.Sizeof(kvmSregs{}), unsafe.Sizeof(kvmSegment{}))
	}
}

// TestVCPUsAMD64: a second vCPU created and run on a thread of its own
// beside the first (doc/GUEST.md, *SMP*), both spinning, both stopped by
// one VCPUsExit; run from a thread not its own it is refused, and once
// destroyed VCPUsExit names it no more.
func TestVCPUsAMD64(t *testing.T) {
	v0, exit0 := newTinyGuest(t, []byte{0xeb, 0xfe}) // jmp .
	created := make(chan VCPU)
	ran := make(chan error)
	destroy := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		v, exit, err := VCPUCreate()
		if err != nil {
			ran <- err
			return
		}
		for _, s := range []struct {
			r   SysReg
			val uint64
		}{{SegCS.Selector(), 0}, {SegCS.Base(), 0}} {
			VCPUSetSysReg(v, s.r, s.val)
		}
		VCPUSetReg(v, RegRIP, 0x1000)
		VCPUSetReg(v, RegRFLAGS, 2)
		created <- v
		err = VCPURun(v)
		if err == nil && exit.Reason != ExitReasonCanceled {
			err = fmt.Errorf("vCPU 1: exit %v, not canceled", exit.Reason)
		}
		ran <- err
		<-destroy
		ran <- VCPUDestroy(v)
	}()
	var v1 VCPU
	select {
	case v1 = <-created:
	case err := <-ran:
		t.Fatal(err)
	}
	if v1 == v0 {
		t.Fatalf("both vCPUs are %d", v0)
	}
	if err := VCPURun(v1); err == nil {
		t.Fatal("vCPU 1 ran on vCPU 0's thread")
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		VCPUsExit(v0, v1)
	}()
	if err := VCPURun(v0); err != nil || exit0.Reason != ExitReasonCanceled {
		t.Fatalf("vCPU 0: %v, exit %v", err, exit0.Reason)
	}
	if err := <-ran; err != nil {
		t.Fatal(err)
	}
	close(destroy)
	if err := <-ran; err != nil {
		t.Fatal(err)
	}
	if err := VCPUsExit(v1); err != BadArgument {
		t.Fatalf("VCPUsExit of a destroyed vCPU: %v", err)
	}
}
