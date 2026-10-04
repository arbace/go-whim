package vmm

import (
	"fmt"
	"sync"
	"time"

	"github.com/arbace/go-whim/guest/abi"
)

// A Go guest on several vCPUs (doc/GUEST.md, *SMP*).  The monitor creates
// them all before the guest runs, each on a thread of its own with an exit
// loop of its own; the boot vCPU starts the image, and the others wait
// until the runtime asks for one (goos.Task, abi.CPUStart).  The Host's
// calls are made one at a time, as the C core makes them: a vCPU waits for
// the one before to be answered.  The vCPUs' own calls -- start, park,
// wake, self -- are not the Host's and wait for nothing.
//
// There are no interrupts, so no IPI: an idle vCPU parks in the monitor
// (abi.CPUPark, TamaGo's goos.Idle) and another's abi.CPUWake ends the park
// (goos.Wake, the runtime's semawakeup).  But TamaGo's scheduler never
// drops an M's P and never wakes an M for new work (its wakep finds no
// idle P): an idle vCPU must look again by itself, for a fork's
// goroutines to steal or a stop of the world to join.  So while another
// vCPU runs the guest, a park lasts at most the limit (DefaultPark); when
// none does -- all parked, or in a Host call (the boot vCPU waiting for a
// key) -- nothing can make work but a Host call's return or a park's end,
// and a park lasts until one of those, which wakes every vCPU so parked.
// The invariant: while a vCPU runs the guest, every park ends within the
// limit.

// maxCPUs is guest/abi's MaxCPUs.
const maxCPUs = abi.MaxCPUs

// DefaultPark is the longest a parked vCPU waits while another runs the
// guest: how late an idle vCPU may see a fork's goroutines or a stop of
// the world.
const DefaultPark = time.Millisecond

// smp is the machine's count of vCPUs running the guest, and its limit on
// a park.
type smp struct {
	sync.Mutex
	running int // vCPUs in the guest: not parked, not in a Host call
	limit   time.Duration
}

// park is a vCPU's park: whether it is parked and for how long, and its
// wake -- a token, so that a wake before the park is not lost.
type park struct {
	state parkState
	wake  chan struct{}
}

type parkState int

const (
	notParked        parkState = iota
	parkedLimited              // while another runs the guest: at most the limit
	parkedUntilWoken           // while none does: until a wake
)

// smpCall answers the vCPUs' own calls.
func (c *vcpu) smpCall(nr uint64, a [5]int64) (int64, error) {
	m := c.m
	switch nr {
	case callCPUSelf:
		return int64(c.id), nil
	case callCPUWake:
		if a[0] < 0 || a[0] >= int64(len(m.cpus)) {
			return -1, nil
		}
		m.cpus[a[0]].wake()
		return 0, nil
	case callCPUStart:
		if a[0] < 1 || a[0] >= int64(len(m.cpus)) {
			return -1, nil
		}
		t := m.cpus[a[0]]
		if t.started.Swap(true) {
			return -1, nil
		}
		m.release(a[4]-abi.TaskStackBytes, a[4])
		t.lastCall.Store(time.Now().UnixNano())
		m.smp.Lock()
		m.smp.running++
		m.smp.Unlock()
		t.start <- a
		return 0, nil
	case callCPUPark:
		if err := c.parkFor(a[0]); err != nil {
			return 0, err
		}
		return 0, nil
	}
	return 0, m.fault(fmt.Sprintf("an unknown call %d", nr))
}

// release gives the pages of guest memory [lo, hi) back to the host: what
// the guest says it will not use (abi.CPUStart's a[4]), read as zeros if
// it does.  A range not wholly in the slot is left alone.
func (m *machine) release(lo, hi int64) {
	lo, hi = (lo+page-1)&^(page-1), hi&^(page-1)
	if lo < imageBase || hi <= lo || uint64(hi) > m.l.size {
		return
	}
	dontNeed(m.mem[lo:hi])
}

// wake ends c's park, or its next.
func (c *vcpu) wake() {
	select {
	case c.park.wake <- struct{}{}:
	default:
	}
}

// parkFor parks c: until a wake, ns nanoseconds of the guest's (ns < 0: no
// limit of its own), and at most the machine's limit while another vCPU
// runs the guest.  A park the limit ends while none runs it goes on, until
// a wake: the guest is idle.
func (c *vcpu) parkFor(ns int64) error {
	m := c.m
	t0 := time.Now()
	defer func() { c.parked += time.Since(t0) }()
	var deadline <-chan time.Time
	if ns >= 0 {
		t := time.NewTimer(time.Duration(ns))
		defer t.Stop()
		deadline = t.C
	}
	m.smp.Lock()
	m.smp.running--
	c.park.state = parkedUntilWoken
	if m.smp.running > 0 {
		c.park.state = parkedLimited
	}
	m.smp.Unlock()
	var limit *time.Timer
	for {
		var limited <-chan time.Time
		m.smp.Lock()
		if c.park.state == parkedLimited {
			if limit == nil {
				limit = time.NewTimer(m.smp.limit)
				defer limit.Stop()
			}
			limited = limit.C
		}
		m.smp.Unlock()
		select {
		case <-c.park.wake:
		case <-deadline:
		case <-limited:
			m.smp.Lock()
			if m.smp.running == 0 {
				c.park.state = parkedUntilWoken
				m.smp.Unlock()
				continue
			}
			m.smp.Unlock()
		case <-m.quit:
			return errStopped
		}
		break
	}
	m.resume(c)
	return nil
}

// resume counts c running the guest again, after a park or a Host call:
// when it is the only one, every vCPU parked until woken is woken -- now
// that one runs, their parks are limited, and they look again.
func (m *machine) resume(c *vcpu) {
	m.smp.Lock()
	defer m.smp.Unlock()
	c.park.state = notParked
	m.smp.running++
	if m.smp.running != 1 {
		return
	}
	for _, o := range m.cpus {
		if o != c && o.park.state == parkedUntilWoken {
			o.park.state = parkedLimited
			o.wake()
		}
	}
}

// lockHost waits for the Host, counting c out of the guest meanwhile;
// false when the run ended first.
func (c *vcpu) lockHost() bool {
	m := c.m
	if len(m.cpus) > 1 {
		m.smp.Lock()
		m.smp.running--
		m.smp.Unlock()
	}
	select {
	case m.host <- struct{}{}:
		return true
	case <-m.quit:
		return false
	}
}

// unlockHost lets the next vCPU's call to the Host go, and counts c in the
// guest again.
func (c *vcpu) unlockHost() {
	<-c.m.host
	if len(c.m.cpus) > 1 {
		c.m.resume(c)
	}
}

// stopOthers ends the run for every vCPU but c, and waits for their loops
// to end: an exit's report counts them all.
func (c *vcpu) stopOthers() {
	if len(c.m.cpus) == 1 {
		return
	}
	c.m.halt()
	for _, o := range c.m.cpus {
		if o != c {
			<-o.loopDone
		}
	}
}

// startedCPUs is how many vCPUs ran the guest: the boot vCPU and those
// started.
func (m *machine) startedCPUs() int {
	n := 1
	for _, c := range m.cpus[1:] {
		if c.started.Load() {
			n++
		}
	}
	return n
}
