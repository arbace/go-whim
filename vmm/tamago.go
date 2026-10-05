package vmm

import (
	"crypto/rand"
	"debug/elf"
	"fmt"

	"github.com/arbace/go-whim/hv"
)

// A second guest (doc/GUEST.md, *A second guest*): the Go editor built with
// TamaGo, GOOS=tamago, on a board of our own (guest/tamago/board).  The
// monitor tells its image from the C guest's by the runtime's own symbol,
// and lays its memory out as a Go runtime wants it: no separate stack and
// heap, but one span the runtime manages itself -- its heap grows up from
// the image's end (sbrk), its first stack sits under the top -- and the
// exception vectors' stack above that.  Every other part is the C guest's:
// the identity map, the system tables, the doorbell, the exit loop and
// the calls.

// GoRAMBytes is a Go guest's RAM from its image to the top: the Go
// editor's 1 GiB arena is its own accounting (editor/host.go), and the
// collector wants room beside the live heap.  Committed as touched; under
// the doorbell.
const GoRAMBytes = 3 << 30

// tamagoSymbol is how a TamaGo image is known: the variable its board sets
// before the runtime starts.
const tamagoSymbol = "runtime/goos.RamStart"

func isTamaGo(f *elf.File) bool {
	syms, err := f.Symbols()
	if err != nil {
		return false
	}
	for _, s := range syms {
		if s.Name == tamagoSymbol {
			return true
		}
	}
	return false
}

// layoutFor is the slot's layout for the image: the C guest's (plan), one
// vCPU, or a Go guest's (planGo) on cpus.
func layoutFor(f *elf.File, im *image, cpus int) *layout {
	if isTamaGo(f) {
		return planGo(im, cpus)
	}
	return plan(im, cpus)
}

// planGo lays a Go guest's slot out: the image, then one span read-write
// to GoRAMBytes above the image's base, its top FaultStackBytes for each
// vCPU the vectors' stacks (IST1) and below them the runtime's first stack.
func planGo(im *image, cpus int) *layout {
	l := &layout{entry: im.entry, vectors: im.vectors, goGuest: true, cpus: cpus, apEntry: im.apEntry}
	l.spans = append(l.spans, span{sysBase, argsEnd, pRead | pWrite})
	base := uint64(Doorbell)
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
		base = min(base, p.Vaddr)
	}
	l.ramStart = base
	l.heap = roundUp(im.end, page)
	l.size = base + GoRAMBytes
	l.faultTop = l.size
	l.stackTop = l.size - uint64(cpus)*FaultStackBytes
	l.spans = append(l.spans, span{l.heap, l.size, pRead | pWrite})
	l.spans = append(l.spans, span{Doorbell, Doorbell + page, pWrite | pDevice})
	return l
}

// boot sets the vCPU up for the image: the C guest's state (setup), and a
// Go guest's registers besides (abi's entry state).
func boot(v hv.VCPU, mem []byte, l *layout) error {
	if l.size > Doorbell {
		return fmt.Errorf("the guest's %d MiB reach the doorbell at %#x", l.size>>20, Doorbell)
	}
	if err := setup(v, mem, l); err != nil {
		return err
	}
	if l.goGuest {
		return setupGo(v, l)
	}
	return nil
}

// random answers abi.Random: buf filled from the host's generator.
func random(buf []byte) int64 {
	if _, err := rand.Read(buf); err != nil {
		return -1
	}
	return int64(len(buf))
}
