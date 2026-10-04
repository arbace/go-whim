package hv

import (
	"encoding/binary"
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"
)

// newTinyGuest is a VM of 64 KiB with code at 0x1000, run at EL1 with the
// MMU off: the smallest guest that makes the exits the monitor decodes.
// On the Mac it runs on Hypervisor.framework: the test binary signed with
// the hypervisor entitlement (guest/mac/mac.sh hv), the first thing to run
// there; 64 KiB is four of its 16 KiB pages.
func newTinyGuest(t *testing.T, code []uint32) (VCPU, *VCPUExit) {
	t.Helper()
	if _, err := os.Stat("/dev/kvm"); err != nil && runtime.GOOS == "linux" {
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
	for i, w := range code {
		binary.LittleEndian.PutUint32(mem[0x1000+4*i:], w)
	}
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
	if err := VCPUSetReg(v, RegCPSR, 0x3c5); err != nil { // EL1h, DAIF masked
		t.Fatal(err)
	}
	if err := VCPUSetReg(v, RegPC, 0x1000); err != nil {
		t.Fatal(err)
	}
	return v, exit
}

// TestDoorbellARM64: a store where no memory is exits as the framework's
// data abort, PC on the store; the monitor advances PC as it must on the
// framework, and the next store exits in its turn.
func TestDoorbellARM64(t *testing.T) {
	v, exit := newTinyGuest(t, []uint32{
		0xd28acf00, // movz x0, #0x5678
		0xf2a24680, // movk x0, #0x1234, lsl #16
		0xd2a00021, // movz x1, #0x10000
		0xf9000020, // str x0, [x1]
		0xf9000420, // str x0, [x1, #8]
		0x14000000, // b .
	})
	for i, want := range []IPA{0x10000, 0x10008} {
		if err := VCPURun(v); err != nil {
			t.Fatal(err)
		}
		s := Syndrome(exit.Exception.Syndrome)
		if exit.Reason != ExitReasonException || s.EC() != ECDataAbortLower || !s.ISV() || !s.WnR() || s.SAS() != 3 {
			t.Fatalf("store %d: exit %v syndrome %#x, not an 8-byte data abort", i, exit.Reason, exit.Exception.Syndrome)
		}
		if exit.Exception.PhysicalAddress != want {
			t.Fatalf("store %d at %#x, not %#x", i, exit.Exception.PhysicalAddress, want)
		}
		r, _ := RegForSRT(s.SRT())
		if val, _ := VCPUGetReg(v, r); val != 0x12345678 {
			t.Fatalf("store %d: the register holds %#x", i, val)
		}
		pc, _ := VCPUGetReg(v, RegPC)
		if pc != 0x100c+4*uint64(i) {
			t.Fatalf("store %d: PC %#x, not on the store", i, pc)
		}
		if err := VCPUSetReg(v, RegPC, pc+4); err != nil {
			t.Fatal(err)
		}
	}
	go func() {
		time.Sleep(200 * time.Millisecond)
		VCPUsExit(v)
	}()
	if err := VCPURun(v); err != nil {
		t.Fatal(err)
	}
	if exit.Reason != ExitReasonCanceled {
		t.Fatalf("exit %v, not canceled", exit.Reason)
	}
	if pc, _ := VCPUGetReg(v, RegPC); pc != 0x1014 {
		t.Fatalf("PC %#x, not on the spin", pc)
	}
}

// TestStoreAgainARM64: a store whose exit the monitor does not advance is
// made again, as on the framework.
func TestStoreAgainARM64(t *testing.T) {
	v, exit := newTinyGuest(t, []uint32{0xd2a00021, 0xf9000020, 0x14000000}) // x1 = 0x10000; str x0, [x1]; b .
	for range 2 {
		if err := VCPURun(v); err != nil {
			t.Fatal(err)
		}
		if exit.Reason != ExitReasonException || exit.Exception.PhysicalAddress != 0x10000 {
			t.Fatalf("exit %v at %#x", exit.Reason, exit.Exception.PhysicalAddress)
		}
	}
}

// TestSysRegsARM64: Apple's encodings reach KVM's registers, the four KVM
// keeps among the core ones included.
func TestSysRegsARM64(t *testing.T) {
	v, _ := newTinyGuest(t, []uint32{0x14000000})
	for _, r := range []SysReg{SysRegTTBR0EL1, SysRegMAIREL1, SysRegVBAREL1, SysRegSPEL0, SysRegSPEL1, SysRegELREL1} {
		if err := VCPUSetSysReg(v, r, 0x8000); err != nil {
			t.Fatalf("%#x: %v", r, err)
		}
		if val, err := VCPUGetSysReg(v, r); err != nil || val != 0x8000 {
			t.Fatalf("%#x: %#x %v", r, val, err)
		}
	}
}
