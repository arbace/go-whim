package edit

import (
	"io"
)

func init() { register("whim147", Whim147) }

// W147Handler and W147Deliver are the host's new functions, exported so the
// check requires the identical text.
const (
	W147Handler = `    static void
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
	W147Deliver = `    static void
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

// Whim147 runs deathtrap() at the host's next wait, woken by a self-pipe.
//
// SIGHUP and SIGTERM ran deathtrap() as their handler: the core's whole way
// out -- preserving, restoring the terminal, writing its message -- inside a
// signal handler, at whatever point the signal found the core, which is
// undefined behaviour in C and not expressible in Go, whose runtime takes the
// signal and hands it to a goroutine (tx/FINDINGS.md, 12).  The handler now
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
func Whim147(text []byte, w io.Writer) ([]byte, error) {
	p := ph{tag: "selfpipe", w: w}
	var err error
	steps := []struct{ old, new, what string }{
		{"#include <termios.h>\n", "#include <termios.h>\n#include <fcntl.h>\n", "the host includes <fcntl.h> for the pipe's flags"},
		{"static volatile sig_atomic_t host_int_pending = FALSE;\n",
			"static volatile sig_atomic_t host_int_pending = FALSE;\nstatic volatile sig_atomic_t host_death_pending = 0;\nstatic int host_death_pipe[2] = {-1, -1};\n",
			"a deadly signal is recorded, beside a pipe that wakes the wait"},
		{"    static void\nhost_on_int(int sigarg)\n{\n    host_int_pending = TRUE;\n}\n",
			"    static void\nhost_on_int(int sigarg)\n{\n    host_int_pending = TRUE;\n}\n\n" + W147Handler + "\n" + W147Deliver,
			"host_on_death() records it and writes to the pipe; host_deliver_death() drains the pipe and runs deathtrap()"},
		{"    host_catch(SIGHUP, deathtrap);\n    host_catch(SIGTERM, deathtrap);\n",
			"    if (pipe2(host_death_pipe, O_NONBLOCK | O_CLOEXEC) != 0)\n    {\n        host_death_pipe[0] = -1;\n        host_death_pipe[1] = -1;\n    }\n    host_catch(SIGHUP, host_on_death);\n    host_catch(SIGTERM, host_on_death);\n",
			"SIGHUP and SIGTERM are caught by host_on_death(), not deathtrap()"},
		{"    for (;;)\n    {\n        if (host_winch_pending || host_tstp_pending || host_int_pending)\n        {\n            return 1;\n        }\n        FD_ZERO(&rfds);\n        FD_SET(0, &rfds);\n        ret = select(1, &rfds, nullptr, nullptr, tvp);\n        if (ret == -1 && errno == EINTR)\n        {\n            continue;\n        }\n        return ret > 0 && FD_ISSET(0, &rfds);\n    }\n",
			"    for (;;)\n    {\n        host_deliver_death();\n        if (host_winch_pending || host_tstp_pending || host_int_pending)\n        {\n            return 1;\n        }\n        FD_ZERO(&rfds);\n        FD_SET(0, &rfds);\n        if (host_death_pipe[0] >= 0)\n        {\n            FD_SET(host_death_pipe[0], &rfds);\n        }\n        ret = select(host_death_pipe[0] >= 0 ? host_death_pipe[0] + 1 : 1, &rfds, nullptr, nullptr, tvp);\n        if (ret == -1 && errno == EINTR)\n        {\n            continue;\n        }\n        if (ret > 0 && host_death_pipe[0] >= 0 && FD_ISSET(host_death_pipe[0], &rfds))\n        {\n            continue;\n        }\n        return ret > 0 && FD_ISSET(0, &rfds);\n    }\n",
			"the wait delivers a deadly signal first, and selects on the pipe beside the input"},
		{"musl_read_input(char *buf, int len)\n{\n", "musl_read_input(char *buf, int len)\n{\n    host_deliver_death();\n", "and so does the read"},
	}
	for _, s := range steps {
		if text, err = p.literal(text, s.old, s.new, s.what, 1); err != nil {
			return nil, err
		}
	}
	return text, nil
}
