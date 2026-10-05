//! whimsy as a guest (doc/GUEST.md, *The Rust guest*): the Rust editor's
//! core -- whimsy/src/editor.rs, generated, with whimsy's runtime and
//! printf, taken by path -- built without std as a static library, and
//! linked with the C guest's runtime (guest/rt/rt.c, -DWHIM_CORE_APART):
//! its entry, its exception vectors, its 15 hypercalls and its arena.  The
//! core calls the C host's 17 functions by name through `host`, which here
//! is a line of glue to rt.c's function of the same name each; one editor,
//! for the guest's life.

#![no_std]
#![allow(dead_code, unused_imports, unused_unsafe, static_mut_refs)]

extern crate alloc;

#[path = "../../whimsy/src/editor.rs"]
pub mod editor;
pub mod host;
pub mod smp;
#[path = "../../whimsy/src/printf.rs"]
pub mod printf;
#[path = "../../whimsy/src/rt.rs"]
pub mod rt;

use core::alloc::{GlobalAlloc, Layout};
use core::ffi::{c_char, c_int};

/// The heap is the guest's arena (host::arena): one region of rt.c's,
/// zeroed, never freed -- what the core allocates from, and the Rust
/// core's Box and Vec with it.
struct Arena;

unsafe impl GlobalAlloc for Arena {
    unsafe fn alloc(&self, l: Layout) -> *mut u8 {
        host::arena(l.size(), l.align().max(16))
    }
    unsafe fn dealloc(&self, _p: *mut u8, _l: Layout) {}
}

#[global_allocator]
static ARENA: Arena = Arena;

/// A panic -- a bound checked, an arithmetic overflow the core does not
/// allow -- is the guest's end, said as the C's abort would be.
#[panic_handler]
fn panic(_info: &core::panic::PanicInfo) -> ! {
    let msg = b"whimsy guest: panic\n";
    unsafe {
        host::c::host_message(msg.as_ptr() as *const c_char, msg.len() as c_int, 1);
        host::c::host_exit(70);
    }
    loop {}
}

/// The editor of this guest, for deathtrap.
static mut ED: *mut editor::Editor = core::ptr::null_mut();

/// What rt.c's whim_main calls: the core's main, on a new editor.
#[no_mangle]
pub unsafe extern "C" fn vim_main(argc: c_int, argv: *mut *mut c_char) -> c_int {
    let ed = editor::new_editor();
    ED = ed;
    let mut g = host::Glue;
    (*ed).host = &raw mut g as *mut core::ffi::c_void;
    editor::vim_main(ed, argc, argv as *mut *mut i8)
}

/// What rt.c calls when the monitor says a deadly signal came: the core's
/// handler, on this guest's editor.
#[no_mangle]
pub unsafe extern "C" fn deathtrap(sig: c_int) {
    if !ED.is_null() {
        editor::deathtrap(ED, sig);
    }
}

/// The unwinder's entries the prebuilt core (built to unwind) names, which
/// a guest built to abort never reaches.
#[no_mangle]
pub extern "C" fn _Unwind_Resume() -> ! {
    loop {}
}

#[no_mangle]
pub extern "C" fn rust_eh_personality() {}
