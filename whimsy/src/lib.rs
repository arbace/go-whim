//! whimsy: the editor in Rust (doc/RUST.md).  The core is `editor`,
//! generated from the C by crefactor/togo's Rust backend and never edited
//! by hand; the rest is by hand: the runtime it is written against (`rt`),
//! the host it calls (`host`: the C host's 17 functions as glue to a
//! `Host`), vim's printf (`printf`) and the terminal host (`term`).

#![deny(warnings)]

// the generated core names alloc's Box, and so builds where std is not (the
// guest's, guest/whimsy)
extern crate alloc;

pub mod editor;
pub mod host;
pub mod printf;
pub mod rt;
pub mod term;
