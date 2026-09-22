package check

import (
	"bytes"
	"io"
	"strings"
	"syscall"
	"time"

	"github.com/arbace/go-whim/internal/edit"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { register("whim147", Whim147) }

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
func Whim147(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim147", "selfpipe")
	if err != nil {
		return err
	}
	r := c.r
	for _, s := range []string{edit.W147Handler, edit.W147Deliver,
		"    host_catch(SIGHUP, host_on_death);\n    host_catch(SIGTERM, host_on_death);\n",
		"    for (;;)\n    {\n        host_deliver_death();\n",
		"musl_read_input(char *buf, int len)\n{\n    host_deliver_death();\n",
		"FD_SET(host_death_pipe[0], &rfds);"} {
		if strings.Count(c.new, s) != 1 {
			r.bad("the host does not have, once: %q", strings.SplitN(s, "\n", 2)[0])
		}
	}
	if strings.Contains(c.new, "host_catch(SIGHUP, deathtrap)") || strings.Contains(c.new, "host_catch(SIGTERM, deathtrap)") {
		r.bad("deathtrap() is still a signal handler")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("SIGHUP and SIGTERM set a flag and write to a pipe; the wait selects on the pipe, and the wait and the read run deathtrap() first")

	if err := c.gate(false); err != nil {
		return err
	}
	before := strings.Fields(c.symbolsInput)
	after := strings.Fields(readFile(".cache/symbols/last/undefined"))
	want := append(append([]string{}, before...), "pipe2")
	if !sameSet(after, want) {
		r.bad("the libc surface is %v, where the input's and pipe2 are %v", after, want)
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("the libc surface grew by exactly pipe2: %d -> %d", len(before), len(after))

	ob, nb := c.bins()
	type run struct {
		out, errb []byte
		rc        int
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
		what string
		keys [][]byte
		sig  syscall.Signal
	}{{"SIGTERM while waiting", idle, syscall.SIGTERM}, {"SIGHUP while waiting", idle, syscall.SIGHUP}, {"SIGTERM while busy", busy, syscall.SIGTERM}} {
		a, e1 := do(ob, pr.keys, pr.sig)
		b, e2 := do(nb, pr.keys, pr.sig)
		if e1 != nil || e2 != nil {
			r.say("a probe did not run: %v %v", e1, e2)
			return harness.ErrReported
		}
		isBusy := len(pr.keys) > 1
		switch {
		case !isBusy && (!bytes.Equal(a.out, b.out) || !bytes.Equal(a.errb, b.errb) || a.rc != b.rc):
			r.bad("%s: the two binaries differ (exit %d and %d)", pr.what, a.rc, b.rc)
		case isBusy && (!bytes.Equal(a.errb, b.errb) || a.rc != b.rc ||
			!bytes.Contains(a.out, []byte("Interrupted")) || !bytes.Contains(b.out, []byte("Interrupted")) ||
			bytes.Contains(a.out, []byte("E486")) || bytes.Contains(b.out, []byte("E486"))):
			// how much of the interrupted screen is drawn by the end of the
			// probe's window depends on the machine's load: run alone the two
			// binaries drew the same bytes, and in a loaded stage they did
			// not, so the busy run compares what the claim is about
			r.bad("%s: the two binaries are not both interrupted with the same stderr and exit (%d and %d)", pr.what, a.rc, b.rc)
		}
		if pr.keys[0][0] == 'i' && len(pr.keys) == 1 && !bytes.Contains(b.out, []byte("Caught deadly signal")) {
			r.bad("%s: the editor did not report a deadly signal", pr.what)
		}
		got = append(got, b)
	}
	if bytes.Equal(got[0].out, got[1].out) {
		r.bad("the CONTROL did not move: SIGTERM and SIGHUP gave the same output")
	}
	if bytes.Contains(got[2].out, []byte("E486")) || !bytes.Contains(got[2].out, []byte("Interrupted")) {
		r.bad("the CONTROL failed: the busy substitution was not interrupted mid-computation")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("PROBE: SIGTERM and SIGHUP while waiting (a deadly signal, exit %d), and SIGTERM mid-substitution (an interrupt), give the same output, stderr and exit status on both binaries; SIGTERM and SIGHUP differ, and the substitution was interrupted", got[0].rc)
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
