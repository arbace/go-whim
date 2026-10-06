package vmm

import (
	"debug/elf"
	"encoding/binary"
	"fmt"
)

// The guest's memory, one slot from guest-physical 0, identity-mapped by
// page tables the monitor writes before the first run (doc/GUEST.md,
// *Memory*):
//
//	0x0000             not mapped: a null pointer faults
//	0x1000 .. 0x4000   the ISA's system tables (amd64: GDT, TSS, IDT)
//	0x4000 .. 0x10000  the command line: argv's pointers and strings
//	0x10000 .. 2 MiB   the page tables
//	2 MiB ..           the image: text read-only and executable, read-only
//	                   data, data and bss, each with its own permissions
//	then, 2 MiB on     a guard page, the stack (StackBytes), the fault stack
//	then, 2 MiB on     the heap: 1 GiB, the C host's arena, committed as used
//	Doorbell           no memory: a store here exits to the monitor
const (
	sysBase    = 0x1000
	argsBase   = 0x4000
	argsEnd    = 0x10000
	tablesBase = 0x10000
	tablesEnd  = 0x200000
	imageBase  = 0x200000
	page       = 0x1000
	block      = 0x200000 // 2 MiB: a leaf one level above the pages
	// Doorbell is the hypercall's address: the guest stores its call
	// block's address here.
	Doorbell = 0xf0000000
	// StackBytes is the guest's stack; FaultStackBytes the stack its
	// exception vectors run on.
	StackBytes      = 64 << 20
	FaultStackBytes = 64 << 10
	// HeapBytes is the guest's heap: the C host's arena, 1 GiB.
	HeapBytes = 1 << 30
)

// perm is what a guest page may be used for.
type perm uint8

const (
	pRead perm = 1 << iota
	pWrite
	pExec
	pDevice // the doorbell: no memory, device attributes
)

// span is a range of guest addresses with one permission.
type span struct {
	lo, hi uint64
	p      perm
}

// layout is where everything is, computed from the image.
type layout struct {
	entry, vectors   uint64
	spans            []span // sorted, disjoint
	stackTop         uint64
	faultTop         uint64
	heap             uint64
	size             uint64 // the slot's
	argc, argv       uint64
	tablesUsed, root uint64
	// a Go guest's (vmm/tamago.go): its RAM starts at its image; its
	// vCPUs, and where one CPUStart starts enters (whim_apentry)
	goGuest  bool
	ramStart uint64
	cpus     int
	apEntry  uint64
}

// faultTopOf is the top of vCPU i's fault stack: the boot vCPU's at
// faultTop, each other's FaultStackBytes under the one before.
func (l *layout) faultTopOf(i int) uint64 { return l.faultTop - uint64(i)*FaultStackBytes }

func roundUp(v, a uint64) uint64 { return (v + a - 1) &^ (a - 1) }

// image is a guest ELF's loadable segments and the two symbols the monitor
// needs: the entry and the exception vectors.
type image struct {
	f       *elf.File
	loads   []*elf.Prog
	entry   uint64
	vectors uint64
	apEntry uint64 // whim_apentry, a Go guest's when it has one
	end     uint64
}

func loadImage(f *elf.File, machine elf.Machine) (*image, error) {
	if f.Class != elf.ELFCLASS64 || f.Machine != machine {
		return nil, fmt.Errorf("the guest image is %v %v, not a 64-bit %v", f.Class, f.Machine, machine)
	}
	im := &image{f: f, entry: f.Entry}
	for _, p := range f.Progs {
		if p.Type != elf.PT_LOAD || p.Memsz == 0 {
			continue
		}
		if p.Vaddr < imageBase || p.Vaddr%page != 0 || p.Paddr != p.Vaddr {
			return nil, fmt.Errorf("the guest image's segment at %#x is not page-aligned at or above %#x", p.Vaddr, imageBase)
		}
		im.loads = append(im.loads, p)
		im.end = max(im.end, p.Vaddr+p.Memsz)
	}
	if len(im.loads) == 0 {
		return nil, fmt.Errorf("the guest image has nothing to load")
	}
	syms, err := f.Symbols()
	if err != nil {
		return nil, fmt.Errorf("the guest image's symbols: %w", err)
	}
	for _, s := range syms {
		switch s.Name {
		case "whim_vectors":
			im.vectors = s.Value
		case "whim_apentry":
			im.apEntry = s.Value
		}
	}
	if im.vectors == 0 {
		return nil, fmt.Errorf("the guest image has no whim_vectors")
	}
	return im, nil
}

// plan lays the slot out around the image, for cpus vCPUs: the stack, and
// above it a fault stack for each vCPU (IST1) -- a guest on several has
// whim_apentry, where CPUStart starts the others.
func plan(im *image, cpus int) *layout {
	l := &layout{entry: im.entry, vectors: im.vectors, cpus: cpus, apEntry: im.apEntry}
	l.spans = append(l.spans, span{sysBase, argsEnd, pRead | pWrite})
	for _, p := range im.loads {
		var pm perm
		if p.Flags&elf.PF_R != 0 {
			pm |= pRead
		}
		if p.Flags&elf.PF_W != 0 {
			pm |= pWrite
		}
		if p.Flags&elf.PF_X != 0 {
			pm |= pExec
		}
		l.spans = append(l.spans, span{p.Vaddr, roundUp(p.Vaddr+p.Memsz, page), pm})
	}
	guard := roundUp(im.end, block)
	stackLo := guard + page
	l.stackTop = stackLo + StackBytes
	l.faultTop = l.stackTop + uint64(cpus)*FaultStackBytes
	l.spans = append(l.spans, span{stackLo, l.faultTop, pRead | pWrite})
	l.heap = roundUp(l.faultTop, block)
	l.size = l.heap + HeapBytes
	l.spans = append(l.spans, span{l.heap, l.size, pRead | pWrite})
	l.spans = append(l.spans, span{Doorbell, Doorbell + page, pWrite | pDevice})
	return l
}

// permAt is the permission of the page at va: zero where nothing is mapped.
func (l *layout) permAt(va uint64) perm {
	for _, s := range l.spans {
		if va >= s.lo && va < s.hi {
			return s.p
		}
	}
	return 0
}

// load copies the image's segments into mem; bss is already zero.
func (im *image) load(mem []byte) error {
	for _, p := range im.loads {
		if p.Filesz == 0 {
			continue
		}
		if _, err := p.ReadAt(mem[p.Vaddr:p.Vaddr+p.Filesz], 0); err != nil {
			return fmt.Errorf("the guest image's segment at %#x: %w", p.Vaddr, err)
		}
	}
	return nil
}

// writeArgs puts argv into the command-line area: the pointers, then the
// strings, every pointer guest-physical.
func (l *layout) writeArgs(mem []byte, args []string) error {
	ptrs := uint64(argsBase)
	strs := ptrs + uint64(len(args)+1)*8
	for i, a := range args {
		if strs+uint64(len(a))+1 > argsEnd {
			return fmt.Errorf("the command line is longer than the guest's %d bytes for it", argsEnd-argsBase)
		}
		binary.LittleEndian.PutUint64(mem[ptrs+uint64(i)*8:], strs)
		copy(mem[strs:], a)
		mem[strs+uint64(len(a))] = 0
		strs += uint64(len(a)) + 1
	}
	binary.LittleEndian.PutUint64(mem[ptrs+uint64(len(args))*8:], 0)
	l.argc, l.argv = uint64(len(args)), ptrs
	return nil
}

// ptFormat is how an ISA writes its page-table entries, for 4 KiB pages
// with 2 MiB leaves one level up.
type ptFormat struct {
	levels int // tables from the root to the pages: 4 on amd64, 3 on arm64 (a 32-bit space)
	table  func(pa uint64) uint64
	leaf   func(pa uint64, p perm, big bool) uint64
}

// buildTables writes the identity map of l's spans into mem's table area,
// 2 MiB leaves where a block is one permission, pages where it is not, and
// returns the root's address.
func (l *layout) buildTables(mem []byte, f ptFormat) (uint64, error) {
	next := uint64(tablesBase)
	alloc := func() (uint64, error) {
		if next >= tablesEnd {
			return 0, fmt.Errorf("the page tables need more than %d KiB", (tablesEnd-tablesBase)>>10)
		}
		t := next
		next += page
		return t, nil
	}
	root, err := alloc()
	if err != nil {
		return 0, err
	}
	shift := func(level int) uint { return uint(12 + 9*(f.levels-1-level)) }
	// walk to the table that holds 2 MiB entries for va (level levels-2)
	walk := func(va uint64) (uint64, error) {
		t := root
		for lv := 0; lv < f.levels-2; lv++ {
			ix := va >> shift(lv) & 511
			e := binary.LittleEndian.Uint64(mem[t+ix*8:])
			if e == 0 {
				n, err := alloc()
				if err != nil {
					return 0, err
				}
				e = f.table(n)
				binary.LittleEndian.PutUint64(mem[t+ix*8:], e)
			}
			t = e & 0x0000fffffffff000
		}
		return t, nil
	}
	seen := map[uint64]bool{}
	for _, s := range l.spans {
		for b := s.lo &^ (block - 1); b < s.hi; b += block {
			if seen[b] {
				continue
			}
			seen[b] = true
			p0 := l.permAt(b)
			uniform := true
			for va := b; va < b+block; va += page {
				if l.permAt(va) != p0 {
					uniform = false
					break
				}
			}
			t, err := walk(b)
			if err != nil {
				return 0, err
			}
			ix := b >> 21 & 511
			if uniform {
				if p0 != 0 {
					binary.LittleEndian.PutUint64(mem[t+ix*8:], f.leaf(b, p0, true))
				}
				continue
			}
			pt, err := alloc()
			if err != nil {
				return 0, err
			}
			binary.LittleEndian.PutUint64(mem[t+ix*8:], f.table(pt))
			for va := b; va < b+block; va += page {
				if p := l.permAt(va); p != 0 {
					binary.LittleEndian.PutUint64(mem[pt+(va>>12&511)*8:], f.leaf(va, p, false))
				}
			}
		}
	}
	l.tablesUsed, l.root = next-tablesBase, root
	return root, nil
}
