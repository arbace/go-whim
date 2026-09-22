package p147

// Whim phase 147, the check -- deathtrap() runs at the host's next wait.
// See phase/147/edit.go, and GOALS.md.
//
// phase/147/check.go requires the handler and delivery, the libc surface
// grown by exactly pipe2, and probes SIGTERM and SIGHUP while waiting and
// SIGTERM while busy, on a real pty, against the input's binary.

import (
	"bytes"
	"io"
	"strings"
	"syscall"
	"time"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim147", Check) }

// Whim147 is phase 147's check: deathtrap() runs at the host's next wait.
//
//  1. THE CUT: host_on_death() and host_deliver_death() are the phase's text;
//     SIGHUP and SIGTERM are caught by host_on_death() and deathtrap() is
//     handed to no sigaction; the wait and the read each deliver a pending
//     signal first, and the wait selects on the pipe.
//  2. THE LIBC SURFACE grows by exactly pipe2 -- the one declared change.
//  3. THE GATE otherwise: a silent compile, the linkage, the build.
//  4. THE PROBES, on a real pty: SIGTERM and SIGHUP while the editor waits for
//     a key, which each say the editor caught a deadly signal; and SIGTERM
//     while it is busy in a substitution that backtracks for longer than the
//     probe runs.  The core already blocks deadly signals outside the wait in
//     ui_inchar() -- vim_handle_signal() turns one into an interrupt and raises
//     it again when the wait unblocks -- so a busy editor says "Interrupted";
//     that the phase does not disturb it is the probe.  The waiting runs are
//     the same output, stderr and exit status on both binaries; the busy run
//     is interrupted on both, with the same stderr and exit status -- how much
//     of its screen is drawn by the end of the window is the machine's load.  The CONTROLS: SIGTERM
//     and SIGHUP differ (the message names the signal), and the busy run was
//     interrupted and never finished its substitution (no E486).
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim147", "selfpipe")
	if err != nil {
		return err
	}
	r := c.R
	for _, s := range []string{W147Handler, W147Deliver,
		"    host_catch(SIGHUP, host_on_death);\n    host_catch(SIGTERM, host_on_death);\n",
		"    for (;;)\n    {\n        host_deliver_death();\n",
		"musl_read_input(char *buf, int len)\n{\n    host_deliver_death();\n",
		"FD_SET(host_death_pipe[0], &rfds);"} {
		if strings.Count(c.New, s) != 1 {
			r.Bad("the host does not have, once: %q", strings.SplitN(s, "\n", 2)[0])
		}
	}
	if strings.Contains(c.New, "host_catch(SIGHUP, deathtrap)") || strings.Contains(c.New, "host_catch(SIGTERM, deathtrap)") {
		r.Bad("deathtrap() is still a signal handler")
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("SIGHUP and SIGTERM set a flag and write to a pipe; the wait selects on the pipe, and the wait and the read run deathtrap() first")

	if err := c.Gate(false); err != nil {
		return err
	}
	before := strings.Fields(c.SymbolsInput)
	after := strings.Fields(check.ReadFile(".cache/symbols/last/undefined"))
	want := append(append([]string{}, before...), "pipe2")
	if !sameSet(after, want) {
		r.Bad("the libc surface is %v, where the input's and pipe2 are %v", after, want)
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("the libc surface grew by exactly pipe2: %d -> %d", len(before), len(after))

	ob, nb := c.Bins()
	type run struct {
		Out, errb []byte
		Rc        int
	}
	do := func(bin string, keys [][]byte, sig syscall.Signal) (run, error) {
		o, e, rc, err := harness.PtySplit(bin, nil, keys, sig, 500*time.Millisecond, "xterm", 24, 80)
		return run{o, e, rc}, err
	}
	idle := [][]byte{[]byte("ihello\x1b")}
	// `a\(a\|aa\)*b` over a line of 34 a's after a b backtracks
	// exponentially from every a and fails: about thirty seconds on the
	// input's binary, measured at 4.3 s for 30 a's and rising 1.6 times an a
	busy := [][]byte{[]byte("ib\x1b34aa\x1b0"), []byte(":s/a\\(a\\|aa\\)*b/x/\r")}
	var got []run
	for _, pr := range []struct {
		What string
		Keys [][]byte
		sig  syscall.Signal
	}{{"SIGTERM while waiting", idle, syscall.SIGTERM}, {"SIGHUP while waiting", idle, syscall.SIGHUP}, {"SIGTERM while busy", busy, syscall.SIGTERM}} {
		a, e1 := do(ob, pr.Keys, pr.sig)
		b, e2 := do(nb, pr.Keys, pr.sig)
		if e1 != nil || e2 != nil {
			r.Say("a probe did not run: %v %v", e1, e2)
			return harness.ErrReported
		}
		isBusy := len(pr.Keys) > 1
		switch {
		case !isBusy && (!bytes.Equal(a.Out, b.Out) || !bytes.Equal(a.errb, b.errb) || a.Rc != b.Rc):
			r.Bad("%s: the two binaries differ (exit %d and %d)", pr.What, a.Rc, b.Rc)
		case isBusy && (!bytes.Equal(a.errb, b.errb) || a.Rc != b.Rc ||
			!bytes.Contains(a.Out, []byte("Interrupted")) || !bytes.Contains(b.Out, []byte("Interrupted")) ||
			bytes.Contains(a.Out, []byte("E486")) || bytes.Contains(b.Out, []byte("E486"))):
			// how much of the interrupted screen is drawn by the end of the
			// probe's window depends on the machine's load: run alone the two
			// binaries drew the same bytes, and in a loaded stage they did
			// not, so the busy run compares what the claim is about
			r.Bad("%s: the two binaries are not both interrupted with the same stderr and exit (%d and %d)", pr.What, a.Rc, b.Rc)
		}
		if pr.Keys[0][0] == 'i' && len(pr.Keys) == 1 && !bytes.Contains(b.Out, []byte("Caught deadly signal")) {
			r.Bad("%s: the editor did not report a deadly signal", pr.What)
		}
		got = append(got, b)
	}
	if bytes.Equal(got[0].Out, got[1].Out) {
		r.Bad("the CONTROL did not move: SIGTERM and SIGHUP gave the same output")
	}
	if bytes.Contains(got[2].Out, []byte("E486")) || !bytes.Contains(got[2].Out, []byte("Interrupted")) {
		r.Bad("the CONTROL failed: the busy substitution was not interrupted mid-computation")
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("PROBE: SIGTERM and SIGHUP while waiting (a deadly signal, exit %d), and SIGTERM mid-substitution (an interrupt), give the same output, stderr and exit status on both binaries; SIGTERM and SIGHUP differ, and the substitution was interrupted", got[0].Rc)
	return nil
}

func sameSet(a, b []string) bool {
	m := map[string]int{}
	for _, x := range a {
		m[x]++
	}
	for _, x := range b {
		m[x]--
	}
	for _, v := range m {
		if v != 0 {
			return false
		}
	}
	return true
}
