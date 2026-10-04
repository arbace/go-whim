# GUEST.md -- whim's core as a bare-metal guest, its host behind hypercalls

2026-10-04. A design, not a plan of record: nothing here is built. It asks
whether the editor's core can run as a virtual machine's only code -- no
operating system, no libc, no syscalls -- at the processor's native speed,
with everything it needs from the outside world asked through the 17 host
functions it already calls, each one a hypercall that a small virtual machine
monitor (VMM) answers. Three targets: KVM on Linux, amd64 and arm64; Apple's
Hypervisor.framework on an M2 Max, arm64.

## Why the core fits

The core is already shaped for this (CLAUDE.md, *The core and the host*):
everything above `whim-vim.c`'s first `#include` is a complete translation
unit that **names no libc function, holds no file descriptor and uses no
floating point**, and reaches the world only through 17 functions the host
defines below the line:

| # | function | what the VMM does | per keystroke |
|---|---|---|---|
| 1 | `musl_host_init()` | install the VMM's signal handlers; record the core's `deathtrap` entry | once |
| 2 | `musl_get_winsize(&rows, &cols)` | `TIOCGWINSZ` on the tty | on resize |
| 3 | `musl_term_start()` | raw mode on the tty | at start, after `:!` |
| 4 | `musl_term_stop()` | restore the tty | at exit, before `:!` |
| 5 | `musl_tty_keys(fd, &bs, &intr, &cr, &nlcr)` | read the tty's erase/interrupt keys and CR/NL modes | once |
| 6 | `musl_now_ms()` | monotonic clock | a few |
| 7 | `musl_delay(ms, interruptible)` | sleep | rare |
| 8 | `musl_wait_for_input(ms)` | `poll` the tty, timeout; a pending signal counts as input | 1-2 |
| 9 | `musl_read_input(buf, len)` | `read` the tty into guest memory; a signal as its key sequence | 1 |
| 10 | `musl_suspend()` | `SIGTSTP` the VMM (restore the tty first) | rare |
| 11 | `host_exit(code)` | end the VM and the VMM with `code` | once |
| 12 | `host_message(msg, len, err)` | write to stdout or stderr | rare |
| 13 | `host_alloc(n)` | -- answered **in the guest** (below) | many |
| 14 | `host_free(p)` | -- answered in the guest | many |
| 15 | `host_write(s, len)` | write the screen's output to the tty | 1 (the core buffers) |
| 16 | `host_time()` | wall clock (`WHIM_TIME` when set, as the suite pins it) | rare |
| 17 | `host_raise(sig)` | raise `sig` in the VMM | rare |

These are `editor/host.go`'s `Host` interface method for method, and Go
already has a terminal implementation of it (`editor/term`). **The VMM's
hypercall handler is that host**: it decodes a call and invokes the same
`Host` method the Go editor calls. The terminal code, the signal handling and
the suite's `WHIM_TIME` are reused, not rewritten.

Two of the 17 never leave the guest: `host_alloc` and `host_free` are a
small allocator over the guest's own RAM (a free-list over a region the VMM
maps once, lazily backed -- below). The core allocates on most edits; an exit
for each would dominate. So **15 hypercalls**, and per keystroke a handful:
`wait_for_input`, `read_input`, one `write` of the buffered screen update.

## The shape

```
  ┌───────────────────────── guest (one vCPU, ring 0 / EL1) ─────────────┐
  │ boot stub ─ page tables on, stack, jump                              │
  │ the core (whim-vim.c above the line, freestanding)                   │
  │ guest runtime: 15 hypercall stubs, the allocator, exception vectors  │
  └──────────────── hypercall: one store to the doorbell page ───────────┘
                                   │  exit
  ┌───────────────────────── VMM (a user process) ───────────────────────┐
  │ backend: KVM (Linux amd64/arm64) │ Hypervisor.framework (macOS arm64)│
  │ exit loop ─ decode ─ editor.Host (editor/term) ─ resume              │
  └──────────────────────────────────────────────────────────────────────┘
```

**One vCPU, no devices, no interrupts.** The core is single-threaded C (the
parallel `:%s` is the translated editors'; the C runs `match_lines` line by
line), so the guest is one vCPU that runs until it asks for something. There
is no timer, no interrupt controller, no virtio: the VMM emulates nothing but
the doorbell. The guest's whole view of the world is 15 calls.

## The hypercall ABI: a doorbell, the same on all three targets

Each target has a native trap (`vmcall`/`vmmcall` or an I/O port on amd64,
`HVC` on arm64), but they differ: KVM answers `vmcall` and arm64 `HVC`
itself (KVM/arm64 forwards an SMCCC range to user space only when the VMM
installs a filter, `KVM_ARM_VM_SMCCC_FILTER`, Linux 6.4 on), while
Hypervisor.framework returns `HVC` to the VMM as an exception exit. **A store
to an unmapped guest-physical page exits to the VMM on all three**, with the
address and the value stored:

- KVM (both ISAs): `KVM_RUN` returns `KVM_EXIT_MMIO` with `phys_addr`,
  `data`, `len`, `is_write`.
- Hypervisor.framework: `hv_vcpu_run` returns an exception exit whose
  syndrome is a data abort with a valid instruction syndrome (ISV), the
  faulting IPA in the exit's `physical_address`, and the source register in
  the syndrome's SRT field.

So the ABI is: **the guest fills a call block in its own RAM, then stores the
block's guest-physical address to the doorbell page** (one 64-bit store; on
arm64 a single `STR` written in assembly, so the syndrome is valid -- a
compiler's `STP` would not be).

```c
struct call {            /* in guest RAM, 64-byte aligned */
    u64 nr;              /* 1..17, the table above */
    i64 a[5];            /* arguments; pointers are guest-physical */
    i64 ret;             /* written by the VMM */
    u64 event;           /* written by the VMM: a pending signal, or 0 */
};
```

The VMM translates every pointer argument by bounds-checking it against the
guest's one memory slot (`gpa + len` inside the slot, else the call fails and
the guest is killed) and copies through the host mapping. Out-parameters
(`rows`, `cols`, `bs`, `intr`, ...) are written back into `a[]`. One exit per
call; the `event` field carries a signal back without a second one.

Alternatives, measured later rather than assumed: `out` to an I/O port on
amd64 (`KVM_EXIT_IO`, decoded with less work than MMIO), and `HVC` where the
target returns it to the VMM. The doorbell is the baseline because it is the
same code on all three.

## Signals and the deathtrap

The core is only ever running between hypercalls, so signals need no
interrupt injection. The VMM catches `SIGWINCH`, `SIGINT`, `SIGHUP`,
`SIGTERM`, `SIGTSTP`, `SIGCONT` itself and records them; the **next**
hypercall returns with `event` set, and the guest stub does what the C host's
handler would have done -- `wait_for_input` reports input, `read_input`
returns the signal's key sequence (as `editor/host.go` specifies), and for
`SIGHUP`/`SIGTERM` the stub calls the core's `deathtrap(sig)` on the way out.
A long computation still polls: vim's `ui_breakcheck` calls
`wait_for_input(0)` periodically, which is a hypercall. Should the guest
ever spin without one, the VMM can force an exit (KVM: a signal to the vCPU
thread with `immediate_exit`; Hypervisor.framework: `hv_vcpus_exit`) and kill
it -- a watchdog, not a delivery path.

## Memory

- **One slot.** The VMM reserves, for example, 4 GiB of address space with
  `mmap(MAP_PRIVATE|MAP_ANONYMOUS|MAP_NORESERVE)` and maps it as the guest's
  RAM from guest-physical 0 (`KVM_SET_USER_MEMORY_REGION`; `hv_vm_map` with
  read/write/execute). Pages are committed on first touch, so the guest's
  heap grows without a hypercall. The 16 KiB host pages of Apple silicon
  mean 16 KiB alignment on the Mac; the guest's own tables still use 4 KiB.
- **Layout:** the image (text, rodata, data, bss) at a fixed base; the boot
  stack; the call block; the page tables (written by the VMM before the
  first run); the heap to the slot's end; the doorbell page just past it,
  deliberately unmapped.
- **Protection inside the guest:** an identity map with text read-only and
  executable, data and heap non-executable, a guard page under the stack. A
  fault goes to the guest's vectors, which report it by a hypercall
  (`host_message` then `host_exit(134)`) -- the C editor's `SIGSEGV`.

## Booting, per target

No firmware and no real mode: the VMM puts the vCPU directly into the mode
the core runs in.

- **KVM, amd64** (here: AMD EPYC 7763, SVM, `/dev/kvm`, nested under the
  outer hypervisor). The VMM writes a GDT with one 64-bit code and one data
  segment and 4-level identity page tables (2 MiB pages) into guest RAM;
  sets `KVM_SET_SREGS` -- `CR0.PE|PG`, `CR4.PAE`, `EFER.LME|LMA`, `CR3`, `CS`
  long-mode -- and `KVM_SET_REGS` -- `RIP` the entry, `RSP` the stack top. The
  guest starts in 64-bit ring 0 at its first instruction.
- **KVM, arm64.** `KVM_ARM_VCPU_INIT` with the preferred target; through
  `KVM_SET_ONE_REG`: `PC`, `SP_EL1`, `PSTATE` = EL1h with interrupts masked,
  `MAIR_EL1`, `TCR_EL1`, `TTBR0_EL1` (the VMM's tables), `VBAR_EL1`, then
  `SCTLR_EL1` with the MMU and caches on.
- **Hypervisor.framework, M2 Max.** The same registers through
  `hv_vcpu_set_reg` (`HV_REG_PC`, `HV_REG_CPSR`) and `hv_vcpu_set_sys_reg`
  (`HV_SYS_REG_SP_EL1`, `MAIR_EL1`, `TCR_EL1`, `TTBR0_EL1`, `VBAR_EL1`,
  `SCTLR_EL1`); the vCPU is created and run on one thread for its life, as
  the framework requires. The VMM binary needs the
  `com.apple.security.hypervisor` entitlement (ad-hoc `codesign`). The guest
  physical address size is the chip's (query it with
  `hv_vm_config_get_max_ipa_size`; 4 GiB is far below it).

The arm64 image is one image for both arm64 targets: KVM/arm64 and
Hypervisor.framework boot it the same way.

## Building the guest

- **The core**, cut as every translation cuts it (`go tool whim cut`),
  compiled freestanding: `-ffreestanding -fno-builtin -nostdlib -fno-pic
  -mcmodel=kernel` (amd64) or `-mgeneral-regs-only` (arm64), `-mno-red-zone`
  on amd64 (interrupts aside, the exception vectors run on the same stack),
  and no SIMD in generated code (`-mgeneral-regs-only` / `-mno-sse -mno-avx`),
  so no FPU or vector state has to be set up or saved. The core uses no
  floating point already; this makes sure the compiler adds none.
- **The guest runtime**, about 400 lines of C and assembly per ISA: the entry
  stub, the 15 stubs filling the call block, the allocator, the vectors,
  `memcpy`/`memset` the compiler may emit.
- **Toolchains here**: `clang` 23 (both targets) and `gcc` / `aarch64-none-elf-gcc`
  16.2 with `aarch64-none-elf-ld` (installed); on the Mac, Apple clang with
  `-target aarch64-none-elf` and `ld.lld`, or the image built here and
  copied.

## The VMM

Go, in this repository (`guest/` for the runtime, `vmm/` for the monitor,
a `go tool whim guest` builder), which keeps the hypercall handler on
`editor/host.go`'s `Host` and the terminal on `editor/term`:

- `vmm/kvm_linux.go`: `/dev/kvm` through raw `ioctl` (`syscall.Syscall`;
  the request numbers are stable ABI), the `kvm_run` page by `mmap`, per ISA
  register setup in `kvm_linux_amd64.go` and `kvm_linux_arm64.go`.
- `vmm/hvf_darwin.go`: Hypervisor.framework through cgo (`#cgo LDFLAGS:
  -framework Hypervisor`), about a dozen functions.
- `vmm/loop.go`: the common exit loop -- doorbell, decode, `Host`, resume;
  any other exit (an unexpected port, a halt, a fault the guest did not
  report) ends the VM with a diagnostic.

Hardening is the VMM's: on Linux a seccomp filter allowing only the
syscalls the 15 calls and `KVM_RUN` need; on macOS the app sandbox. The
guest itself has no syscalls to restrict.

## Testing arm64 on this amd64 machine

KVM runs only its host's ISA, so here arm64 is emulated, not virtualized:

- **The guest image alone**: `qemu-system-aarch64 -machine virt -cpu max`
  (QEMU 11.1, installed) runs the arm64 image as a bare-metal kernel. Its
  hypercalls then need a QEMU-side answer -- a small QEMU-specific stub that
  maps the doorbell to semihosting or a PL011 is enough to see it boot and
  draw, not to run the suite.
- **The arm64 VMM path**: an arm64 Linux VM under
  `qemu-system-aarch64 -machine virt,virtualization=on` has an emulated EL2
  and KVM/arm64 inside; the real VMM and guest run there unchanged. Slow --
  every instruction emulated, every exit nested -- but it is the KVM/arm64
  code itself, so the suite's correctness can be gated on it.
- **Natively**: an arm64 Linux host -- or Asahi Linux on the M2 Max itself,
  which has KVM/arm64 -- runs the same VMM and image at full speed.

## Expected performance

Guest instructions run at native speed on all three. The cost is per exit:
roughly a microsecond on bare metal, several times that here, where the
VMM's own host is a virtual machine (nested SVM: each exit goes through the
outer hypervisor). Per keystroke the core makes a handful of hypercalls and
already buffers its screen output into one write, so interactive use stays
far below a millisecond. The heavy case (5,000 lines, three `:s` and a `:g`)
is computation with a few writes: it should run at the C binary's speed.
`host_alloc` staying in the guest is what keeps it so.

## Milestones, each with a gate

1. **A guest that writes and exits**, KVM amd64: the VMM, the boot stub, the
   doorbell, `host_write` and `host_exit`. *Gate:* "hello" on the terminal,
   exit code back, one exit per call (counted).
2. **The core on KVM amd64.** *Gate:* the suite runs it as an editor
   (`whim test --guest`, the VMM binary in the C's place): 80 of 80 and 240
   of 240 answer as the C does, the control seen; exits per keystroke and
   the heavy case's time against the native C reported.
3. **The arm64 image**, under KVM/arm64 in `qemu-system-aarch64
   virtualization=on` here. *Gate:* the quick suite answers as the C does
   (slowly).
4. **Hypervisor.framework on the M2 Max.** *Gate:* the suite, run on the Mac
   against the C built there, 80 and 240; the heavy case at native speed.
5. **Hardening and measurement**: seccomp, the watchdog, exits per second
   under the stress test, the I/O-port and `HVC` alternatives measured
   against the doorbell.

## Decided (2026-10-04)

The Mac is the eventual port, not a near goal; the priority is that the KVM
implementation follows Hypervisor.framework's concepts so closely that the
port is almost trivial. Asahi Linux is out for now: this host runs Alpine
Edge, the Mac runs macOS. Milestones 1-3 and 5 go ahead; 4 waits.

- **The VMM is Go over a package `hv` whose API mirrors Apple's arm64 C API
  one to one**: `VMCreate`, `VMMap(mem, ipa, size, flags)`, `VCPUCreate`
  (returning the vCPU and its exit record), `VCPURun`, `GetReg`/`SetReg`,
  `GetSysReg`/`SetSysReg`, Apple's register and system-register names, and
  an exit struct shaped like `hv_vcpu_exit_t` (a reason, an exception
  syndrome, a virtual and a physical address). The Linux backend is KVM by
  raw `ioctl`, no cgo; the macOS backend, later, is a thin cgo file per
  function. The hypercall handler is `editor/host.go`'s `Host`
  (`editor/term`).
- **amd64 sits behind the same `hv` shape** with x86 register names (`RIP`,
  `RSP`, `CR0`/`CR3`/`CR4`, `EFER`, segments): one exit loop for both ISAs,
  only register setup per ISA. Apple's own x86 API is not the model.
- **A hypercall is the MMIO doorbell** on all three targets. The KVM backend
  reports a doorbell store as HVF does -- an exception exit carrying a
  data-abort syndrome and the faulting physical address -- so the loop
  decodes HVF's form everywhere.
- **arm64 is tested in an Alpine Edge aarch64 VM** under
  `qemu-system-aarch64 -machine virt,virtualization=on -cpu max`: real
  KVM/arm64 inside an emulated EL2, the real VMM and guest unchanged.

## A second guest: the Go editor on TamaGo (an option, not scheduled)

Standard Go cannot build a guest: every `GOOS` assumes an operating system
(threads, `mmap`, timers, signals), and bare-metal support is only proposed
upstream (golang/go#73608). **TamaGo** (github.com/usbarmory/tamago) is a
modified Go distribution adding `GOOS=tamago`: unmodified Go on bare metal,
no C and no OS, on amd64, arm, arm64 and riscv64. It already runs under KVM
micro-VMs on amd64 (its Firecracker, Cloud Hypervisor and QEMU microvm
boards), single- or multi-core, with interrupts on amd64 and arm64.

TamaGo adapts to a machine through a **board** package: a handful of
runtime hooks -- CPU setup, the clock, console output, memory, random
numbers -- which is the shape of the core's 17 host functions. So a second
guest is possible beside the C core:

- **The guest**: the Go editor (`editor/`, already a library on
  `editor/host.go`'s `Host`), built with TamaGo for amd64 and arm64.
- **A board for the `hv` VMM**: TamaGo's runtime hooks and the editor's
  `Host` both answered by the doorbell -- the same 15 hypercalls, plus the
  runtime's few (clock, memory, randomness), so the VMM side is the one this
  design already builds.
- **What it would add**: the Go editor's parallel `:%s` on several vCPUs
  through goroutines, which the single-threaded C core cannot; and the
  guest's memory safety from Go itself.
- **What it costs**: a separate Go distribution, typically pinned a little
  behind upstream releases (installed only with the user's go-ahead); OS
  packages (`os`, `net`) limited, though `editor/` needs little beyond its
  `Host`; a board package of our own, since TamaGo's boot through PVH or
  UEFI and a serial console, not our doorbell; and a larger, less auditable
  guest than the C core.

Recommended order: the C core first (milestones 1-5), then this as a later
milestone once the hypercall surface is proven on it.

## Open questions

- Is one 4 GiB lazily committed slot enough, or should the allocator ask the
  VMM to map more (a sixteenth hypercall) for very large files?
- Should `musl_wait_for_input` and `musl_read_input` be merged into one call
  (wait and read when ready), halving the exits per key?
- The parallel `:%s` of the translated editors on several vCPUs: a fork/join
  in shared memory with a halt-and-kick protocol, with no new hypercall --
  worth it only once a single vCPU is measured.
- Where does the guest's C reference come from on the Mac for the suite:
  `whim-vim.c` built there (its host region is POSIX), or outputs recorded
  here?
