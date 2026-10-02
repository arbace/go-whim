//! The terminal host: a `crate::host::Host` on the process's own file
//! descriptors 0, 1 and 2 -- the Rust port of the half of whim-vim.c's host
//! that asks the operating system for something, from `host_winch_pending`
//! to `host_write`: `host_catch` and the four signal handlers,
//! `host_deliver_death`, `host_tty_set`, `musl_host_init`,
//! `musl_get_winsize`, `musl_term_start`/`stop`, `musl_tty_keys`,
//! `musl_now_ms`, `host_time`, `musl_delay`, `musl_wait_for_input`,
//! `musl_read_input`, `host_raise`, `musl_suspend`, `host_message` and
//! `host_write`, function by function, as editor/term/term.go and
//! caprice/host/Caprice/Term.hs port them.  It is the Host bin/whimsy runs
//! the editor with (`host::run(Box::new(term::new()), &args)`).
//!
//! Unlike the Go and Haskell ports, nothing stands between the editor and
//! the signals: the handlers are real, installed with sigaction(2) as the C
//! installs them (no SA_RESTART, so a caught signal interrupts select(2),
//! read(2) and nanosleep(2) with EINTR, as in the C), and the libc calls are
//! the C's own -- tcgetattr, tcsetattr, ioctl, pipe2, select, read, write,
//! nanosleep, gettimeofday, time, kill, getpid -- declared here, `extern
//! "C"`, against std's libc (musl on x86_64; every struct declared is laid
//! out as musl's, which on x86_64 is glibc's too, sizes checked with gcc).
//!
//! The handlers store to statics (atomics, which are async-signal-safe) and
//! the SIGHUP/SIGTERM one writes one byte to the non-blocking, close-on-exec
//! pipe, exactly as the C's do.  Signal dispositions are the process's, so
//! the pending flags and the pipe are statics, as they are in the C: one
//! process has one terminal.  What is the instance's -- the saved terminal
//! modes, raw or not, the clock's base, the core's deathtrap -- is kept in
//! cells, since every method takes `&self`.
//!
//! Deviations from the C are marked DEVIATION where they happen:
//!
//!   - SIGHUP and SIGTERM: none in substance.  The C's handler too only
//!     records the signal, and `host_deliver_death` calls deathtrap on the
//!     editor's stack where the host waits, reads or sleeps; here the call
//!     is of the closure `init` was handed, never from inside the handler.
//!   - `host_time`'s `atol` is spelled out (the same digits read).
//!   - The size report a SIGWINCH is read as is formatted by Rust's
//!     `format!` where the C calls vim_snprintf: the same decimal digits.
//!   - `musl_host_init` run a second time (another Term in one process)
//!     closes the pipe it made the first time, where the C would leak it.

use crate::host::Host;
use std::cell::{Cell, RefCell};
use std::rc::Rc;
use std::sync::atomic::{AtomicBool, AtomicI32, Ordering};

// The libc this file calls, as musl declares it on x86_64.

/// struct termios: musl's (and glibc's) on x86_64, 60 bytes.
#[repr(C)]
#[derive(Clone, Copy)]
struct Termios {
    c_iflag: u32,
    c_oflag: u32,
    c_cflag: u32,
    c_lflag: u32,
    c_line: u8,
    c_cc: [u8; 32],
    c_ispeed: u32,
    c_ospeed: u32,
}

/// struct sigaction: 152 bytes, the mask at 8, the flags at 136.
#[repr(C)]
struct SigAction {
    sa_handler: usize,
    sa_mask: [u64; 16],
    sa_flags: i32,
    sa_restorer: usize,
}

#[repr(C)]
struct Timeval {
    tv_sec: i64,
    tv_usec: i64,
}

#[repr(C)]
struct Timespec {
    tv_sec: i64,
    tv_nsec: i64,
}

#[repr(C)]
#[derive(Default)]
struct Winsize {
    ws_row: u16,
    ws_col: u16,
    ws_xpixel: u16,
    ws_ypixel: u16,
}

/// fd_set: 1024 bits.
#[repr(C)]
struct FdSet {
    bits: [u64; 16],
}

impl FdSet {
    fn zero() -> FdSet {
        FdSet { bits: [0; 16] }
    }
    fn set(&mut self, fd: i32) {
        self.bits[fd as usize / 64] |= 1u64 << (fd as usize % 64);
    }
    fn is_set(&self, fd: i32) -> bool {
        self.bits[fd as usize / 64] & (1u64 << (fd as usize % 64)) != 0
    }
}

extern "C" {
    fn sigaction(sig: i32, act: *const SigAction, old: *mut SigAction) -> i32;
    fn tcgetattr(fd: i32, t: *mut Termios) -> i32;
    fn tcsetattr(fd: i32, act: i32, t: *const Termios) -> i32;
    fn ioctl(fd: i32, req: i32, ...) -> i32;
    fn pipe2(fds: *mut i32, flags: i32) -> i32;
    fn close(fd: i32) -> i32;
    fn select(n: i32, r: *mut FdSet, w: *mut FdSet, e: *mut FdSet, tv: *mut Timeval) -> i32;
    fn read(fd: i32, buf: *mut u8, n: usize) -> isize;
    fn write(fd: i32, buf: *const u8, n: usize) -> isize;
    fn nanosleep(req: *const Timespec, rem: *mut Timespec) -> i32;
    fn gettimeofday(tv: *mut Timeval, tz: *mut u8) -> i32;
    fn time(t: *mut i64) -> i64;
    fn kill(pid: i32, sig: i32) -> i32;
    fn getpid() -> i32;
    fn __errno_location() -> *mut i32;
}

const SIGHUP: i32 = 1;
const SIGINT: i32 = 2;
const SIGPIPE: i32 = 13;
const SIGALRM: i32 = 14;
const SIGTERM: i32 = 15;
const SIGCONT: i32 = 18;
const SIGTSTP: i32 = 20;
const SIGWINCH: i32 = 28;
const SIG_DFL: usize = 0;
const SIG_IGN: usize = 1;

const ICRNL: u32 = 0o400;
const IXON: u32 = 0o2000;
const ISIG: u32 = 0o1;
const ICANON: u32 = 0o2;
const ECHO: u32 = 0o10;
const ECHOE: u32 = 0o20;
const IEXTEN: u32 = 0o100000;
const ONLCR: u32 = 0o4;
const XTABS: u32 = 0o14000;
const VINTR: usize = 0;
const VERASE: usize = 2;
const VTIME: usize = 5;
const VMIN: usize = 6;
const TCSANOW: i32 = 0;
const TIOCGWINSZ: i32 = 0x5413;
const O_NONBLOCK: i32 = 0o4000;
const O_CLOEXEC: i32 = 0o2000000;
const EINTR: i32 = 4;

fn errno() -> i32 {
    std::io::Error::last_os_error().raw_os_error().unwrap_or(0)
}

// What the C keeps at file scope and its handlers touch.

static HOST_WINCH_PENDING: AtomicBool = AtomicBool::new(false);
static HOST_TSTP_PENDING: AtomicBool = AtomicBool::new(false);
static HOST_INT_PENDING: AtomicBool = AtomicBool::new(false);
static HOST_DEATH_PENDING: AtomicI32 = AtomicI32::new(0);
static HOST_DEATH_PIPE: [AtomicI32; 2] = [AtomicI32::new(-1), AtomicI32::new(-1)];

/// host_catch: sig handled by f, the mask empty, no flags.
fn host_catch(sig: i32, f: usize) {
    let sa = SigAction { sa_handler: f, sa_mask: [0; 16], sa_flags: 0, sa_restorer: 0 };
    unsafe {
        sigaction(sig, &sa, core::ptr::null_mut());
    }
}

extern "C" fn host_on_winch(_sig: i32) {
    HOST_WINCH_PENDING.store(true, Ordering::SeqCst);
}

extern "C" fn host_on_tstp(_sig: i32) {
    HOST_TSTP_PENDING.store(true, Ordering::SeqCst);
}

extern "C" fn host_on_int(_sig: i32) {
    HOST_INT_PENDING.store(true, Ordering::SeqCst);
}

/// SIGHUP and SIGTERM: the signal recorded and a byte down the pipe, errno
/// kept as it was.
extern "C" fn host_on_death(sig: i32) {
    unsafe {
        let e = *__errno_location();
        HOST_DEATH_PENDING.store(sig, Ordering::SeqCst);
        let w = HOST_DEATH_PIPE[1].load(Ordering::SeqCst);
        if w >= 0 {
            write(w, b"\0".as_ptr(), 1);
        }
        *__errno_location() = e;
    }
}

fn handler(f: extern "C" fn(i32)) -> usize {
    f as usize
}

/// The terminal host.  `new` makes it; the editor's `init` starts catching
/// the signals.
pub struct Term {
    tty_saved: Cell<Termios>,
    tty_valid: Cell<bool>,
    tty_raw: Cell<bool>,
    now_base: Cell<i64>,
    now_based: Cell<bool>,
    deathtrap: RefCell<Option<Rc<dyn Fn(i32)>>>,
}

/// The terminal host, not yet catching signals.
pub fn new() -> Term {
    Term {
        tty_saved: Cell::new(Termios { c_iflag: 0, c_oflag: 0, c_cflag: 0, c_lflag: 0, c_line: 0, c_cc: [0; 32], c_ispeed: 0, c_ospeed: 0 }),
        tty_valid: Cell::new(false),
        tty_raw: Cell::new(false),
        now_base: Cell::new(0),
        now_based: Cell::new(false),
        deathtrap: RefCell::new(None),
    }
}

impl Term {
    /// host_deliver_death: the pipe drained, then a pending SIGHUP or
    /// SIGTERM handed to the core's deathtrap, here on the editor's stack.
    fn deliver_death(&self) {
        let r = HOST_DEATH_PIPE[0].load(Ordering::SeqCst);
        if r >= 0 {
            let mut b = [0u8; 16];
            while unsafe { read(r, b.as_mut_ptr(), b.len()) } > 0 {}
        }
        let sig = HOST_DEATH_PENDING.load(Ordering::SeqCst);
        if sig != 0 {
            HOST_DEATH_PENDING.store(0, Ordering::SeqCst);
            // Cloned out of the cell first: deathtrap may enter the host
            // again, and does not return when it ends the editor.
            let f = self.deathtrap.borrow().clone();
            if let Some(f) = f {
                f(sig);
            }
        }
    }

    /// host_tty_set: fd 0's saved modes, made raw, or relaxed for a sleep
    /// (no canonical input, no echo), or as saved.
    fn tty_set(&self, raw: bool, sleep: bool) {
        let mut n = 10;
        if !self.tty_valid.get() {
            let mut t = self.tty_saved.get();
            if unsafe { tcgetattr(0, &mut t) } == -1 {
                return;
            }
            self.tty_saved.set(t);
            self.tty_valid.set(true);
        }
        let mut tnew = self.tty_saved.get();
        if raw {
            tnew.c_iflag &= !(ICRNL | IXON);
            tnew.c_lflag &= !(ICANON | ECHO | ISIG | ECHOE | IEXTEN);
            tnew.c_oflag &= !(ONLCR | XTABS);
            tnew.c_cc[VMIN] = 1;
            tnew.c_cc[VTIME] = 0;
        } else if sleep {
            tnew.c_lflag &= !(ICANON | ECHO);
            tnew.c_cc[VMIN] = 1;
            tnew.c_cc[VTIME] = 0;
        }
        while unsafe { tcsetattr(0, TCSANOW, &tnew) } == -1 && errno() == EINTR && n > 0 {
            n -= 1;
        }
    }
}

/// atol: white space skipped, a sign, the digits there are.
fn atol(s: &[u8]) -> i64 {
    let mut i = 0;
    while i < s.len() && (s[i] == b' ' || (b'\t'..=b'\r').contains(&s[i])) {
        i += 1;
    }
    let mut neg = false;
    if i < s.len() && (s[i] == b'-' || s[i] == b'+') {
        neg = s[i] == b'-';
        i += 1;
    }
    let mut n: i64 = 0;
    while i < s.len() && s[i].is_ascii_digit() {
        n = n.wrapping_mul(10).wrapping_sub((s[i] - b'0') as i64);
        i += 1;
    }
    if neg {
        n
    } else {
        n.wrapping_neg()
    }
}

impl Host for Term {
    /// musl_host_init: the pipe, and the handlers.
    fn init(&self, deathtrap: Rc<dyn Fn(i32)>) {
        *self.deathtrap.borrow_mut() = Some(deathtrap);
        let mut fds = [-1i32; 2];
        if unsafe { pipe2(fds.as_mut_ptr(), O_NONBLOCK | O_CLOEXEC) } != 0 {
            fds = [-1, -1];
        }
        for (slot, fd) in HOST_DEATH_PIPE.iter().zip(fds) {
            let old = slot.swap(fd, Ordering::SeqCst);
            if old >= 0 {
                // DEVIATION: a second init's pipe replaces the first, which
                // is closed; the C makes one and would leak the first.
                unsafe {
                    close(old);
                }
            }
        }
        host_catch(SIGHUP, handler(host_on_death));
        host_catch(SIGTERM, handler(host_on_death));
        host_catch(SIGWINCH, handler(host_on_winch));
        host_catch(SIGCONT, handler(host_on_winch));
        host_catch(SIGTSTP, handler(host_on_tstp));
        host_catch(SIGINT, handler(host_on_int));
        host_catch(SIGPIPE, SIG_IGN);
        host_catch(SIGALRM, SIG_IGN);
    }

    /// musl_get_winsize: TIOCGWINSZ on fd 1.
    fn win_size(&self) -> Option<(i32, i32)> {
        let mut ws = Winsize::default();
        if unsafe { ioctl(1, TIOCGWINSZ, &mut ws as *mut Winsize) } != 0 {
            return None;
        }
        if ws.ws_row == 0 || ws.ws_col == 0 {
            return None;
        }
        Some((ws.ws_row as i32, ws.ws_col as i32))
    }

    fn term_start(&self) {
        self.tty_raw.set(true);
        self.tty_set(true, false);
    }

    fn term_stop(&self) {
        self.tty_raw.set(false);
        self.tty_set(false, false);
    }

    /// musl_tty_keys: fd's erase and interrupt characters, ICRNL and ONLCR.
    fn tty_keys(&self, fd: i32) -> Option<(i32, i32, bool, bool)> {
        let mut keys = self.tty_saved.get();
        if unsafe { tcgetattr(fd, &mut keys) } == -1 {
            return None;
        }
        Some((keys.c_cc[VERASE] as i32, keys.c_cc[VINTR] as i32, keys.c_iflag & ICRNL != 0, keys.c_oflag & ONLCR != 0))
    }

    /// musl_now_ms: gettimeofday's milliseconds from the first call's second.
    fn now_ms(&self) -> i64 {
        let mut tv = Timeval { tv_sec: 0, tv_usec: 0 };
        unsafe {
            gettimeofday(&mut tv, core::ptr::null_mut());
        }
        if !self.now_based.get() {
            self.now_based.set(true);
            self.now_base.set(tv.tv_sec);
        }
        (tv.tv_sec - self.now_base.get()) * 1000 + tv.tv_usec / 1000
    }

    /// host_time: WHIM_TIME when set and not empty (a clock held still,
    /// phase 180), read as atol reads it; time(2) otherwise.
    fn time(&self) -> i64 {
        if let Some(pinned) = std::env::var_os("WHIM_TIME") {
            use std::os::unix::ffi::OsStrExt;
            let b = pinned.as_bytes();
            // getenv's string ends at its first NUL, which an environment
            // string cannot hold anyway.
            if !b.is_empty() {
                // DEVIATION: atol spelled out; the same digits read.
                return atol(b);
            }
        }
        unsafe { time(core::ptr::null_mut()) }
    }

    /// musl_delay: nanosleep, a raw terminal relaxed for a long
    /// interruptible one.  A caught signal ends it early (EINTR), as in the
    /// C; a negative time does not sleep (EINVAL).
    fn delay(&self, ms: i64, interruptible: bool) {
        let relax = interruptible && self.tty_raw.get() && ms > 500;
        if relax {
            self.tty_set(false, true);
        }
        let ts = Timespec { tv_sec: ms / 1000, tv_nsec: (ms % 1000) * 1_000_000 };
        unsafe {
            nanosleep(&ts, core::ptr::null_mut());
        }
        if relax {
            self.tty_set(true, false);
        }
    }

    /// musl_wait_for_input: select on fd 0 and the pipe, for ms or for ever;
    /// a pending SIGHUP/SIGTERM delivered first, a pending SIGWINCH, SIGTSTP
    /// or SIGINT an answer.  Linux's select leaves the time remaining in tv,
    /// so a wait resumed after EINTR waits only what is left, as the C's.
    fn wait_for_input(&self, ms: i64) -> bool {
        let mut tv = Timeval { tv_sec: 0, tv_usec: 0 };
        let tvp: *mut Timeval = if ms >= 0 {
            tv.tv_sec = ms / 1000;
            tv.tv_usec = (ms % 1000) * 1000;
            &mut tv
        } else {
            core::ptr::null_mut()
        };
        loop {
            self.deliver_death();
            if HOST_WINCH_PENDING.load(Ordering::SeqCst) || HOST_TSTP_PENDING.load(Ordering::SeqCst) || HOST_INT_PENDING.load(Ordering::SeqCst) {
                return true;
            }
            let p = HOST_DEATH_PIPE[0].load(Ordering::SeqCst);
            let mut rfds = FdSet::zero();
            rfds.set(0);
            if p >= 0 {
                rfds.set(p);
            }
            let ret = unsafe { select(if p >= 0 { p + 1 } else { 1 }, &mut rfds, core::ptr::null_mut(), core::ptr::null_mut(), tvp) };
            if ret == -1 && errno() == EINTR {
                continue;
            }
            if ret > 0 && p >= 0 && rfds.is_set(p) {
                continue;
            }
            return ret > 0 && rfds.is_set(0);
        }
    }

    /// musl_read_input: the signals the core reads as input first -- SIGINT
    /// as Ctrl-C, SIGWINCH as the size report, SIGTSTP as the suspend key --
    /// then read(0), which a caught signal interrupts (-1, EINTR) as in the C.
    fn read_input(&self, buf: &mut [u8]) -> i32 {
        self.deliver_death();
        let len = buf.len();
        if HOST_INT_PENDING.load(Ordering::SeqCst) {
            HOST_INT_PENDING.store(false, Ordering::SeqCst);
            if len >= 1 {
                buf[0] = 3;
                return 1;
            }
        }
        if HOST_WINCH_PENDING.load(Ordering::SeqCst) {
            HOST_WINCH_PENDING.store(false, Ordering::SeqCst);
            if let Some((rows, cols)) = self.win_size() {
                if len >= 32 {
                    // DEVIATION: format! where the C calls vim_snprintf; the
                    // report (at most 22 bytes) fits, so both write it whole.
                    let s = format!("\x1b[48;{};{};0;0t", rows, cols);
                    buf[..s.len()].copy_from_slice(s.as_bytes());
                    return s.len() as i32;
                }
            }
        }
        if HOST_TSTP_PENDING.load(Ordering::SeqCst) {
            HOST_TSTP_PENDING.store(false, Ordering::SeqCst);
            if len >= 5 {
                buf[..5].copy_from_slice(b"\x1b[?1z");
                return 5;
            }
        }
        unsafe { read(0, buf.as_mut_ptr(), len) as i32 }
    }

    /// host_raise: kill(getpid(), sig) -- the installed handler runs.
    fn raise(&self, sig: i32) {
        unsafe {
            kill(getpid(), sig);
        }
    }

    /// musl_suspend: SIGTSTP at its default action to the process group,
    /// the handler back after.
    fn suspend(&self) {
        host_catch(SIGTSTP, SIG_DFL);
        unsafe {
            kill(0, SIGTSTP);
        }
        host_catch(SIGTSTP, handler(host_on_tstp));
    }

    /// host_message: all of msg to fd 2 when err, else fd 1, until a write
    /// writes nothing or fails.
    fn message(&self, msg: &[u8], err: bool) {
        let fd = if err { 2 } else { 1 };
        let mut off = 0;
        while off < msg.len() {
            let w = unsafe { write(fd, msg[off..].as_ptr(), msg.len() - off) };
            if w <= 0 {
                return;
            }
            off += w as usize;
        }
    }

    /// host_write: one write(1).
    fn write(&self, p: &[u8]) -> i32 {
        unsafe { write(1, p.as_ptr(), p.len()) as i32 }
    }
}
