package vmm

import (
	"debug/elf"
	"testing"

	"github.com/arbace/go-whim/guest/abi"
)

// TestABI holds the monitor's numbers to guest/abi's, which the Go guest
// is built on (guest/rt/rt.c spells the same in C).
func TestABI(t *testing.T) {
	for _, c := range [][2]int{
		{callHostInit, abi.HostInit}, {callGetWinsize, abi.GetWinsize}, {callTermStart, abi.TermStart},
		{callTermStop, abi.TermStop}, {callTTYKeys, abi.TTYKeys}, {callNowMs, abi.NowMs}, {callDelay, abi.Delay},
		{callWaitForInput, abi.WaitForInput}, {callReadInput, abi.ReadInput}, {callSuspend, abi.Suspend},
		{callExit, abi.Exit}, {callMessage, abi.Message}, {callAlloc, abi.Alloc}, {callFree, abi.Free},
		{callWrite, abi.Write}, {callTime, abi.Time}, {callRaise, abi.Raise}, {callFault, abi.Fault},
		{callRandom, abi.Random}, {callWaitRead, abi.WaitRead}, {nCalls, abi.NCalls},
		{Doorbell, abi.Doorbell}, {cbNr, abi.BlockNr}, {cbArgs, abi.BlockArgs}, {cbRet, abi.BlockRet},
		{cbEvent, abi.BlockEvent}, {cbSize, abi.BlockSize},
	} {
		if c[0] != c[1] {
			t.Errorf("the monitor's %d is the ABI's %d", c[0], c[1])
		}
	}
}

// TestPlanGo: a Go guest's slot is its image and one span read-write to
// GoRAMBytes above the image's base, the vectors' stack at the top.
func TestPlanGo(t *testing.T) {
	text := &elf.Prog{ProgHeader: elf.ProgHeader{Type: elf.PT_LOAD, Flags: elf.PF_R | elf.PF_X, Vaddr: imageBase, Memsz: 0x200000}}
	data := &elf.Prog{ProgHeader: elf.ProgHeader{Type: elf.PT_LOAD, Flags: elf.PF_R | elf.PF_W, Vaddr: imageBase + 0x200000, Memsz: 0x145678}}
	im := &image{entry: imageBase + 0x1000, vectors: imageBase + 0x2000, end: imageBase + 0x345678, loads: []*elf.Prog{text, data}}
	l := planGo(im)
	if l.ramStart != imageBase || l.size != imageBase+GoRAMBytes {
		t.Fatalf("ramStart %#x", l.ramStart)
	}
	if l.size-l.stackTop != FaultStackBytes || l.faultTop != l.size || l.heap != imageBase+0x346000 {
		t.Fatalf("layout %+v", l)
	}
	if l.permAt(l.heap) != pRead|pWrite || l.permAt(l.size-1) != pRead|pWrite || l.permAt(l.size) != 0 || l.permAt(imageBase) != pRead|pExec {
		t.Fatalf("spans %+v", l.spans)
	}
}
