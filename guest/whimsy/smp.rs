//! The Rust guest on several vCPUs (doc/GUEST.md, *The Rust guest on
//! several vCPUs*): `match_lines`' chunks run as whimsy's on Linux runs them
//! on threads -- about four a vCPU, none smaller than 64 lines -- here on
//! the machine's other vCPUs, by the calls a Go guest's runtime makes
//! (guest/abi: CPUStart, CPUPark, CPUWake, CPUSelf).  There are no
//! interrupts: a vCPU with no chunk to take parks in the monitor, and the
//! boot vCPU, having published a job, wakes them; the last one done wakes
//! the boot vCPU, which has taken chunks meanwhile.  A vCPU's calls are its
//! own: a call block each, where rt.c has one for the Host's calls, which
//! the boot vCPU alone makes.

use core::arch::{asm, global_asm};
use core::sync::atomic::{AtomicBool, AtomicPtr, AtomicUsize, Ordering};

/// guest/abi's call numbers, and the call block's layout.
const CPU_START: u64 = 21;
const CPU_PARK: u64 = 22;
const CPU_WAKE: u64 = 23;
const MAX_CPUS: usize = 32;
const DOORBELL: usize = 0xf000_0000;

#[repr(C, align(64))]
struct Block {
    nr: u64,
    a: [i64; 5],
    ret: i64,
    event: u64,
}

const EMPTY: Block = Block { nr: 0, a: [0; 5], ret: 0, event: 0 };
static mut BLOCKS: [Block; MAX_CPUS] = [EMPTY; MAX_CPUS];

/// call makes call nr from vCPU cpu, on its own block: its address stored
/// to the doorbell from RAX, as rt.c's.
unsafe fn call(cpu: usize, nr: u64, a: [i64; 5]) -> i64 {
    let b = &raw mut BLOCKS[cpu];
    (*b).nr = nr;
    (*b).a = a;
    (*b).ret = 0;
    (*b).event = 0;
    asm!("mov qword ptr [{door}], rax", door = in(reg) DOORBELL, in("rax") b as usize, options(nostack));
    core::ptr::read_volatile(&raw const (*b).ret)
}

// An AP's first instruction (abi.CPUStart): the boot vCPU's mode, RDI the
// argument, RSI the function, RDX its number, RSP its stack.  SSE on, as
// rt.c turns it on for the boot vCPU; the function does not return.
global_asm!(
    ".globl whim_apentry",
    "whim_apentry:",
    "cli",
    "mov rax, cr4",
    "or rax, 0x600",
    "mov cr4, rax",
    "mov rdi, rdx",
    "call rsi",
    "2: hlt",
    "jmp 2b",
);

/// The job the vCPUs share: the work, the range and its chunks, the next
/// chunk to take, whether one failed, and how many vCPUs are still at it.
static WORK: AtomicPtr<()> = AtomicPtr::new(core::ptr::null_mut());
static N: AtomicUsize = AtomicUsize::new(0);
static SIZE: AtomicUsize = AtomicUsize::new(0);
static NEXT: AtomicUsize = AtomicUsize::new(0);
static FAILED: AtomicBool = AtomicBool::new(false);
static BUSY: AtomicUsize = AtomicUsize::new(0);
/// A job's number: a vCPU takes the chunks of each job once.
static JOB: AtomicUsize = AtomicUsize::new(0);
/// The vCPUs started, beside the boot one: 0 until the first job asks.
static APS: AtomicUsize = AtomicUsize::new(0);
static STARTED: AtomicBool = AtomicBool::new(false);

type Work<'a> = &'a (dyn Fn(i64, i64) -> bool + Sync);

/// take runs the job's chunks until there are none left or one failed.
unsafe fn take() {
    let w: Work = *(WORK.load(Ordering::Acquire) as *const Work);
    let (n, size) = (N.load(Ordering::Acquire), SIZE.load(Ordering::Acquire));
    loop {
        let from = NEXT.fetch_add(size, Ordering::AcqRel);
        if from >= n || FAILED.load(Ordering::Acquire) {
            break;
        }
        if !w(from as i64, (from + size).min(n) as i64) {
            FAILED.store(true, Ordering::Release);
        }
    }
}

/// An AP's life: each job's chunks, then parked until the next.
extern "C" fn ap(cpu: usize) -> ! {
    let mut seen = 0;
    loop {
        let job = JOB.load(Ordering::Acquire);
        if job == seen {
            unsafe { call(cpu, CPU_PARK, [-1, 0, 0, 0, 0]) };
            continue;
        }
        seen = job;
        unsafe { take() };
        if BUSY.fetch_sub(1, Ordering::AcqRel) == 1 {
            unsafe { call(cpu, CPU_WAKE, [0, 0, 0, 0, 0]) };
        }
    }
}

/// AP_STACK is each AP's stack, from the arena.
const AP_STACK: usize = 4 << 20;

/// start starts every vCPU the machine has beside the boot one, once:
/// CPUStart answers -1 for a vCPU it has not.
unsafe fn start() {
    if STARTED.swap(true, Ordering::AcqRel) {
        return;
    }
    let mut n = 0;
    for cpu in 1..MAX_CPUS {
        let stack = crate::host::arena(AP_STACK, 16) as usize + AP_STACK;
        let entry = ap as extern "C" fn(usize) -> ! as usize;
        if call(0, CPU_START, [cpu as i64, stack as i64, 0, entry as i64, 0]) != 0 {
            break;
        }
        n += 1;
    }
    APS.store(n, Ordering::Release);
}

/// The fewest lines a chunk is given (whimsy's rt.rs).
const CHUNK_LEAST: usize = 64;

/// chunks is whimsy's rt::chunks on the machine's vCPUs: work(from, to) over
/// [0, n), and whether every chunk's did.
pub fn chunks(n: i64, work: Work) -> bool {
    let n = n.max(0) as usize;
    unsafe { start() };
    let cpus = APS.load(Ordering::Acquire) + 1;
    let size = CHUNK_LEAST.max(n.div_ceil(4 * cpus));
    if cpus == 1 || size >= n {
        return work(0, n as i64);
    }
    let w: *const Work = &work;
    WORK.store(w as *mut (), Ordering::Release);
    N.store(n, Ordering::Release);
    SIZE.store(size, Ordering::Release);
    NEXT.store(0, Ordering::Release);
    FAILED.store(false, Ordering::Release);
    let aps = APS.load(Ordering::Acquire);
    BUSY.store(aps, Ordering::Release);
    JOB.fetch_add(1, Ordering::AcqRel);
    for cpu in 1..=aps {
        unsafe { call(0, CPU_WAKE, [cpu as i64, 0, 0, 0, 0]) };
    }
    unsafe { take() };
    while BUSY.load(Ordering::Acquire) > 0 {
        unsafe { call(0, CPU_PARK, [-1, 0, 0, 0, 0]) };
    }
    WORK.store(core::ptr::null_mut(), Ordering::Release);
    !FAILED.load(Ordering::Acquire)
}
