package p073

// Whim phase 73 (formerly 147) -- deathtrap() runs at the host's next wait.  See GOAL.md.
//
// SIGHUP and SIGTERM ran deathtrap() as their handler (internal/gen/FINDINGS.md, 12).
// The handler records the signal and writes a byte to a pipe; the host's wait
// selects on the pipe beside the input, and the wait and the read run
// deathtrap() first, inside the wait where the core unblocks deadly signals.
// The libc surface grows by pipe2.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim73", Edit) }

const (
	W73Handler = `    static void
host_on_death(int sigarg)
{
    int         e = errno;

    host_death_pending = sigarg;
    if (host_death_pipe[1] >= 0)
    {
        (void)write(host_death_pipe[1], "", 1);
    }
    errno = e;
}
`
	W73Deliver = `    static void
host_deliver_death(void)
{
    char        b[16];
    int         sig;

    if (host_death_pipe[0] >= 0)
    {
        while (read(host_death_pipe[0], b, sizeof(b)) > 0)
        {
            ;
        }
    }
    sig = host_death_pending;
    if (sig != 0)
    {
        host_death_pending = 0;
        deathtrap(sig);
    }
}
`
)

// Edit runs deathtrap() at the host's next wait, woken by a self-pipe.
//
// SIGHUP and SIGTERM ran deathtrap() as their handler: the core's whole way
// out -- preserving, restoring the terminal, writing its message -- inside a
// signal handler, at whatever point the signal found the core, which is
// undefined behaviour in C and not expressible in Go, whose runtime takes the
// signal and hands it to a goroutine (internal/gen/FINDINGS.md, 12).  The handler now
// records the signal and writes a byte to a pipe; the host's wait selects on
// the pipe beside the input, and the wait and the read run deathtrap() first.
// The pipe is what makes it race-free: a signal that lands after the flag was
// tested and before select() leaves a byte that ends the select at once.  The
// Go host already does exactly this; it now transpiles line for line.
//
// The core already blocks deadly signals everywhere but the wait in
// ui_inchar(): vim_handle_signal() turns one that arrives while it is busy into
// an interrupt and raises it again when that wait unblocks.  So deathtrap() only
// ever ran in that window, and now runs at the wait or read inside it -- the
// same moment but for the few statements between the unblock and the wait.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): the objects, the functions and
// the statements written by FRAG in one unit (Together), then <fcntl.h> moved
// beside <termios.h> (INCLUDE: one form moved, its id kept, under the extern
// rule).  History keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("selfpipe", e, w)
	v.Together(func(v *graph.Verbs) {
		v.TopAfterC("host_int_pending", "static volatile sig_atomic_t host_death_pending = 0;\nstatic int host_death_pipe[2] = {-1, -1};\n",
			"a deadly signal is recorded, beside a pipe that wakes the wait")
		v.TopAfterC("host_on_int", W73Handler+"\n"+W73Deliver,
			"host_on_death() records it and writes to the pipe; host_deliver_death() drains the pipe and runs deathtrap()")
		v.LiteralC("    host_catch(SIGHUP, deathtrap);\n    host_catch(SIGTERM, deathtrap);\n", "    if (pipe2(host_death_pipe, O_NONBLOCK | O_CLOEXEC) != 0)\n    {\n        host_death_pipe[0] = -1;\n        host_death_pipe[1] = -1;\n    }\n    host_catch(SIGHUP, host_on_death);\n    host_catch(SIGTERM, host_on_death);\n", 1,
			"SIGHUP and SIGTERM are caught by host_on_death(), not deathtrap()")
		v.LiteralC("    for (;;)\n    {\n        if (host_winch_pending || host_tstp_pending || host_int_pending)\n        {\n            return 1;\n        }\n        FD_ZERO(&rfds);\n        FD_SET(0, &rfds);\n        ret = select(1, &rfds, nullptr, nullptr, tvp);\n        if (ret == -1 && errno == EINTR)\n        {\n            continue;\n        }\n        return ret > 0 && FD_ISSET(0, &rfds);\n    }\n", "    for (;;)\n    {\n        host_deliver_death();\n        if (host_winch_pending || host_tstp_pending || host_int_pending)\n        {\n            return 1;\n        }\n        FD_ZERO(&rfds);\n        FD_SET(0, &rfds);\n        if (host_death_pipe[0] >= 0)\n        {\n            FD_SET(host_death_pipe[0], &rfds);\n        }\n        ret = select(host_death_pipe[0] >= 0 ? host_death_pipe[0] + 1 : 1, &rfds, nullptr, nullptr, tvp);\n        if (ret == -1 && errno == EINTR)\n        {\n            continue;\n        }\n        if (ret > 0 && host_death_pipe[0] >= 0 && FD_ISSET(host_death_pipe[0], &rfds))\n        {\n            continue;\n        }\n        return ret > 0 && FD_ISSET(0, &rfds);\n    }\n", 1,
			"the wait delivers a deadly signal first, and selects on the pipe beside the input")
		v.InFunction("musl_read_input", func(v *graph.Verbs) {
			v.BeforeC("(if host_int_pending _)", "host_deliver_death();\n", 1, "and so does the read")
		})
	})
	if v.Failed() {
		return v.Done()
	}
	// <fcntl.h> is still there, since phase 88 drops the unused headers last: it
	// moves to where this phase has always put it.
	var fcntl, termios *graph.Node
	for _, inc := range e.Includes() {
		switch graph.IncludeSpec(inc) {
		case "<fcntl.h>":
			fcntl = inc
		case "<termios.h>":
			termios = inc
		}
	}
	if fcntl == nil || termios == nil {
		v.Die("<fcntl.h> or <termios.h> is not included")
		return v.Done()
	}
	if err := e.MoveFormsAfter(termios, fcntl); err != nil {
		v.Die("<fcntl.h> moves beside <termios.h> -- %v", err)
		return v.Done()
	}
	v.Say("<fcntl.h>, never dropped, moves beside <termios.h>, where the host includes it for the pipe's flags")
	return v.Done()
}
