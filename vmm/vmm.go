// Package vmm is the monitor that runs whim's core as a virtual machine's
// only code (doc/GUEST.md): the guest image loaded into one slot of RAM, the
// vCPU put directly into the mode the core runs in, and an exit loop that
// answers each hypercall -- a store to the doorbell, decoded in
// Hypervisor.framework's form (package hv) -- by calling the same
// editor.Host the Go editor runs on.
package vmm

import (
	"bytes"
	"cmp"
	"debug/elf"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
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
	callRandom   // a Go guest's (vmm/tamago.go)
	callWaitRead // a wait and, when there is input, a read (guest/abi's WaitRead)
	callCPUStart // a Go guest's on several vCPUs (vmm/smp.go)
	callCPUPark
	callCPUWake
	callCPUSelf
	nCalls
)

var callNames = [nCalls]string{"", "host_init", "get_winsize", "term_start", "term_stop", "tty_keys",
	"now_ms", "delay", "wait_for_input", "read_input", "suspend", "exit", "message", "alloc", "free",
	"write", "time", "raise", "fault", "random", "wait_read", "cpu_start", "cpu_park", "cpu_wake", "cpu_self"}

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
	// CPUs is a Go guest's vCPUs (doc/GUEST.md, *SMP*), 1 to MaxCPUs: 0
	// or 1 is one, the boot vCPU alone, as before there were more.  A C
	// guest has one whatever this says.
	CPUs int
	// Park is the longest a parked vCPU waits while another runs the
	// guest (vmm/smp.go): 0 is DefaultPark.
	Park time.Duration
}

// MaxCPUs is the most vCPUs a guest is given (guest/abi's).
const MaxCPUs = maxCPUs

// Stats is what a run counted, over its vCPUs.
type Stats struct {
	Calls    [nCalls]uint64
	Exits    uint64 // VCPURun's returns
	Runs     uint64 // KVM_RUN calls (the backend's)
	Spurious uint64 // of those, interrupted by a signal and run again
	Guest    time.Duration
	Longest  time.Duration // the longest run between two exits
	Host     time.Duration // answering calls, a park's wait not counted
}

// deadly is the panic the host's deathtrap callback unwinds with: a deadly
// signal, carried back to the guest in the call's event.
type deadly int32

// Fault is a run's end by a fault the guest took, or an exit the monitor
// cannot answer: what the C editor would have died of a signal for.
type Fault struct{ Msg string }

func (f *Fault) Error() string { return f.Msg }

// Run runs the guest in cfg and returns its exit code: the boot vCPU on
// the calling goroutine, which it locks to its thread, and each other a
// goroutine of its own on a thread of its own.  A Host whose Exit does not
// return (the terminal's) ends the process there; one that panics with
// editor.Exit has its code returned, as editor.Main does.
func Run(cfg Config) (code int, err error) {
	m, err := newMachine(cfg)
	if err != nil {
		return 1, err
	}
	defer m.close()
	r := m.run()
	if r.panic != nil {
		c, ok := r.panic.(editor.Exit)
		if !ok {
			panic(r.panic)
		}
		return int(c), nil
	}
	return r.code, r.err
}

type machine struct {
	cfg  Config
	mem  []byte
	l    *layout
	cpus []*vcpu
	// calls counts each call, from every vCPU
	calls [nCalls]atomic.Uint64
	// host is the Host's lock: one call at a time, as the C core makes
	// them (vmm/smp.go)
	host chan struct{}
	smp  smp
	// quit is closed when the run ends: every vCPU leaves its loop
	quit     chan struct{}
	quitOnce sync.Once
	end      result
	endOnce  sync.Once
	killed   atomic.Bool
}

// vcpu is one of the machine's vCPUs and its exit loop's own counts.
type vcpu struct {
	m    *machine
	id   int
	v    hv.VCPU
	exit *hv.VCPUExit
	// the loop's counts, written by its goroutine alone
	exits, runs, spurious        uint64
	guest, longest, host, parked time.Duration // parked: of host, in CPUPark
	// the watchdog's: when it last made a call (ns), and whether it is in
	// one or parked
	lastCall atomic.Int64
	inHost   atomic.Bool
	// an AP's: its CPUStart, whether it has had one, its park
	start   chan [5]int64
	started atomic.Bool
	park    park
	// loopDone is closed when its loop has ended, done when its goroutine
	// has (its vCPU destroyed)
	loopDone chan struct{}
	done     chan struct{}
}

// result is how a run ended: an exit code, an error, or a panic the Host
// made, carried to Run's goroutine.
type result struct {
	code  int
	err   error
	panic any
}

// errStopped is a loop's end because another vCPU's ended the run.
var errStopped = errors.New("stopped")

func newMachine(cfg Config) (*machine, error) {
	f, err := elf.NewFile(bytes.NewReader(cfg.Image))
	if err != nil {
		return nil, fmt.Errorf("the guest image: %w", err)
	}
	im, err := loadImage(f, elfMachine)
	if err != nil {
		return nil, err
	}
	n := min(max(cfg.CPUs, 1), maxCPUs)
	if !isTamaGo(f) {
		n = 1
	}
	if n > 1 && im.apEntry == 0 {
		return nil, fmt.Errorf("the guest image has no whim_apentry: it runs on one vCPU, not %d", n)
	}
	m := &machine{cfg: cfg, l: layoutFor(f, im, n), host: make(chan struct{}, 1), quit: make(chan struct{})}
	m.smp.limit = cmp.Or(cfg.Park, DefaultPark)
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
	boot := m.newVCPU(0)
	boot.v, boot.exit, err = hv.VCPUCreate()
	if err != nil {
		hv.VMDestroy()
		m.close()
		return nil, err
	}
	m.cpus = []*vcpu{boot}
	if err := boot0(boot.v, m.mem, m.l); err != nil {
		m.destroy()
		return nil, err
	}
	// The other vCPUs, each created on a thread of its own and kept there
	// (hv_vcpu_create's rule), all before the guest runs: the filter
	// (Seccomp) goes on with them all there, and allows no KVM_CREATE_VCPU.
	for i := 1; i < n; i++ {
		c := m.newVCPU(i)
		ready := make(chan error)
		go c.ap(ready)
		if err := <-ready; err != nil {
			<-c.done
			m.destroy()
			return nil, err
		}
		m.cpus = append(m.cpus, c)
	}
	return m, nil
}

func (m *machine) newVCPU(i int) *vcpu {
	c := &vcpu{m: m, id: i, loopDone: make(chan struct{}), done: make(chan struct{})}
	c.park.wake = make(chan struct{}, 1)
	if i > 0 {
		c.start = make(chan [5]int64, 1)
	}
	return c
}

// boot0 sets the boot vCPU up for the image.
func boot0(v hv.VCPU, mem []byte, l *layout) error { return boot(v, mem, l) }

// destroy ends the machine: the other vCPUs told to quit and waited for
// (each destroys its own, on its own thread), the boot vCPU, the VM.
func (m *machine) destroy() {
	m.halt()
	for _, c := range m.cpus[1:] {
		<-c.done
	}
	hv.VCPUDestroy(m.cpus[0].v)
	hv.VMDestroy()
	m.close()
}

func (m *machine) close() {
	if m.mem != nil {
		syscall.Munmap(m.mem)
		m.mem = nil
	}
}

// halt ends the run for every vCPU: those waiting to start, parked or
// waiting for the Host leave at once, and those in the guest are made to
// exit (hv_vcpus_exit).
func (m *machine) halt() {
	m.quitOnce.Do(func() {
		close(m.quit)
		for _, c := range m.cpus {
			hv.VCPUsExit(c.v)
		}
	})
}

// finish records how the run ended, the first end only, and halts it.
func (m *machine) finish(r result) {
	m.endOnce.Do(func() { m.end = r })
	m.halt()
}

// run runs the boot vCPU's loop on this goroutine, until the run ends, and
// tears the machine down.
func (m *machine) run() result {
	defer m.destroy()
	if m.cfg.Seccomp != nil {
		if err := m.cfg.Seccomp(); err != nil {
			return result{code: 1, err: fmt.Errorf("seccomp: %w", err)}
		}
	}
	stop := make(chan struct{})
	defer close(stop)
	if m.cfg.Watchdog > 0 {
		go m.watchdog(stop)
	}
	m.smp.running = 1
	m.cpus[0].serve()
	close(m.cpus[0].loopDone)
	for _, c := range m.cpus[1:] {
		<-c.loopDone
	}
	return m.end
}

// ap is an AP's goroutine: its vCPU created and set up on this thread,
// then its start waited for and its loop run, until the run ends.
func (c *vcpu) ap(ready chan<- error) {
	defer close(c.done)
	hv.LockThread() // never unlocked: the thread ends with the goroutine
	var err error
	c.v, c.exit, err = hv.VCPUCreate()
	if err != nil {
		close(c.loopDone)
		ready <- err
		return
	}
	defer hv.VCPUDestroy(c.v)
	defer close(c.loopDone)
	if err := setupAP(c.v, c.m.l, c.id); err != nil {
		ready <- err
		return
	}
	ready <- nil
	select {
	case a := <-c.start:
		if err := startAP(c.v, c.m.l, a); err != nil {
			c.m.finish(result{code: 1, err: err})
			return
		}
		c.serve()
	case <-c.m.quit:
	}
}

// serve runs the vCPU's loop and ends the run with its end: an exit, a
// fault, an error, or a Host's panic.
func (c *vcpu) serve() {
	defer func() { c.runs, c.spurious = hv.Stats(c.v) }()
	defer func() {
		if r := recover(); r != nil {
			c.m.finish(result{code: 1, panic: r})
		}
	}()
	code, err := c.loop()
	if err != errStopped {
		c.m.finish(result{code: code, err: err})
	}
}

// loop runs the vCPU until the guest exits, answering its calls.
func (c *vcpu) loop() (int, error) {
	m := c.m
	c.lastCall.Store(time.Now().UnixNano())
	for {
		t0 := time.Now()
		if err := hv.VCPURun(c.v); err != nil {
			return 1, err
		}
		t1 := time.Now()
		c.guest += t1.Sub(t0)
		c.longest = max(c.longest, t1.Sub(t0))
		c.exits++
		switch c.exit.Reason {
		case hv.ExitReasonException:
			code, done, err := c.exception()
			c.host += time.Since(t1)
			if done || err != nil {
				return code, err
			}
		case hv.ExitReasonCanceled:
			if m.killed.Load() {
				return 1, m.fault(fmt.Sprintf("the guest ran %s without a hypercall: ended by the watchdog", m.cfg.Watchdog))
			}
			if m.stopped() {
				return 1, errStopped
			}
		default:
			return 1, m.fault(fmt.Sprintf("the guest stopped: %s %s", c.exit.Reason, c.exit.Detail))
		}
	}
}

// stopped is whether the run has ended.
func (m *machine) stopped() bool {
	select {
	case <-m.quit:
		return true
	default:
		return false
	}
}

// exception answers an exception exit: a store to the doorbell is a call.
func (c *vcpu) exception() (int, bool, error) {
	m := c.m
	e := c.exit.Exception
	s := hv.Syndrome(e.Syndrome)
	addr := uint64(e.PhysicalAddress)
	reg, alt := altCall(c.v, s, addr)
	if !alt {
		if s.EC() != hv.ECDataAbortLower || addr != Doorbell || !s.ISV() || !s.WnR() {
			return 1, false, m.fault(fmt.Sprintf("the guest touched %#x with no memory there (syndrome %#x, pc %#x)", addr, e.Syndrome, pc(c.v)))
		}
		var ok bool
		if reg, ok = hv.RegForSRT(s.SRT()); !ok {
			return 1, false, m.fault(fmt.Sprintf("a doorbell store from register %d", s.SRT()))
		}
	}
	cb, err := hv.VCPUGetReg(c.v, reg)
	if err != nil {
		return 1, false, err
	}
	if err := advance(c.v, s); err != nil {
		return 1, false, err
	}
	if cb%cbSize != 0 || cb < imageBase || cb+cbSize > m.l.size {
		return 1, false, m.fault(fmt.Sprintf("a call block at %#x, outside the guest's memory", cb))
	}
	c.lastCall.Store(time.Now().UnixNano())
	c.inHost.Store(true)
	defer func() {
		c.lastCall.Store(time.Now().UnixNano())
		c.inHost.Store(false)
	}()
	return c.call(m.mem[cb : cb+cbSize])
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

// call answers the call in block cb: the vCPUs' own (vmm/smp.go) at
// once, a Host's on m.cfg.Host, one at a time.
func (c *vcpu) call(cb []byte) (code int, done bool, err error) {
	m := c.m
	le := binary.LittleEndian
	nr := le.Uint64(cb[cbNr:])
	var a [5]int64
	for i := range a {
		a[i] = int64(le.Uint64(cb[cbArgs+8*i:]))
	}
	if nr == 0 || nr >= nCalls || nr == callAlloc || nr == callFree {
		return 1, false, m.fault(fmt.Sprintf("an unknown call %d", nr))
	}
	m.calls[nr].Add(1)
	ret := int64(0)
	switch nr {
	case callCPUStart, callCPUPark, callCPUWake, callCPUSelf:
		ret, err = c.smpCall(nr, a)
		if err != nil {
			return 1, false, err
		}
		le.PutUint64(cb[cbRet:], uint64(ret))
		return 0, false, nil
	case callFault:
		return 1, false, m.fault(describeFault(a))
	}
	if !c.lockHost() {
		return 1, false, errStopped
	}
	defer c.unlockHost()
	h := m.cfg.Host
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
	case callWaitRead:
		// The wait, and the read the core makes after it, in one exit: the
		// read into the guest runtime's buffer, which serves the core's
		// read_input from it.  A deadly signal in either unwinds before a
		// byte is read (both deliver before they touch stdin), and the guest
		// makes the call again, as it makes a wait again.
		buf, ok := m.guest(a[1], a[2])
		if !ok || a[2] <= 0 {
			return 1, false, m.fault(fmt.Sprintf("wait_read into %#x+%d, outside the guest's memory", a[1], a[2]))
		}
		if ret = b2i(h.WaitForInput(a[0])); ret != 0 {
			a[0] = int64(h.ReadInput(buf))
		}
	case callSuspend:
		h.Suspend()
	case callExit:
		c.stopOthers()
		m.report(c)
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

// report writes the stats, when asked for: every vCPU's, summed, once
// the others have stopped.
func (m *machine) report(self *vcpu) {
	if m.cfg.Stats == nil {
		return
	}
	var st Stats
	var parked time.Duration
	for _, c := range m.cpus {
		runs, spurious := c.runs, c.spurious
		if c == self {
			runs, spurious = hv.Stats(c.v)
		}
		st.Exits += c.exits
		st.Runs += runs
		st.Spurious += spurious
		st.Guest += c.guest
		st.Longest = max(st.Longest, c.longest)
		st.Host += c.host - c.parked
		parked += c.parked
	}
	for nr := range st.Calls {
		st.Calls[nr] = m.calls[nr].Load()
	}
	s := &st
	var calls uint64
	fmt.Fprintf(m.cfg.Stats, "exits %d runs %d spurious %d guest %dus host %dus longest %dus\n",
		s.Exits, s.Runs, s.Spurious, s.Guest.Microseconds(), s.Host.Microseconds(), s.Longest.Microseconds())
	if len(m.cpus) > 1 {
		fmt.Fprintf(m.cfg.Stats, "cpus %d started %d parked %dus\n", len(m.cpus), m.startedCPUs(), parked.Microseconds())
	}
	for nr, n := range s.Calls {
		if n > 0 {
			calls += n
			fmt.Fprintf(m.cfg.Stats, "call %s %d\n", callNames[nr], n)
		}
	}
	fmt.Fprintf(m.cfg.Stats, "calls %d\n", calls)
}

// watchdog ends a guest one of whose vCPUs has run Watchdog without a
// call: every vCPU is made to exit (hv_vcpus_exit), and the loops see it
// was killed.  A vCPU in a call or parked is not running the guest.
func (m *machine) watchdog(stop <-chan struct{}) {
	t := time.NewTicker(m.cfg.Watchdog / 4)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			for _, c := range m.cpus {
				if c.id > 0 && !c.started.Load() {
					continue
				}
				if c.inHost.Load() {
					c.lastCall.Store(time.Now().UnixNano())
					continue
				}
				if time.Since(time.Unix(0, c.lastCall.Load())) > m.cfg.Watchdog {
					m.killed.Store(true)
					m.halt()
					return
				}
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
