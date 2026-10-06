//go:build !linux

package vmm

import "time"

// A parker is a vCPU's park: a wake token in a channel, waited for with a
// timer (park_linux.go has why Linux's is a futex).
type parker struct{ ch chan struct{} }

func (p *parker) init() { p.ch = make(chan struct{}, 1) }

// wake ends the park, or the next: the token kept until it is taken.
func (p *parker) wake() {
	select {
	case p.ch <- struct{}{}:
	default:
	}
}

// wait waits for a wake, until until (zero: no limit): true when woken.
func (p *parker) wait(until time.Time) bool {
	if until.IsZero() {
		<-p.ch
		return true
	}
	d := time.Until(until)
	if d <= 0 {
		select {
		case <-p.ch:
			return true
		default:
			return false
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-p.ch:
		return true
	case <-t.C:
		return false
	}
}
