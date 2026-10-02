//! bin/whimsy, the launcher: the editor (`whimsy::host::run`) on the
//! terminal host (`whimsy::term`), with the command line's arguments as
//! bytes, the process ending with the editor's status -- whim-vim.c's
//! `main`, whose `__builtin_setjmp` is `run`'s catch of host_exit.
//!
//! Output is not buffered by Rust: the host writes with write(2) on fd 1
//! and 2 directly, and nothing here prints through `std::io::stdout`.
//!
//! DEVIATION (undone): Rust's runtime sets SIGPIPE to SIG_IGN before `main`
//! runs, where a C program starts with it at SIG_DFL; it is put back, so
//! that until the host's init ignores it (as the C's does) the process is
//! as the C's.

use std::os::unix::ffi::OsStrExt;

extern "C" {
    fn signal(sig: i32, handler: usize) -> usize;
}

const SIGPIPE: i32 = 13;
const SIG_DFL: usize = 0;

fn main() {
    unsafe {
        signal(SIGPIPE, SIG_DFL);
    }
    let args: Vec<Vec<u8>> = std::env::args_os().map(|a| a.as_bytes().to_vec()).collect();
    let code = whimsy::host::run(Box::new(whimsy::term::new()), &args);
    std::process::exit(code);
}
