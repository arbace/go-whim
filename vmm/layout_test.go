package vmm

import (
	"debug/elf"
	"os"
	"syscall"
	"testing"
	"unsafe"
)

// hvfPage is Apple silicon's page: hv_vm_map wants the host address, the
// IPA and the size all multiples of it.
const hvfPage = 16 << 10

// TestLayoutHVFAlignment holds the layout to what Hypervisor.framework asks
// of a mapping on the M2 Max (doc/GUEST.md, *Running on the Mac*): the
// monitor maps one slot, guest-physical [0, size), with one hv_vm_map, so
// its IPA and size must be multiples of 16 KiB, whatever the image's size;
// the doorbell page must lie outside it (it exits because nothing is mapped
// there) and below the framework's default 36-bit IPA space.  The guest's
// own tables still use 4 KiB pages inside the slot, which the framework
// does not see.
func TestLayoutHVFAlignment(t *testing.T) {
	for _, end := range []uint64{imageBase + page, imageBase + 3*page + 17, 5<<20 + 12345, 9<<20 - 1, 64 << 20} {
		im := &image{entry: imageBase, vectors: imageBase, end: end, loads: []*elf.Prog{
			{ProgHeader: elf.ProgHeader{Type: elf.PT_LOAD, Flags: elf.PF_R | elf.PF_X, Vaddr: imageBase, Paddr: imageBase, Memsz: end - imageBase}},
		}}
		l := plan(im)
		const ipa = 0 // newMachine's one hv_vm_map
		if ipa%hvfPage != 0 || l.size%hvfPage != 0 {
			t.Errorf("image to %#x: the slot [%#x, %#x) is not 16 KiB-aligned", end, ipa, l.size)
		}
		if l.size > Doorbell || Doorbell%hvfPage != 0 {
			t.Errorf("image to %#x: the slot ends at %#x, the doorbell at %#x: not outside it on a 16 KiB page", end, l.size, Doorbell)
		}
		if Doorbell+page > 1<<36 {
			t.Errorf("the doorbell %#x is beyond a 36-bit IPA space", Doorbell)
		}
		for _, s := range l.spans {
			if s.p&pDevice == 0 && s.hi > l.size {
				t.Errorf("image to %#x: span [%#x, %#x) is past the slot's end %#x", end, s.lo, s.hi, l.size)
			}
		}
	}
}

// TestSlotHostAlignment maps a slot as newMachine does and holds its host
// address to the machine's page and to 16 KiB on a machine whose page that
// is (Apple silicon): where the test runs on the Mac, it checks hv_vm_map's
// host-side condition for real.
func TestSlotHostAlignment(t *testing.T) {
	const size = 64 << 20
	mem, err := syscall.Mmap(-1, 0, size, syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_PRIVATE|syscall.MAP_ANON|syscall.MAP_NORESERVE)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Munmap(mem)
	addr, pg := uintptr(unsafe.Pointer(&mem[0])), uintptr(os.Getpagesize())
	if addr%pg != 0 {
		t.Fatalf("the slot's host address %#x is not on a %d-byte page", addr, pg)
	}
	if pg >= hvfPage && addr%hvfPage != 0 {
		t.Fatalf("the slot's host address %#x is not 16 KiB-aligned", addr)
	}
}
