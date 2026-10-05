//! The C host's 17 functions as the core calls them -- whimsy/src/host.rs's
//! signatures, the editor first -- each the function of the same name in
//! rt.c, which answers it by a hypercall or from the guest's arena.  There
//! is one editor, so the editor argument is not needed; host_exit ends the
//! guest (rt.c's), which is the C's longjmp back to main without one.

use crate::editor::{self, Editor};
use crate::rt::VArg;
use core::ffi::{c_char, c_int, c_long, c_void};

/// What an editor's `host` field points at: nothing, here.
pub struct Glue;

pub mod c {
    use core::ffi::{c_char, c_int, c_long, c_void};
    extern "C" {
        pub fn musl_host_init();
        pub fn musl_get_winsize(rows: *mut c_int, cols: *mut c_int) -> c_int;
        pub fn musl_term_start();
        pub fn musl_term_stop();
        pub fn musl_tty_keys(fd: c_int, bs: *mut c_int, intr: *mut c_int, cr: *mut c_int, nlcr: *mut c_int) -> c_int;
        pub fn musl_now_ms() -> c_long;
        pub fn musl_delay(ms: c_long, interruptible: c_int);
        pub fn musl_wait_for_input(ms: c_long) -> c_int;
        pub fn musl_read_input(buf: *mut c_char, len: c_int) -> c_int;
        pub fn musl_suspend();
        pub fn host_exit(r: c_int) -> !;
        pub fn host_message(msg: *const c_char, len: c_int, err: c_int);
        pub fn host_write(s: *const c_char, len: c_int) -> c_int;
        pub fn host_time() -> c_long;
        pub fn host_raise(sig: c_int);
        pub fn host_alloc(n: usize) -> *mut c_void;
    }
}

pub unsafe fn musl_host_init(_ed: *mut Editor) {
    c::musl_host_init()
}

pub unsafe fn musl_get_winsize(_ed: *mut Editor, rows: *mut i32, cols: *mut i32) -> i32 {
    c::musl_get_winsize(rows, cols)
}

pub unsafe fn musl_term_start(_ed: *mut Editor) {
    c::musl_term_start()
}

pub unsafe fn musl_term_stop(_ed: *mut Editor) {
    c::musl_term_stop()
}

pub unsafe fn musl_tty_keys(_ed: *mut Editor, fd: i32, bs: *mut i32, intr: *mut i32, cr: *mut i32, nlcr: *mut i32) -> i32 {
    c::musl_tty_keys(fd, bs, intr, cr, nlcr)
}

pub unsafe fn musl_now_ms(_ed: *mut Editor) -> i64 {
    c::musl_now_ms() as i64
}

pub unsafe fn host_time(_ed: *mut Editor) -> i64 {
    c::host_time() as i64
}

pub unsafe fn musl_delay(_ed: *mut Editor, ms: i64, interruptible: i32) {
    c::musl_delay(ms as c_long, interruptible)
}

pub unsafe fn musl_wait_for_input(_ed: *mut Editor, ms: i64) -> i32 {
    c::musl_wait_for_input(ms as c_long)
}

pub unsafe fn musl_read_input(_ed: *mut Editor, buf: *mut i8, len: i32) -> i32 {
    c::musl_read_input(buf as *mut c_char, len)
}

pub unsafe fn musl_suspend(_ed: *mut Editor) {
    c::musl_suspend()
}

pub unsafe fn host_raise(_ed: *mut Editor, sig: i32) {
    c::host_raise(sig)
}

pub unsafe fn host_exit(_ed: *mut Editor, r: i32) {
    c::host_exit(r)
}

pub unsafe fn host_message(_ed: *mut Editor, msg: *mut i8, len: i32, err: i32) {
    c::host_message(msg as *const c_char, len, err)
}

pub unsafe fn host_write(_ed: *mut Editor, s: *mut i8, len: i32) -> i32 {
    c::host_write(s as *const c_char, len)
}

pub unsafe fn host_alloc(_ed: *mut Editor, n: u64) -> *mut c_void {
    c::host_alloc(n as usize)
}

pub unsafe fn vim_snprintf(ed: *mut Editor, str: *mut i8, str_m: u64, fmt: *mut i8, args: &[VArg]) -> i32 {
    crate::printf::vim_snprintf(ed, str, str_m, fmt, args)
}
