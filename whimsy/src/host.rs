//! The host: what the editor core asks of the world it runs in, behind a
//! trait.  In whim-vim.c the host is everything from the first `#include`
//! to the end of the file, and the core calls it by name -- `host_write`,
//! `musl_read_input` and the rest.  Here the core (`editor`, generated)
//! still calls those names, `crate::host::host_write(ed, ...)`, and each is
//! a line of glue to the `Host` its editor runs on: the C signature on this
//! side, Rust's types on the trait's (editor/host.go's Host, caprice's
//! Caprice.Host).  What is the editor's and not the Host's -- the arena,
//! which the C host keeps as the core's memory -- is kept here, per editor.
//! So a process holds any number of editors, each on its own Host: the
//! terminal (`crate::term`) or anything else.

use crate::editor::{self, Editor};
use crate::rt::VArg;
use core::ffi::c_void;
use std::panic::{self, AssertUnwindSafe};
use std::rc::Rc;
use std::sync::atomic::{AtomicUsize, Ordering};

/// What the core needs of the world it runs in: a terminal, a clock, input
/// with a timeout, the signals, output.  Every method takes `&self`: the
/// core may call the host again from inside a call of it (a SIGHUP's
/// deathtrap, run where the host waits), so a host keeps its state in
/// cells.
pub trait Host {
    /// Start catching the signals the editor handles; `deathtrap` is the
    /// core's handler for SIGHUP and SIGTERM, which the host calls on the
    /// editor's thread, where the C handler would have run.
    fn init(&self, deathtrap: Rc<dyn Fn(i32)>);
    /// The terminal's rows and columns, when it has them.
    fn win_size(&self) -> Option<(i32, i32)>;
    /// Raw mode, and out of it.
    fn term_start(&self);
    fn term_stop(&self);
    /// fd's erase and interrupt characters, and whether it maps CR to NL on
    /// input and NL to CR NL on output, when fd is a terminal.
    fn tty_keys(&self, fd: i32) -> Option<(i32, i32, bool, bool)>;
    /// Milliseconds of a clock that starts at the first call, and the Unix
    /// time.
    fn now_ms(&self) -> i64;
    fn time(&self) -> i64;
    /// Sleep ms milliseconds; interruptible lets the terminal relax meanwhile.
    fn delay(&self, ms: i64, interruptible: bool);
    /// Whether input -- or a signal the core reads as input -- is there
    /// within ms milliseconds (for ever when negative).
    fn wait_for_input(&self, ms: i64) -> bool;
    /// Up to buf.len() bytes of input: the count, 0 at its end, -1 when a
    /// signal came first.  A signal the core reads as input is written as
    /// the keys that stand for it.
    fn read_input(&self, buf: &mut [u8]) -> i32;
    /// Raise sig in this editor's process, and suspend it.
    fn raise(&self, sig: i32);
    fn suspend(&self);
    /// A message outside the screen, to the error stream when err.
    fn message(&self, msg: &[u8], err: bool);
    /// The screen's output: the count written, or -1.
    fn write(&self, p: &[u8]) -> i32;
}

/// The C host's arena: 1 GiB, zeroed, never freed -- allocated lazily, so an
/// editor that uses little costs little.
pub const HOST_ARENA_BYTES: usize = 1024 * 1024 * 1024;

/// What an editor's `host` field points at: its Host, and its arena.
pub struct Glue {
    host: Box<dyn Host>,
    arena: *mut u8,
    used: AtomicUsize,
}

/// host_exit: the editor ends with this status, which `run` catches -- the
/// C's longjmp back to main.
pub struct HostExit(pub i32);

fn arena_layout() -> std::alloc::Layout {
    std::alloc::Layout::from_size_align(HOST_ARENA_BYTES, 16).unwrap()
}

impl Glue {
    fn new(host: Box<dyn Host>) -> Glue {
        let arena = unsafe { std::alloc::alloc_zeroed(arena_layout()) };
        if arena.is_null() {
            std::alloc::handle_alloc_error(arena_layout());
        }
        Glue { host, arena, used: AtomicUsize::new(0) }
    }
}

impl Drop for Glue {
    fn drop(&mut self) {
        unsafe { std::alloc::dealloc(self.arena, arena_layout()) }
    }
}

/// The editor's glue.
#[inline]
unsafe fn glue<'a>(ed: *mut Editor) -> &'a Glue {
    &*((*ed).host as *const Glue)
}

#[inline]
unsafe fn host<'a>(ed: *mut Editor) -> &'a dyn Host {
    &*glue(ed).host
}

/// Runs an editor on `host` with the command line `args` (args[0] the
/// program's name) to its end, and returns its exit status: vim_main's, or
/// host_exit's.  The editor is freed after; another may run meanwhile, on a
/// thread of its own.
pub fn run(host: Box<dyn Host>, args: &[Vec<u8>]) -> i32 {
    let mut strs: Vec<Vec<u8>> = args
        .iter()
        .map(|a| {
            let mut v = a.clone();
            v.push(0);
            v
        })
        .collect();
    let mut argv: Vec<*mut i8> = strs.iter_mut().map(|s| s.as_mut_ptr() as *mut i8).collect();
    argv.push(core::ptr::null_mut());
    let ed = editor::new_editor();
    let g = Box::into_raw(Box::new(Glue::new(host)));
    unsafe { (*ed).host = g as *mut c_void };
    let r = panic::catch_unwind(AssertUnwindSafe(|| unsafe { editor::vim_main(ed, args.len() as i32, argv.as_mut_ptr()) }));
    unsafe {
        drop(Box::from_raw(g));
        drop(Box::from_raw(ed));
    }
    drop(strs);
    match r {
        Ok(code) => code,
        Err(p) => match p.downcast_ref::<HostExit>() {
            Some(HostExit(code)) => *code,
            None => panic::resume_unwind(p),
        },
    }
}

// The glue: the host functions the core calls, in the C's signatures.

pub unsafe fn musl_host_init(ed: *mut Editor) {
    let e = ed as usize;
    host(ed).init(Rc::new(move |sig| unsafe { editor::deathtrap(e as *mut Editor, sig) }));
}

pub unsafe fn musl_get_winsize(ed: *mut Editor, rows: *mut i32, cols: *mut i32) -> i32 {
    match host(ed).win_size() {
        Some((r, c)) => {
            *rows = r;
            *cols = c;
            editor::OK
        }
        None => editor::FAIL,
    }
}

pub unsafe fn musl_term_start(ed: *mut Editor) {
    host(ed).term_start()
}

pub unsafe fn musl_term_stop(ed: *mut Editor) {
    host(ed).term_stop()
}

pub unsafe fn musl_tty_keys(ed: *mut Editor, fd: i32, bs: *mut i32, intr: *mut i32, cr: *mut i32, nlcr: *mut i32) -> i32 {
    match host(ed).tty_keys(fd) {
        Some((e, i, icrnl, onlcr)) => {
            *bs = e;
            *intr = i;
            *cr = icrnl as i32;
            *nlcr = onlcr as i32;
            editor::OK
        }
        None => editor::FAIL,
    }
}

pub unsafe fn musl_now_ms(ed: *mut Editor) -> i64 {
    host(ed).now_ms()
}

pub unsafe fn host_time(ed: *mut Editor) -> i64 {
    host(ed).time()
}

pub unsafe fn musl_delay(ed: *mut Editor, ms: i64, interruptible: i32) {
    host(ed).delay(ms, interruptible != 0)
}

pub unsafe fn musl_wait_for_input(ed: *mut Editor, ms: i64) -> i32 {
    host(ed).wait_for_input(ms) as i32
}

/// The host is handed no room for a negative length, and the answer is -1
/// for it, as read(2) of (size_t)len gives; the host still takes the
/// signals it reads as input, as the C does before its read.
pub unsafe fn musl_read_input(ed: *mut Editor, buf: *mut i8, len: i32) -> i32 {
    let n = host(ed).read_input(std::slice::from_raw_parts_mut(buf as *mut u8, len.max(0) as usize));
    if len < 0 {
        return -1;
    }
    n
}

pub unsafe fn musl_suspend(ed: *mut Editor) {
    host(ed).suspend()
}

pub unsafe fn host_raise(ed: *mut Editor, sig: i32) {
    host(ed).raise(sig)
}

/// The editor ends: unwound to `run`, which returns the code.
pub unsafe fn host_exit(_ed: *mut Editor, r: i32) {
    panic::resume_unwind(Box::new(HostExit(r)))
}

unsafe fn strlen(s: *const u8) -> usize {
    let mut n = 0;
    while *s.add(n) != 0 {
        n += 1;
    }
    n
}

pub unsafe fn host_message(ed: *mut Editor, msg: *mut i8, len: i32, err: i32) {
    let n = if len < 0 { strlen(msg as *const u8) } else { len as usize };
    host(ed).message(std::slice::from_raw_parts(msg as *const u8, n), err != 0)
}

pub unsafe fn host_write(ed: *mut Editor, s: *mut i8, len: i32) -> i32 {
    if len < 0 {
        return -1;
    }
    if len == 0 {
        return 0;
    }
    host(ed).write(std::slice::from_raw_parts(s as *const u8, len as usize))
}

/// n zeroed bytes of the editor's arena, aligned as max_align_t is (16);
/// counted atomically, since the regex engine's chunks allocate at once.
pub unsafe fn host_alloc(ed: *mut Editor, n: u64) -> *mut c_void {
    let g = glue(ed);
    let n = n as usize;
    let want = n.wrapping_add(15) & !15;
    let used = g.used.fetch_add(want, Ordering::Relaxed);
    if want < n || want > HOST_ARENA_BYTES - used.min(HOST_ARENA_BYTES) {
        arena_exhausted(ed, n, used);
    }
    g.arena.add(used) as *mut c_void
}

unsafe fn arena_exhausted(ed: *mut Editor, n: usize, used: usize) {
    let m = format!("whim-vim: host arena exhausted: {} bytes, {} used, request {}\n", HOST_ARENA_BYTES, used, n);
    host(ed).message(m.as_bytes(), true);
    host_exit(ed, 1);
}

pub unsafe fn vim_snprintf(ed: *mut Editor, str: *mut i8, str_m: u64, fmt: *mut i8, args: &[VArg]) -> i32 {
    crate::printf::vim_snprintf(ed, str, str_m, fmt, args)
}
