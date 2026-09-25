// The host: what the editor core asks of an operating system, behind an
// interface.  In whim-vim.c the host is everything from the first #include to
// the end of the file, and the core calls it by name -- musl_read_input,
// host_write and the rest.  Here the core (editor.go, generated) still calls
// those names, and each is a line of glue to the Host an embedding program
// hands Main: the C signature on this side, Go types on the Host's.
//
// The terminal host, the one bin/whim runs with, is package term.  vim's own
// printf is the library's (format.go): it needs no operating system.
package editor

// Host is what the core needs of the world it runs in: a terminal, a clock,
// input with a timeout, the signals, output and an exit.
type Host interface {
	// Init starts catching the signals the editor handles.  deathtrap is the
	// core's handler for SIGHUP and SIGTERM: the host calls it, on the
	// goroutine running the core, where the C handler would have run.
	Init(deathtrap func(sig int32))
	// WinSize is the terminal's size, and false when it has none.
	WinSize() (rows, cols int32, ok bool)
	// TermStart and TermStop put the terminal in raw mode and take it out.
	TermStart()
	TermStop()
	// TTYKeys is the erase and interrupt characters of the terminal on fd,
	// and whether it maps CR to NL on input and NL to CR-NL on output.
	TTYKeys(fd int32) (erase, intr int32, icrnl, onlcr, ok bool)
	// NowMs is milliseconds since the first call; Time is the Unix time.
	NowMs() int64
	Time() int64
	// Delay sleeps ms milliseconds; interruptible lets the terminal relax
	// during a long one.
	Delay(ms int64, interruptible bool)
	// WaitForInput waits up to ms milliseconds (for ever when negative) and
	// reports whether input -- or a signal the editor reads as input -- is
	// there.
	WaitForInput(ms int64) bool
	// ReadInput reads into buf: the count, 0 at the end of input, -1 when
	// a signal came first.  A signal the editor reads as input is written as
	// the key sequence that stands for it.
	ReadInput(buf []byte) int32
	// Raise sends the process sig; Suspend stops it (SIGTSTP).
	Raise(sig int32)
	Suspend()
	// Exit ends the editor with code, and does not return: the terminal
	// host ends the process; a host embedding the editor panics with
	// Exit(code), which Main recovers and returns.
	Exit(code int32)
	// Message writes a message, to the error stream when err; Write writes
	// the screen's output and returns the count, or -1.
	Message(msg []byte, err bool)
	Write(p []byte) int32
}

// host is the Host of the editor running in this process: one, as in the C.
var host Host

// Exit is what a Host that must not end the process panics with in its Exit:
// Main recovers it and returns the code.
type Exit int32

// Main runs the editor on h with the command line args, args[0] the program's
// name, and returns its exit status: vim_main's, or the code of an Exit
// panic from h.
func Main(h Host, args []string) (status int) {
	defer func() {
		if r := recover(); r != nil {
			code, ok := r.(Exit)
			if !ok {
				panic(r)
			}
			status = int(code)
		}
	}()
	host = h
	argv := make([]Ptr[byte], len(args)+1)
	for i, a := range args {
		argv[i] = View([]byte(a + "\x00"))
	}
	return int(vim_main(int32(len(args)), argv))
}

// The glue: the host functions the core calls, in the C's signatures.

func musl_host_init() { host.Init(deathtrap) }

func musl_get_winsize(rows *int32, cols *int32) int32 {
	r, c, ok := host.WinSize()
	if !ok {
		return FAIL
	}
	*rows, *cols = r, c
	return OK
}

func musl_term_start() { host.TermStart() }

func musl_term_stop() { host.TermStop() }

func musl_tty_keys(fd int32, bs *int32, intr *int32, cr *int32, nlcr *int32) int32 {
	e, i, icrnl, onlcr, ok := host.TTYKeys(fd)
	if !ok {
		return FAIL
	}
	*bs, *intr, *cr, *nlcr = e, i, B2i(icrnl), B2i(onlcr)
	return OK
}

func musl_now_ms() int64 { return host.NowMs() }

func host_time() int64 { return host.Time() }

func musl_delay(ms int64, interruptible int32) { host.Delay(ms, interruptible != 0) }

func musl_wait_for_input(ms int64) int32 { return B2i(host.WaitForInput(ms)) }

// musl_read_input hands the host no room for a negative length and answers
// -1 for it, as read(2) of (size_t)len does; the host still takes the signals
// it reads as input, as the C did before its read.
func musl_read_input(buf Ptr[byte], len_ int32) int32 {
	n := host.ReadInput(buf.Slice(int(max(len_, 0))))
	if len_ < 0 {
		return -1
	}
	return n
}

func host_raise(sig int32) { host.Raise(sig) }

func musl_suspend() { host.Suspend() }

func host_exit(r int32) { host.Exit(r) }

func host_message(msg Ptr[byte], len_ int32, err int32) {
	n := len_
	if n < 0 {
		n = int32(hostStrlen(msg))
	}
	host.Message(msg.Slice(int(n)), err != 0)
}

func host_write(s Ptr[byte], len_ int32) int32 {
	if len_ < 0 {
		return -1
	}
	if len_ == 0 {
		return 0
	}
	return host.Write(s.Slice(int(len_)))
}

// The C host allocates from a static 1 GiB arena and never frees; the
// garbage collector is the allocator here, but the arena's accounting (and
// its exhaustion message) are kept.  They are the core's, not a Host's.
const HOST_ARENA_BYTES = 1024 * 1024 * 1024

var host_arena_used usize

func host_arena_exhausted(n usize) {
	m := "whim-vim: host arena exhausted: " + hostUtoa(HOST_ARENA_BYTES) + " bytes, " +
		hostUtoa(host_arena_used) + " used, request " + hostUtoa(n) + "\n"
	b := Mk[byte](len(m) + 1)
	copy(b.Slice(len(m)), m)
	host_message(b, int32(len(m)), TRUE)
	host_exit(1)
}

func hostUtoa(v usize) string {
	var d [24]byte
	i := 24
	if v == 0 {
		i--
		d[i] = '0'
	}
	for v > 0 {
		i--
		d[i] = byte('0' + v%10)
		v /= 10
	}
	return string(d[i:])
}

// host_alloc returns n zeroed bytes, a Ptr[byte] as `any`.
func host_alloc(n usize) any {
	want := (n + 15) &^ usize(15) // alignof(max_align_t) is 16
	if want < n || want > HOST_ARENA_BYTES-host_arena_used {
		host_arena_exhausted(n)
	}
	host_arena_used += want
	return Alloc(int(n))
}
