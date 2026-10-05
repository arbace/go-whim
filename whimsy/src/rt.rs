//! The runtime the generated Rust is written against (doc/RUST.md): the few
//! things C does that Rust spells as a function -- an array's first
//! element's address, the difference of two pointers, a function pointer's
//! address, a char array from a string -- the arguments of a variadic
//! call, and the parallel chunks of `match_lines`.  The generated module
//! imports it whole (`use crate::rt::*`).

use core::ffi::c_void;
use core::sync::atomic::{AtomicBool, AtomicUsize, Ordering};

/// An argument of a C variadic call, as C passes it once promoted: a signed
/// integer sign-extended, an unsigned one zero-extended, a pointer.
#[derive(Clone, Copy, Debug)]
pub enum VArg {
    I(i64),
    U(u64),
    P(*const c_void),
}

impl VArg {
    /// The argument read as a C integer of 64 bits (va_arg's `long`).
    pub fn bits(self) -> u64 {
        match self {
            VArg::I(v) => v as u64,
            VArg::U(v) => v,
            VArg::P(p) => p as u64,
        }
    }
    /// The argument read as a pointer (va_arg's `char *`).
    pub fn ptr(self) -> *const u8 {
        match self {
            VArg::P(p) => p as *const u8,
            v => v.bits() as *const u8,
        }
    }
}

/// An array's value where C converts it: the address of its first element.
#[inline(always)]
pub fn decay<T, const N: usize>(a: *mut [T; N]) -> *mut T {
    a as *mut T
}

/// The same of an array reached through a *const pointer: a *const one.
#[inline(always)]
pub fn decay_const<T, const N: usize>(a: *const [T; N]) -> *const T {
    a as *const T
}

/// p - q, in elements of T: C's difference of two pointers into one array
/// (void's elements are bytes, as gcc has them).
#[inline(always)]
pub fn pdiff<T>(p: *const T, q: *const T) -> i64 {
    let size = core::mem::size_of::<T>().max(1) as isize;
    ((p as isize).wrapping_sub(q as isize) / size) as i64
}

/// A function pointer's address, for C's comparisons and conversions of
/// one: 0 for none.
#[inline(always)]
pub fn fn_addr<F: Copy>(f: Option<F>) -> usize {
    match f {
        None => 0,
        Some(g) => unsafe { core::mem::transmute_copy::<F, usize>(&g) },
    }
}

/// A char array of N from a string's bytes: C's `char a[N] = "..."`, the
/// rest zeros.
pub fn str_u8<const N: usize>(s: &[u8]) -> [u8; N] {
    let mut a = [0u8; N];
    let n = s.len().min(N);
    a[..n].copy_from_slice(&s[..n]);
    a
}

/// The same of a signed char array.
pub fn str_i8<const N: usize>(s: &[u8]) -> [i8; N] {
    let mut a = [0i8; N];
    for (i, b) in s.iter().take(N).enumerate() {
        a[i] = *b as i8;
    }
    a
}

/// A value shared with the chunks' threads: a raw pointer, which Rust will
/// not send, sent all the same -- the C's own memory, which each chunk
/// reads and writes apart from the others (phase 96).
#[derive(Clone, Copy)]
pub struct Shared<T>(pub T);

unsafe impl<T> Send for Shared<T> {}
unsafe impl<T> Sync for Shared<T> {}

impl<T: Copy> Shared<T> {
    /// The value: a method, so that a closure captures the whole Shared and
    /// not its field.
    #[inline(always)]
    pub fn get(&self) -> T {
        self.0
    }
}

/// The fewest lines a chunk is given: below it a thread costs more than it
/// saves.
const CHUNK_LEAST: usize = 64;

/// Runs work(from, to) over [0, n) in chunks, on as many threads as the
/// machine has cores, and says whether every chunk's work did: the parallel
/// body of `match_lines`, which the C writes as one loop over the range
/// (editor/chunks.go is the Go's).  There are about four chunks a core, so
/// that one slow chunk does not hold the rest, and none smaller than
/// CHUNK_LEAST; a range of one chunk runs on the caller's thread, and a
/// chunk that did not stops the chunks not yet started.
#[cfg(not(feature = "guest"))]
pub fn chunks<F>(n: i64, work: F) -> bool
where
    F: Fn(i64, i64) -> bool + Sync,
{
    let n = n.max(0) as usize;
    let cores = std::thread::available_parallelism().map(|c| c.get()).unwrap_or(1);
    let size = CHUNK_LEAST.max(n.div_ceil(4 * cores));
    if size >= n {
        return work(0, n as i64);
    }
    let failed = AtomicBool::new(false);
    let next = AtomicUsize::new(0);
    std::thread::scope(|s| {
        for _ in 0..cores.min(n.div_ceil(size)) {
            s.spawn(|| loop {
                let from = next.fetch_add(size, Ordering::Relaxed);
                if from >= n || failed.load(Ordering::Relaxed) {
                    break;
                }
                if !work(from as i64, (from + size).min(n) as i64) {
                    failed.store(true, Ordering::Relaxed);
                }
            });
        }
    });
    !failed.load(Ordering::Relaxed)
}

/// The guest's chunks (guest/whimsy): on the machine's vCPUs, by the
/// guest's own fork and join (guest/whimsy/smp.rs) -- a bare machine has no
/// threads to give them.
#[cfg(feature = "guest")]
pub fn chunks<F>(n: i64, work: F) -> bool
where
    F: Fn(i64, i64) -> bool + Sync,
{
    crate::smp::chunks(n, &work)
}
