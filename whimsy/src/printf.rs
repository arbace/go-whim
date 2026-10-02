//! vim_snprintf: a placeholder until the port (doc/RUST.md, milestone 3).
use crate::editor::Editor;
use crate::rt::VArg;

/// Formats nothing yet.
pub unsafe fn vim_snprintf(_ed: *mut Editor, _str: *mut i8, _str_m: u64, _fmt: *mut i8, _args: &[VArg]) -> i32 {
    0
}
