// Package term is the editor's host on a terminal: the Go port of the part of
// whim-vim.c from its first #include to its end that asks the operating
// system for something -- the terminal, the clock, input with a timeout, the
// signals, output and the exit.  It is the Host bin/whim runs the editor with
// (editor.Main(term.New(), os.Args)).
//
// Deviations from the C are marked DEVIATION where they happen; the
// important ones are about signals, which Go cannot run asynchronously on the
// goroutine running the core:
//
//   - The C handlers for SIGWINCH/SIGCONT, SIGTSTP and SIGINT only set a flag.
//     Here a goroutine receives the signal from os/signal and sets the same
//     flag (atomically), then writes one byte to a wake-up pipe so a blocking
//     select in the host returns, as the C select/read/nanosleep returns with
//     EINTR.  The flags are read only by the host, as in the C.
//   - SIGHUP and SIGTERM run the core's deathtrap() in the C, at whatever
//     point the core is.  Here the goroutine queues the signal and deathtrap()
//     runs on the core's goroutine at the next point the host is entered to
//     wait, read or sleep (WaitForInput, ReadInput, Delay).  vim's long
//     operations poll for input (ui_breakcheck), so those points are reached;
//     a core that loops without ever polling would not die of SIGTERM/SIGHUP,
//     where the C would.
//   - Raise of a signal the host catches runs that handler's effect directly
//     (the C kill(getpid()) runs the handler before kill returns).
package term

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// XTABS (TABDLY) is not in package syscall; Linux's value.
const xtabs = 0x1800

// Host is the terminal host.  There is one terminal, so there is one Host a
// process can use; New makes it.
type Host struct {
	winchPending atomic.Bool
	tstpPending  atomic.Bool
	intPending   atomic.Bool
	ttySaved     syscall.Termios
	ttyValid     bool
	ttyRaw       bool
	nowBase      int64
	nowBased     bool

	// the Go replacements for asynchronous handlers
	caught    bool           // Init has run
	sigch     chan os.Signal // signals the host catches
	wakeR     int            // the read end of the wake-up pipe
	wakeW     int            // its write end
	dyingMu   sync.Mutex
	dying     []int32 // SIGHUP/SIGTERM received, for deathtrap
	deathtrap func(sig int32)
}

// New is the terminal host, not yet catching signals (the editor's Init
// does that).
func New() *Host { return &Host{wakeR: -1, wakeW: -1, deathtrap: func(int32) {}} }

func ioctl(fd int, req uintptr, arg unsafe.Pointer) syscall.Errno {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(arg))
	return e
}

func tcgetattr(fd int, t *syscall.Termios) bool {
	return ioctl(fd, syscall.TCGETS, unsafe.Pointer(t)) == 0
}

func (h *Host) ttySet(raw, sleep bool) {
	var tnew syscall.Termios
	n := 10

	if !h.ttyValid {
		if !tcgetattr(0, &h.ttySaved) {
			return
		}
		h.ttyValid = true
	}
	tnew = h.ttySaved
	if raw {
		tnew.Iflag &^= syscall.ICRNL | syscall.IXON
		tnew.Lflag &^= syscall.ICANON | syscall.ECHO | syscall.ISIG | syscall.ECHOE | syscall.IEXTEN
		tnew.Oflag &^= syscall.ONLCR | xtabs
		tnew.Cc[syscall.VMIN] = 1
		tnew.Cc[syscall.VTIME] = 0
	} else if sleep {
		tnew.Lflag &^= syscall.ICANON | syscall.ECHO
		tnew.Cc[syscall.VMIN] = 1
		tnew.Cc[syscall.VTIME] = 0
	}
	for {
		e := ioctl(0, syscall.TCSETS, unsafe.Pointer(&tnew)) // tcsetattr(0, TCSANOW)
		if !(e != 0 && e == syscall.EINTR && n > 0) {
			break
		}
		n--
	}
}

// signals is the goroutine that stands for the C handlers: it sets the flag
// a handler sets (or queues SIGHUP/SIGTERM for deathtrap), then wakes a host
// select that is waiting, as the signal's EINTR would.
func (h *Host) signals() {
	for s := range h.sigch {
		sig := s.(syscall.Signal)
		switch sig {
		case syscall.SIGWINCH, syscall.SIGCONT:
			h.winchPending.Store(true)
		case syscall.SIGTSTP:
			h.tstpPending.Store(true)
		case syscall.SIGINT:
			h.intPending.Store(true)
		case syscall.SIGHUP, syscall.SIGTERM:
			h.dyingMu.Lock()
			h.dying = append(h.dying, int32(sig))
			h.dyingMu.Unlock()
		}
		if h.wakeW >= 0 {
			syscall.Write(h.wakeW, []byte{0})
		}
	}
}

// drain empties the wake-up pipe: the flags, not the pipe, say what arrived;
// the pipe only ends a wait.  Drained before the flags are read, so a signal
// after the read still wakes the wait that follows.
func (h *Host) drain() {
	if h.wakeR < 0 {
		return
	}
	var b [64]byte
	for {
		n, err := syscall.Read(h.wakeR, b[:])
		if n <= 0 || err != nil {
			return
		}
	}
}

// deliver runs deathtrap for each SIGHUP/SIGTERM received, on the core's
// goroutine -- where the C handler would have run at the signal.
func (h *Host) deliver() {
	for {
		h.dyingMu.Lock()
		if len(h.dying) == 0 {
			h.dyingMu.Unlock()
			return
		}
		sig := h.dying[0]
		h.dying = h.dying[1:]
		h.dyingMu.Unlock()
		h.deathtrap(sig)
	}
}

func fdSet(s *syscall.FdSet, fd int) {
	s.Bits[fd/64] |= 1 << (uint(fd) % 64)
}

func fdIsSet(s *syscall.FdSet, fd int) bool {
	return s.Bits[fd/64]&(1<<(uint(fd)%64)) != 0
}

// Init starts catching the signals, with deathtrap the core's handler for
// SIGHUP and SIGTERM.
func (h *Host) Init(deathtrap func(sig int32)) {
	h.deathtrap = deathtrap
	if h.sigch == nil {
		var fds [2]int
		if syscall.Pipe2(fds[:], syscall.O_NONBLOCK|syscall.O_CLOEXEC) == nil {
			h.wakeR, h.wakeW = fds[0], fds[1]
		}
		h.sigch = make(chan os.Signal, 64)
		go h.signals()
	}
	signal.Notify(h.sigch, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGWINCH,
		syscall.SIGCONT, syscall.SIGTSTP, syscall.SIGINT)
	signal.Ignore(syscall.SIGPIPE, syscall.SIGALRM)
	h.caught = true
}

// WinSize is the terminal's size, from TIOCGWINSZ on stdout.
func (h *Host) WinSize() (rows, cols int32, ok bool) {
	var ws struct{ row, col, xpixel, ypixel uint16 }

	if ioctl(1, syscall.TIOCGWINSZ, unsafe.Pointer(&ws)) != 0 {
		return 0, 0, false
	}
	if ws.row <= 0 || ws.col <= 0 {
		return 0, 0, false
	}
	return int32(ws.row), int32(ws.col), true
}

// TermStart puts the terminal in raw mode.
func (h *Host) TermStart() {
	h.ttyRaw = true
	h.ttySet(true, false)
}

// TermStop takes it out.
func (h *Host) TermStop() {
	h.ttyRaw = false
	h.ttySet(false, false)
}

// TTYKeys reads the erase and interrupt characters and the CR/NL mapping.
func (h *Host) TTYKeys(fd int32) (erase, intr int32, icrnl, onlcr, ok bool) {
	var keys syscall.Termios

	if !tcgetattr(int(fd), &keys) {
		return 0, 0, false, false, false
	}
	return int32(keys.Cc[syscall.VERASE]), int32(keys.Cc[syscall.VINTR]),
		keys.Iflag&syscall.ICRNL != 0, keys.Oflag&syscall.ONLCR != 0, true
}

// NowMs is milliseconds since the first call.
func (h *Host) NowMs() int64 {
	t := time.Now()
	sec := t.Unix()
	usec := int64(t.Nanosecond() / 1000)
	if !h.nowBased {
		h.nowBased = true
		h.nowBase = sec
	}
	return (sec-h.nowBase)*1000 + usec/1000
}

// Time is the Unix time.
func (h *Host) Time() int64 {
	return time.Now().Unix()
}

// sleep is nanosleep(ms): it ends early when a caught signal arrives, as
// nanosleep does with EINTR.  A negative time does not sleep (nanosleep's
// EINVAL).
func (h *Host) sleep(ms int64) {
	if ms < 0 {
		return
	}
	if h.wakeR < 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return
	}
	h.drain()
	tv := syscall.Timeval{Sec: ms / 1000, Usec: (ms % 1000) * 1000}
	for {
		var r syscall.FdSet
		fdSet(&r, h.wakeR)
		// Linux's select leaves the time remaining in tv, so an EINTR from
		// the Go runtime's own signals resumes the same sleep.
		_, err := syscall.Select(h.wakeR+1, &r, nil, nil, &tv)
		if err == syscall.EINTR {
			continue
		}
		return
	}
}

// Delay sleeps, relaxing a raw terminal during a long interruptible one.
func (h *Host) Delay(ms int64, interruptible bool) {
	relax := interruptible && h.ttyRaw && ms > 500

	h.deliver()
	if relax {
		h.ttySet(false, true)
	}
	h.sleep(ms)
	if relax {
		h.ttySet(true, false)
	}
	h.deliver()
}

// WaitForInput waits for stdin or a signal the editor reads as input.
func (h *Host) WaitForInput(ms int64) bool {
	var tv syscall.Timeval
	var tvp *syscall.Timeval

	if ms >= 0 {
		tv.Sec = ms / 1000
		tv.Usec = (ms % 1000) * 1000
		tvp = &tv
	}
	for {
		h.drain()
		h.deliver()
		if h.winchPending.Load() || h.tstpPending.Load() || h.intPending.Load() {
			return true
		}
		var rfds syscall.FdSet
		nfd := 1
		fdSet(&rfds, 0)
		if h.wakeR >= 0 {
			fdSet(&rfds, h.wakeR)
			nfd = h.wakeR + 1
		}
		ret, err := syscall.Select(nfd, &rfds, nil, nil, tvp)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			return false
		}
		if ret > 0 && fdIsSet(&rfds, 0) {
			return true
		}
		if ret > 0 {
			// only the wake-up pipe: the C select returned EINTR
			continue
		}
		return false
	}
}

// ReadInput reads stdin, after the signals the editor reads as input:
// SIGINT as Ctrl-C, SIGWINCH as the size report, SIGTSTP as the suspend key.
func (h *Host) ReadInput(buf []byte) int32 {
	h.drain()
	h.deliver()
	if h.intPending.Load() {
		h.intPending.Store(false)
		if len(buf) >= 1 {
			buf[0] = 3
			return 1
		}
	}
	if h.winchPending.Load() {
		h.winchPending.Store(false)
		if rows, cols, ok := h.WinSize(); ok && len(buf) >= 32 {
			return int32(copy(buf, fmt.Sprintf("\033[48;%d;%d;0;0t", rows, cols)))
		}
	}
	if h.tstpPending.Load() {
		h.tstpPending.Store(false)
		if len(buf) >= 5 {
			return int32(copy(buf, "\033[?1z"))
		}
	}
	if len(buf) == 0 {
		return 0
	}
	// DEVIATION: read(0) in the C is interrupted by a caught signal and
	// returns -1 (EINTR).  A Go read would not be, so wait first for either
	// input or the wake-up pipe, and return -1 when the pipe won.
	if h.wakeR >= 0 {
		for {
			var rfds syscall.FdSet
			fdSet(&rfds, 0)
			fdSet(&rfds, h.wakeR)
			ret, err := syscall.Select(h.wakeR+1, &rfds, nil, nil, nil)
			if err == syscall.EINTR {
				continue
			}
			if err == nil && ret > 0 && !fdIsSet(&rfds, 0) {
				h.drain()
				h.deliver()
				return -1
			}
			break
		}
	}
	for {
		n, err := syscall.Read(0, buf)
		if err == syscall.EINTR {
			continue // only the Go runtime's own signals get here
		}
		if err != nil {
			return -1
		}
		return int32(n)
	}
}

// Raise sends the process sig.
func (h *Host) Raise(sig int32) {
	// DEVIATION: the C kill(getpid(), sig) runs the installed handler before
	// kill returns; here a caught signal's handler effect runs directly.
	if h.caught {
		switch syscall.Signal(sig) {
		case syscall.SIGHUP, syscall.SIGTERM:
			h.deathtrap(sig)
			return
		case syscall.SIGWINCH, syscall.SIGCONT:
			h.winchPending.Store(true)
			return
		case syscall.SIGTSTP:
			h.tstpPending.Store(true)
			return
		case syscall.SIGINT:
			h.intPending.Store(true)
			return
		case syscall.SIGPIPE, syscall.SIGALRM:
			return
		}
	}
	syscall.Kill(syscall.Getpid(), syscall.Signal(sig))
}

// Suspend stops the process group with SIGTSTP at its default action.  Go's
// signal.Reset would leave the runtime's handler installed, which swallows
// SIGTSTP, so the kernel action is set to SIG_DFL with rt_sigaction directly
// and the runtime's own restored after.
func (h *Host) Suspend() {
	var old, dfl [4]uint64 // struct kernel_sigaction: handler, flags, restorer, mask

	_, _, e := syscall.RawSyscall6(syscall.SYS_RT_SIGACTION, uintptr(syscall.SIGTSTP), 0, uintptr(unsafe.Pointer(&old)), 8, 0, 0)
	if e != 0 {
		// DEVIATION (fallback only): stop with SIGSTOP instead
		syscall.Kill(0, syscall.SIGSTOP)
		return
	}
	dfl = old
	dfl[0] = 0 // SIG_DFL
	syscall.RawSyscall6(syscall.SYS_RT_SIGACTION, uintptr(syscall.SIGTSTP), uintptr(unsafe.Pointer(&dfl)), 0, 8, 0, 0)
	syscall.Kill(0, syscall.SIGTSTP)
	syscall.RawSyscall6(syscall.SYS_RT_SIGACTION, uintptr(syscall.SIGTSTP), uintptr(unsafe.Pointer(&old)), 0, 8, 0, 0)
}

// Exit is the C longjmp back to main, which returns the code: here the
// process simply exits with it.
func (h *Host) Exit(code int32) {
	os.Exit(int(code))
}

// Message writes msg to stdout, or to stderr when err.
func (h *Host) Message(msg []byte, err bool) {
	fd := 1
	if err {
		fd = 2
	}
	for len(msg) > 0 {
		w, e := syscall.Write(fd, msg)
		if e == syscall.EINTR {
			continue
		}
		if w <= 0 || e != nil {
			return
		}
		msg = msg[w:]
	}
}

// Write writes the screen's output to stdout.
func (h *Host) Write(p []byte) int32 {
	for {
		n, err := syscall.Write(1, p)
		if err == syscall.EINTR {
			continue // the Go runtime's own signals
		}
		if err != nil {
			return -1
		}
		return int32(n)
	}
}
