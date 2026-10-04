// Package vmm is the monitor that runs whim's core as a virtual machine's
// only code (doc/GUEST.md): the guest image loaded into one slot of RAM, the
// vCPU put directly into the mode the core runs in, and an exit loop that
// answers each hypercall -- a store to the doorbell, decoded in
// Hypervisor.framework's form (package hv) -- by calling the same
// editor.Host the Go editor runs on.
package vmm

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/arbace/go-whim/editor"
	"github.com/arbace/go-whim/hv"
)

// The hypercalls: the 17 host functions in doc/GUEST.md's order, and the
// runtime's report of a fault.  13 and 14, host_alloc and host_free, are
// answered in the guest and never made.
const (
	callHostInit = iota + 1
	callGetWinsize
	callTermStart
	callTermStop
	callTTYKeys
	callNowMs
	callDelay
	callWaitForInput
	callReadInput
	callSuspend
	callExit
	callMessage
	callAlloc
	callFree
	callWrite
	callTime
	callRaise
	callFault
	callRandom // a Go guest's (vmm/tamago.go)
	nCalls
)

var callNames = [nCalls]string{"", "host_init", "get_winsize", "term_start", "term_stop", "tty_keys",
	"now_ms", "delay", "wait_for_input", "read_input", "suspend", "exit", "message", "alloc", "free",
	"write", "time", "raise", "fault", "random"}

// The call block, as guest/rt.c lays it out: nr, a[5], ret, event.
const (
	cbNr    = 0
	cbArgs  = 8
	cbRet   = 48
	cbEvent = 56
	cbSize  = 64
)

// Config is a run: the guest's ELF image, the host it asks, its command
// line, and the monitor's own options.
type Config struct {
	Image []byte
	Host  editor.Host
	Args  []string
	// Watchdog, when not zero, ends a guest that runs that long without a
	// hypercall.
	Watchdog time.Duration
	// Stats, when not nil, is written the run's counts when it ends.
	Stats io.Writer
	// Seccomp, when set, is called on the vCPU's thread once the VM is
	// built and before the guest first runs: the filter goes on there.
	Seccomp func() error
}

// Stats is what a run counted.
type Stats struct {
	Calls    [nCalls]uint64
	Exits    uint64 // VCPURun's returns
	Runs     uint64 // KVM_RUN calls (the backend's)
	Spurious uint64 // of those, interrupted by a signal and run again
	Guest    time.Duration
	Longest  time.Duration // the longest run between two exits
	Host     time.Duration
}

// deadly is the panic the host's deathtrap callback unwinds with: a deadly
// signal, carried back to the guest in the call's event.
type deadly int32

// Fault is a run's end by a fault the guest took, or an exit the monitor
// cannot answer: what the C editor would have died of a signal for.
type Fault struct{ Msg string }

func (f *Fault) Error() string { return f.Msg }

// Run runs the guest in cfg on the calling goroutine, which it locks to its
// thread, and returns the guest's exit code.  A Host whose Exit does not
// return (the terminal's) ends the process there; one that panics with
// editor.Exit has its code returned, as editor.Main does.
func Run(cfg Config) (code int, err error) {
	m, err := newMachine(cfg)
	if err != nil {
		return 1, err
	}
	defer m.close()
	defer func() {
		if r := recover(); r != nil {
			c, ok := r.(editor.Exit)
			if !ok {
				panic(r)
			}
			code, err = int(c), nil
		}
	}()
	return m.loop()
}

type machine struct {
	cfg      Config
	mem      []byte
	l        *layout
	v        hv.VCPU
	exit     *hv.VCPUExit
	stats    Stats
	lastCall atomic.Int64 // when the guest last made a call, for the watchdog (ns)
	inHost   atomic.Bool
	killed   atomic.Bool
	stop     chan struct{}
}

func newMachine(cfg Config) (*machine, error) {
	f, err := elf.NewFile(bytes.NewReader(cfg.Image))
	if err != nil {
		return nil, fmt.Errorf("the guest image: %w", err)
	}
	im, err := loadImage(f, elfMachine)
	if err != nil {
		return nil, err
	}
	m := &machine{cfg: cfg, l: layoutFor(f, im), stop: make(chan struct{})}
	m.mem, err = syscall.Mmap(-1, 0, int(m.l.size), syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_PRIVATE|syscall.MAP_ANON|syscall.MAP_NORESERVE)
	if err != nil {
		return nil, fmt.Errorf("the guest's %d MiB: %w", m.l.size>>20, err)
	}
	if err := im.load(m.mem); err != nil {
		m.close()
		return nil, err
	}
	if err := m.l.writeArgs(m.mem, cfg.Args); err != nil {
		m.close()
		return nil, err
	}
	hv.LockThread()
	if err := hv.VMCreate(nil); err != nil {
		m.close()
		return nil, err
	}
	vmSetup()
	if err := hv.VMMap(m.mem, 0, hv.MemoryRead|hv.MemoryWrite|hv.MemoryExec); err != nil {
		hv.VMDestroy()
		m.close()
		return nil, err
	}
	m.v, m.exit, err = hv.VCPUCreate()
	if err != nil {
		hv.VMDestroy()
		m.close()
		return nil, err
	}
	if err := boot(m.v, m.mem, m.l); err != nil {
		m.destroy()
		return nil, err
	}
	return m, nil
}

func (m *machine) destroy() {
	hv.VCPUDestroy(m.v)
	hv.VMDestroy()
	m.close()
}

func (m *machine) close() {
	if m.mem != nil {
		syscall.Munmap(m.mem)
		m.mem = nil
	}
}

// loop runs the vCPU until the guest exits, answering its calls.
func (m *machine) loop() (int, error) {
	defer m.destroy()
	defer close(m.stop)
	if m.cfg.Seccomp != nil {
		if err := m.cfg.Seccomp(); err != nil {
			return 1, fmt.Errorf("seccomp: %w", err)
		}
	}
	if m.cfg.Watchdog > 0 {
		go m.watchdog()
	}
	m.lastCall.Store(time.Now().UnixNano())
	for {
		t0 := time.Now()
		if err := hv.VCPURun(m.v); err != nil {
			return 1, err
		}
		t1 := time.Now()
		m.stats.Guest += t1.Sub(t0)
		m.stats.Longest = max(m.stats.Longest, t1.Sub(t0))
		m.stats.Exits++
		switch m.exit.Reason {
		case hv.ExitReasonException:
			code, done, err := m.exception()
			m.stats.Host += time.Since(t1)
			if done || err != nil {
				return code, err
			}
		case hv.ExitReasonCanceled:
			if m.killed.Load() {
				return 1, m.fault(fmt.Sprintf("the guest ran %s without a hypercall: ended by the watchdog", m.cfg.Watchdog))
			}
		default:
			return 1, m.fault(fmt.Sprintf("the guest stopped: %s %s", m.exit.Reason, m.exit.Detail))
		}
	}
}

// exception answers an exception exit: a store to the doorbell is a call.
func (m *machine) exception() (int, bool, error) {
	e := m.exit.Exception
	s := hv.Syndrome(e.Syndrome)
	addr := uint64(e.PhysicalAddress)
	reg, alt := altCall(m.v, s, addr)
	if !alt {
		if s.EC() != hv.ECDataAbortLower || addr != Doorbell || !s.ISV() || !s.WnR() {
			return 1, false, m.fault(fmt.Sprintf("the guest touched %#x with no memory there (syndrome %#x, pc %#x)", addr, e.Syndrome, pc(m.v)))
		}
		var ok bool
		if reg, ok = hv.RegForSRT(s.SRT()); !ok {
			return 1, false, m.fault(fmt.Sprintf("a doorbell store from register %d", s.SRT()))
		}
	}
	cb, err := hv.VCPUGetReg(m.v, reg)
	if err != nil {
		return 1, false, err
	}
	if err := advance(m.v, s); err != nil {
		return 1, false, err
	}
	if cb%cbSize != 0 || cb < imageBase || cb+cbSize > m.l.size {
		return 1, false, m.fault(fmt.Sprintf("a call block at %#x, outside the guest's memory", cb))
	}
	m.lastCall.Store(time.Now().UnixNano())
	m.inHost.Store(true)
	defer m.inHost.Store(false)
	return m.call(m.mem[cb : cb+cbSize])
}

// fault is the end of a guest that did what it must not: the diagnostic,
// then the C editor's death of a SIGSEGV for the process.
func (m *machine) fault(msg string) error {
	return &Fault{"whim-guest: " + msg}
}

// guest is a slice of guest memory a call names, or false when it is not
// all in the slot.
func (m *machine) guest(gpa, n int64) ([]byte, bool) {
	if n < 0 || gpa < 0 || uint64(gpa) > m.l.size || uint64(n) > m.l.size-uint64(gpa) {
		return nil, false
	}
	return m.mem[gpa : gpa+n], true
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// call answers the call in block cb, on m.cfg.Host.
func (m *machine) call(cb []byte) (code int, done bool, err error) {
	le := binary.LittleEndian
	nr := le.Uint64(cb[cbNr:])
	var a [5]int64
	for i := range a {
		a[i] = int64(le.Uint64(cb[cbArgs+8*i:]))
	}
	if nr == 0 || nr >= nCalls || nr == callAlloc || nr == callFree {
		return 1, false, m.fault(fmt.Sprintf("an unknown call %d", nr))
	}
	m.stats.Calls[nr]++
	h := m.cfg.Host
	ret := int64(0)
	defer func() {
		if r := recover(); r != nil {
			sig, ok := r.(deadly)
			if !ok {
				panic(r)
			}
			le.PutUint64(cb[cbEvent:], uint64(sig))
		}
	}()
	switch nr {
	case callHostInit:
		h.Init(func(sig int32) { panic(deadly(sig)) })
	case callGetWinsize:
		r, c, ok := h.WinSize()
		ret, a[0], a[1] = b2i(ok), int64(r), int64(c)
	case callTermStart:
		h.TermStart()
	case callTermStop:
		h.TermStop()
	case callTTYKeys:
		e, i, icrnl, onlcr, ok := h.TTYKeys(int32(a[0]))
		ret, a[0], a[1], a[2], a[3] = b2i(ok), int64(e), int64(i), b2i(icrnl), b2i(onlcr)
	case callNowMs:
		ret = h.NowMs()
	case callDelay:
		h.Delay(a[0], a[1] != 0)
	case callWaitForInput:
		ret = b2i(h.WaitForInput(a[0]))
	case callReadInput:
		buf, ok := m.guest(a[0], max(int64(int32(a[1])), 0))
		if !ok {
			return 1, false, m.fault(fmt.Sprintf("read_input into %#x+%d, outside the guest's memory", a[0], a[1]))
		}
		ret = int64(h.ReadInput(buf))
		if int32(a[1]) < 0 {
			ret = -1
		}
	case callSuspend:
		h.Suspend()
	case callExit:
		m.report()
		h.Exit(int32(a[0]))
		return int(int32(a[0])), true, nil
	case callMessage:
		msg, ok := m.guest(a[0], a[1])
		if !ok {
			return 1, false, m.fault(fmt.Sprintf("message from %#x+%d, outside the guest's memory", a[0], a[1]))
		}
		h.Message(msg, a[2] != 0)
	case callWrite:
		p, ok := m.guest(a[0], a[1])
		if !ok {
			return 1, false, m.fault(fmt.Sprintf("write from %#x+%d, outside the guest's memory", a[0], a[1]))
		}
		ret = int64(h.Write(p))
	case callTime:
		ret = h.Time()
	case callRaise:
		h.Raise(int32(a[0]))
	case callFault:
		return 1, false, m.fault(describeFault(a))
	case callRandom:
		buf, ok := m.guest(a[0], a[1])
		if !ok {
			return 1, false, m.fault(fmt.Sprintf("random into %#x+%d, outside the guest's memory", a[0], a[1]))
		}
		ret = random(buf)
	}
	le.PutUint64(cb[cbRet:], uint64(ret))
	for i := range a {
		le.PutUint64(cb[cbArgs+8*i:], uint64(a[i]))
	}
	return 0, false, nil
}

// report writes the stats, when asked for.
func (m *machine) report() {
	if m.cfg.Stats == nil {
		return
	}
	m.stats.Runs, m.stats.Spurious = hv.Stats(m.v)
	s := &m.stats
	var calls uint64
	fmt.Fprintf(m.cfg.Stats, "exits %d runs %d spurious %d guest %dus host %dus longest %dus\n",
		s.Exits, s.Runs, s.Spurious, s.Guest.Microseconds(), s.Host.Microseconds(), s.Longest.Microseconds())
	for nr, n := range s.Calls {
		if n > 0 {
			calls += n
			fmt.Fprintf(m.cfg.Stats, "call %s %d\n", callNames[nr], n)
		}
	}
	fmt.Fprintf(m.cfg.Stats, "calls %d\n", calls)
}

// watchdog ends a guest that has run Watchdog without a call: the vCPU is
// made to exit (hv_vcpus_exit), and the loop sees it was killed.
func (m *machine) watchdog() {
	t := time.NewTicker(m.cfg.Watchdog / 4)
	defer t.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-t.C:
			if m.inHost.Load() {
				m.lastCall.Store(time.Now().UnixNano())
				continue
			}
			if time.Since(time.Unix(0, m.lastCall.Load())) > m.cfg.Watchdog {
				m.killed.Store(true)
				hv.VCPUsExit(m.v)
				return
			}
		}
	}
}

// ErrNoImage is what the launcher says when it carries no guest.
var ErrNoImage = errors.New("whim-guest: no guest image: none appended to this program, no WHIM_GUEST_IMAGE, no program.elf beside it (go tool whim guest builds one)")

// The image is appended to the launcher: the ELF, then its length and a
// magic word.
const trailerMagic = "WHIMGST1"

// Appended is the guest image appended to the program at path, if any.
func Appended(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	var tr [16]byte
	if st.Size() < 16 {
		return nil, ErrNoImage
	}
	if _, err := f.ReadAt(tr[:], st.Size()-16); err != nil {
		return nil, err
	}
	if string(tr[8:]) != trailerMagic {
		return nil, ErrNoImage
	}
	n := int64(binary.LittleEndian.Uint64(tr[:8]))
	if n <= 0 || n > st.Size()-16 {
		return nil, ErrNoImage
	}
	img := make([]byte, n)
	if _, err := f.ReadAt(img, st.Size()-16-n); err != nil {
		return nil, err
	}
	return img, nil
}

// Append is a launcher carrying img: the program's bytes, img, the trailer.
func Append(prog, img []byte) []byte {
	out := append(append([]byte{}, prog...), img...)
	var tr [16]byte
	binary.LittleEndian.PutUint64(tr[:8], uint64(len(img)))
	copy(tr[8:], trailerMagic)
	return append(out, tr[:]...)
}

func describeFault(a [5]int64) string {
	return fmt.Sprintf("the guest took exception %d (code %#x) at pc %#x, address %#x", a[0], a[1], uint64(a[2]), uint64(a[3]))
}
