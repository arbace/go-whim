// Command tamago is the Go guest (doc/GUEST.md, *A second guest*): the Go
// editor (editor/) as a virtual machine's only code, built with TamaGo for
// whim's monitor, its Host the C guest's hypercalls (guest/abi) through
// the board's doorbell.  go tool whim guest --go builds it and appends it
// to the monitor, as bin/whim-guest-go.
package main

import (
	"github.com/arbace/go-whim/editor"
	"github.com/arbace/go-whim/guest/abi"
	"github.com/arbace/go-whim/guest/tamago/board"
)

// host is editor.Host as hypercalls: each method one call, the monitor
// answering it on its own Host (editor/term) -- but for the read after a
// wait, which the wait's call made already (abi.WaitRead) and input keeps.
type host struct {
	deathtrap func(int32)
	input     input
}

// input is what the merged wait read ahead (doc/GUEST.md, *The merged wait
// and read*; guest/rt/rt.c's whim_input, in Go): as much as the core's one
// read asks at most, editor.INBUFLEN (in fill_input_buf: INBUFLEN less what
// its own inbuf holds, which is nothing after a wait in WaitForChar).  When
// kept, ret is what is left of the read's answer -- the count not yet the
// core's, from at, or the 0 or -1 it returned.
type input struct {
	buf  [editor.INBUFLEN]byte
	kept bool
	ret  int64
	at   int
}

// call makes hypercall nr.  A deadly signal the monitor caught during it
// comes back as the block's event: the editor's deathtrap runs here, where
// the C host's handler would have; when it returns (the signal blocked), a
// wait or a read is made again, as the C host's select and read go on
// after their EINTR -- guest/rt/rt.c's whim_hcall, in Go.
func (h *host) call(nr uint64, a0, a1, a2, a3, a4 int64) board.Block {
	for {
		b := board.Call(nr, a0, a1, a2, a3, a4)
		if b.Event != 0 && h.deathtrap != nil {
			h.deathtrap(int32(b.Event))
			if nr == abi.WaitForInput || nr == abi.ReadInput || nr == abi.WaitRead {
				continue
			}
		}
		return b
	}
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func (h *host) Init(deathtrap func(int32)) {
	h.deathtrap = deathtrap
	h.call(abi.HostInit, 0, 0, 0, 0, 0)
}

func (h *host) WinSize() (rows, cols int32, ok bool) {
	b := h.call(abi.GetWinsize, 0, 0, 0, 0, 0)
	return int32(b.A[0]), int32(b.A[1]), b.Ret != 0
}

func (h *host) TermStart() { h.call(abi.TermStart, 0, 0, 0, 0, 0) }

func (h *host) TermStop() { h.call(abi.TermStop, 0, 0, 0, 0, 0) }

func (h *host) TTYKeys(fd int32) (erase, intr int32, icrnl, onlcr, ok bool) {
	b := h.call(abi.TTYKeys, int64(fd), 0, 0, 0, 0)
	return int32(b.A[0]), int32(b.A[1]), b.A[2] != 0, b.A[3] != 0, b.Ret != 0
}

func (h *host) NowMs() int64 { return h.call(abi.NowMs, 0, 0, 0, 0, 0).Ret }

func (h *host) Time() int64 { return h.call(abi.Time, 0, 0, 0, 0, 0).Ret }

func (h *host) Delay(ms int64, interruptible bool) {
	h.call(abi.Delay, ms, b2i(interruptible), 0, 0, 0)
}

// WaitForInput is one call, which reads at once when there is input; while
// what it read is kept, the answer is yes with no call, as the C's select
// says yes to bytes in the tty.
func (h *host) WaitForInput(ms int64) bool {
	in := &h.input
	if in.kept {
		return true
	}
	b := h.call(abi.WaitRead, ms, board.Addr(in.buf[:]), int64(len(in.buf)), 0, 0)
	if b.Ret == 0 {
		return false
	}
	in.kept, in.ret, in.at = true, b.A[0], 0
	return true
}

// ReadInput is served from what the wait read, while any is kept: the
// bytes, as many as buf holds, or the read's 0 or -1.  Otherwise it is a
// call of its own, as before.
func (h *host) ReadInput(buf []byte) int32 {
	in := &h.input
	if !in.kept || len(buf) == 0 {
		return int32(h.call(abi.ReadInput, board.Addr(buf), int64(len(buf)), 0, 0, 0).Ret)
	}
	if in.ret <= 0 {
		in.kept = false
		return int32(in.ret)
	}
	n := copy(buf, in.buf[in.at:in.at+int(in.ret)])
	in.at += n
	in.ret -= int64(n)
	in.kept = in.ret > 0
	return int32(n)
}

func (h *host) Raise(sig int32) { h.call(abi.Raise, int64(sig), 0, 0, 0, 0) }

func (h *host) Suspend() { h.call(abi.Suspend, 0, 0, 0, 0, 0) }

func (h *host) Exit(code int32) { board.Exit(code) }

func (h *host) Message(msg []byte, err bool) {
	h.call(abi.Message, board.Addr(msg), int64(len(msg)), b2i(err), 0, 0)
}

func (h *host) Write(p []byte) int32 {
	return int32(h.call(abi.Write, board.Addr(p), int64(len(p)), 0, 0, 0).Ret)
}

func main() {
	board.Exit(int32(editor.Main(&host{}, board.Args())))
}
