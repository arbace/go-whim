# GUEST.md -- whim's core as a bare-metal guest, its host behind hypercalls

2026-10-04. A design, and its milestones as built (*As built*, below). It asks
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
`wait_for_input`, `read_input`, one `write` of the buffered screen update --
and since *The merged wait and read* (below), the wait and the read are one
call, `wait_read`, the read answered in the guest from what it brought.

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
  `-target aarch64-none-elf` and an `ld.lld` Xcode does not ship -- so the
  image is built here and copied (*Running on the Mac*).

## The VMM

Go, in this repository (`guest/` the image's builder and `guest/rt/` the
runtime, `hv/` the hypervisor's shape and backends, `vmm/` the monitor,
`go tool whim guest` the command), which keeps the hypercall handler on
`editor/host.go`'s `Host` and the terminal on `editor/term`:

- `hv/kvm_linux.go`: `/dev/kvm` through raw `ioctl` (`syscall.Syscall`;
  the request numbers are stable ABI), the `kvm_run` page by `mmap`, per ISA
  registers in `kvm_linux_amd64.go` and `kvm_linux_arm64.go`; the guest's
  register setup per ISA in `vmm/setup_amd64.go` and `setup_arm64.go`.
- `hv/hvf_darwin.go`: Hypervisor.framework through purego, no cgo
  (*Decided*), fourteen functions.
- `vmm/vmm.go`: the common exit loop -- doorbell, decode, `Host`, resume;
  any other exit (an unexpected port, a halt, a fault the guest did not
  report) ends the VM with a diagnostic.

Hardening is the VMM's: on Linux a seccomp filter allowing only the
syscalls the 15 calls and `KVM_RUN` need. On macOS the app sandbox was
weighed and not taken (*Running on the Mac*): the monitor is signed with
the hypervisor entitlement alone. The guest itself has no syscalls to
restrict.

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
roughly a microsecond on bare metal, and 16-18 µs measured here, where the
VMM's own host is a virtual machine (nested SVM: each exit goes through the
outer hypervisor; milestone 5). Per keystroke the core makes a handful of
hypercalls -- 6.7 exits a key measured, a wait and a read for each key and
a write for the screen (3.7 since the wait and the read were merged) -- so
interactive use stays far below a millisecond.
The heavy case (5,000 lines, three `:s` and a `:g`) turned out to be 20,786
exits, the C's own selects and reads as `:g` feeds `normal` its keys: 1.0-1.6
times the native C's time here, of which the exits are about 0.33 s.
Merged, 10,416 exits and 0.8-0.9 times the C's time.
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
   Its groundwork is done (*Running on the Mac*); the gate waits for the Mac.
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
  raw `ioctl`, no cgo; the macOS backend, later, binds each framework
  function by purego (`github.com/ebitengine/purego`: the framework
  `Dlopen`ed, each `hv_*` function a Go function of its C signature by
  `RegisterLibFunc`), so there is no cgo anywhere and `GOOS=darwin
  GOARCH=arm64 CGO_ENABLED=0` compiles it here; a vCPU locked to its thread
  for its life (`runtime.LockOSThread`), as on Linux. The hypercall handler is `editor/host.go`'s `Host`
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

## A second guest: the Go editor on TamaGo (built: *As built*)

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
milestone once the hypercall surface is proven on it -- which is how it
went: *As built*, *The second guest*.

## As built

### Milestone 1: a guest that writes and exits

`hv/` is the framework's shape (`hv.go`: `Return`, `VCPUExit`, the
syndrome's fields; `regs_arm64.go` Apple's register names and values,
`regs_amd64.go` x86 names behind the same shape), its KVM backend
(`kvm_linux.go`, `kvm_linux_amd64.go`: raw `ioctl`, no cgo; the general
registers through `kvm_run`'s synced area, so reading the doorbell's
register costs no system call) and `hvf_darwin.go`, the framework bound
by purego (built for `darwin/arm64` with cgo off, as a compile check here;
never run). `vmm/` is the
monitor: the image loaded, the slot laid out (`layout.go`), the vCPU put in
long mode (`setup_amd64.go`), the exit loop and the calls (`vmm.go`).
`guest/` builds the image: `rt/rt.c` the runtime, `rt/entry_amd64.S` the
entry and vectors, `rt/hello.c` the stand-in core. *Gate met:* `go tool whim
guest --hello` writes `bin/whim-guest-hello`, which prints `hello` and
exits 3; `guest`'s `TestHello` runs it on a recording host and counts 2
exits for its 2 calls, and `hv`'s tests run a real-mode guest of 11 bytes:
a store where no memory is, reported as a 4-byte data abort from RAX with
RIP past it, a halt, and a spin ended by `VCPUsExit`.

### Milestone 2: the core on KVM amd64

`guest.Source` is the guest's one translation unit: the core as `whim.Cut`
cuts it, the C host's `vim_snprintf` and its helpers (the host from its
`#include`s to its first object of the system's, which call nothing but the
core; its `static_assert`s on the system's headers left out), and
`guest/rt/rt.c`, which defines the core's 17 host functions: 15 as calls
through the doorbell, `host_alloc` and `host_free` as the C host's own bump
arena of 1 GiB over the heap the monitor hands the entry (zero, committed on
first touch, exhausted in the C's words). One unit because the core
declares its host functions `static`. It is compiled by clang 23 at `-O2`,
`-ffreestanding -fno-builtin -nostdlib -mgeneral-regs-only -mno-red-zone
-fno-pic`, no stack protector and no unwind tables; `-mcmodel=small`, not
`kernel`, since the image is linked low, at 2 MiB (`rt/link_amd64.ld`, one
segment each for text, read-only data and data). 11 s of clang, 20 s with
the monitor. The image is 0.9 MB; the launcher, `bin/whim-guest`, is the
monitor with it appended (4.2 MB). The C's `-O0` is the reference; the
guest is held to behaviour, not to its code.

Signals: the monitor's host is `editor/term`, as `bin/whim`'s; its
`deathtrap` callback panics with the signal, which the call's dispatch
recovers into the call block's `event`, and the runtime calls the core's
`deathtrap` on the way back -- where the C handler would have run; a wait or
read is made again when it returns (the signal blocked). SIGTERM at the
start screen: the guest prints the C's bytes, "Vim: Caught deadly signal
TERM", and exits 1 as the C does. An exception the guest takes goes through
its IDT (every gate on IST1, so a stack overflow is reported too) to
`whim_fault`, which makes a call of its own; the monitor prints it and the
process dies of SIGSEGV, as the C would (`guest`'s `TestFault`: a null
store, and a recursion through the guard page under the 64 MiB stack).

*Gate met:* `go tool whim test --guest`: 80 of 80 cases answer exactly as
the C does, its control (the launcher with its one `" INSERT"` changed in
its bytes) seen by 76; `--wide --guest`: 240 of 240 (keys 102, ex 98, argv
30, pty 10), its control seen by 94, 0, 0 and 6 -- as the Go editor's.
The heavy case: C 430-584 ms, the guest 529-672 ms (1.1-1.6x); of the
guest's 0.64 s, 0.56 s is inside `KVM_RUN` and 47 ms in the monitor, for
20,786 exits -- every one a call, 10,370 of them `wait_for_input` and as
many `read_input`, the C's own `select` and `read` calls. Exits a key: 61
over the quick suite's 5,192 bytes of keys (317,193 exits, 112,331 reads),
whose `par_*` cases print 3,000 lines after each command; 6.7 in the
others: a key costs a wait and a read, and the screen's update one write.


### Milestone 3: the arm64 image, on KVM/arm64

`hv/kvm_linux_arm64.go` is the arm64 backend: every register through
`KVM_GET/SET_ONE_REG`, an `HV_SYS_REG_*` value ORed into KVM's sysreg id
(the two pack op0, op1, CRn, CRm and op2 alike), and the four KVM keeps
among its core registers (`SP_EL0`, `SP_EL1`, `ELR_EL1`, `SPSR_EL1`)
mapped there. The framework leaves PC on a store that exits and the monitor
moves it; KVM moves it itself when the vCPU next runs. So while an MMIO
exit is pending, PC reads as the store's address, a PC set is written 4
short of where it is to land (nothing is written when it is the store's
next instruction), and a PC left alone is put back on the store, which is
made again -- the framework's behaviour (`TestStoreAgainARM64`). KVM does
not say which register a store read; the ABI stores from X0, which is read
and checked against the value KVM reports. A call costs three ioctls:
`KVM_RUN`, X0 read, PC read. `vmm/setup_arm64.go` boots the image at EL1t
with the MMU on: 3-level tables for a 32-bit space (4 KiB granule, 2 MiB
blocks; the doorbell's page Device-nGnRE), `MAIR_EL1` 0x04ff, `TCR_EL1`
T0SZ 32 with TTBR1 walks off, `SCTLR_EL1` M, C and I, the stack on
`SP_EL0` and the vectors (`VBAR_EL1`, `rt/entry_arm64.S`) on `SP_EL1`, so a
fault in the stack's guard page is reported. The image is built here by
clang `--target=aarch64-none-elf -mgeneral-regs-only` and
`aarch64-none-elf-ld` (`go tool whim guest --arch arm64`, `make
bin/whim-guest-arm64`), and the monitor by `GOARCH=arm64 CGO_ENABLED=0`.

*The aarch64 VM* (`guest/vm/aarch64.sh`: `fetch`, `start`, `put`, `run`,
`stop`; its state in `.tmp/vm`, nothing installed on the host): Alpine
Edge's aarch64 netboot files (`vmlinuz-virt`,
`initramfs-virt`, `modloop-virt`, Linux 6.18.42) from dl-cdn.alpinelinux.org,
booted by `qemu-system-aarch64 -machine virt,virtualization=on,gic-version=3
-cpu max -accel tcg,thread=multi -smp 8 -m 6144` with an apkovl whose local
service fetches shell jobs over HTTP from the host and posts their output
back. The host side is QEMU's own: user networking forwards the VM's
connections to 10.0.2.100:80 to the script itself (`guestfwd=...-cmd:`),
one process a connection, which answers a GET from `.tmp/vm/www` -- the
apkovl, the modloop, job N, a file `put` for it -- and stores a POST as
job N's output; so there is no server to keep running, and the VM boots in
about 30 s. `run` waits for the output and exits as the job did. Its kernel starts at EL2 and runs KVM in VHE mode ("VHE mode
initialized successfully"); `/dev/kvm` is there. gcc and musl-dev were
installed in it (`apk add`) to build the C reference with the one compile
line (70 s under emulation). `hv`'s arm64 tests and `internal/suite`'s
`TestGuestPrebuilt` -- the quick suite's cases on a C editor and a guest
launcher built already, the control the launcher with its image's
`" INSERT"` changed -- are cross-compiled here (`GOARCH=arm64 go test -c`)
and run there.
For the Go guest, as run:

```sh
guest/vm/aarch64.sh fetch && guest/vm/aarch64.sh start
go tool whim guest --go --arch arm64 -o .tmp/arm/whim-guest-go    # TAMAGO_ROOT set
GOARCH=arm64 CGO_ENABLED=0 go test -c -o .tmp/arm/suite.test ./internal/suite
guest/vm/aarch64.sh put src/whim-vim.c .tmp/arm/whim-guest-go .tmp/arm/suite.test
guest/vm/aarch64.sh run 'apk add -q gcc musl-dev util-linux-misc &&
  gcc -O0 -fno-stack-protector -static -no-pie -s -o whim-vim whim-vim.c &&
  TMPDIR=/h WHIM_SUITE_LIMIT=60s WHIM_SUITE_C=/h/whim-vim WHIM_SUITE_GUEST=/h/whim-guest-go \
  taskset -c 0-3 ./suite.test -test.run TestGuestPrebuilt -test.v -test.timeout 3h'
guest/vm/aarch64.sh stop
```

*Gate met:* inside that VM the quick suite's 80 cases answer exactly as the
C built there does, the control seen by 76 (the guest's runs 266 s of the
test's 449 s); the exits are the amd64 guest's to the count: 317,193, 61.09
a key, 6.68 without the `par_*` cases. `hv`'s three arm64 tests pass: two
doorbell stores decoded as 8-byte data aborts from X0 with PC on each and
moved by the monitor, a spin ended by `VCPUsExit`, a store made again when
PC is left alone, and Apple's encodings reaching KVM's registers.
`bin/whim-guest-hello` for arm64 prints "hello" and exits 3.

### Milestone 5: hardening and measurement

*Seccomp* (`vmm/seccomp_linux*.go`): once the VM is built, before the guest
first runs, a filter goes on every thread of the monitor (`TSYNC`; threads
the Go runtime starts later inherit it): 43 system calls on amd64 and 42 on
arm64 -- the Go runtime's, the terminal host's (`select`/`pselect6`,
`pipe2`, `kill`, `rt_sigpending` for `:suspend`) -- and of `ioctl` only
`KVM_RUN`, `TCGETS`, `TCSETS`, `TIOCGWINSZ` and the ISA's register access
in the exit loop (`KVM_GET/SET_SREGS`; `KVM_GET/SET_ONE_REG`); anything else
kills the process. The list was found by running both suites with the
filter logging instead (`WHIM_GUEST_SECCOMP=log`; the kernel's audit
showed `KVM_SET_SREGS` at the first run and `rt_sigpending`) and is held
to them in kill mode: the quick and wide suites pass under it on amd64,
the quick suite in the aarch64 VM, and SIGTERM still ends the guest in the
C's words. `WHIM_GUEST_SECCOMP=0` leaves it off.

*The watchdog*: a goroutine ends a guest that runs `WHIM_GUEST_WATCHDOG`
(60 s by default) without a hypercall, by `hv.VCPUsExit` -- KVM's
`immediate_exit` and a SIGURG to the vCPU's thread, which the Go runtime
ignores -- and the loop reports it as a fault (`guest`'s `TestWatchdog`: a
spinning stand-in ended at 200 ms). The longest run between two exits,
which the monitor now counts: under 1 ms over the quick suite, 31 ms in
the heavy case, 2.1 s for a `:%s` over 200,000 lines -- vim's breakcheck
asks for input less often than that loop takes, so 60 s is about 5.7
million lines of `:%s`, and a file that size wants a longer watchdog.

*Exits per second*: over the quick suite's 80 cases run twice, 634,386
exits in 14.6 s of the monitor's loop, 43,300 a second; under the stress
test's load (48 busy loops on the 64 cores, `TestStress`, 10 runs a case),
3,171,930 exits in 88.2 s, 36,000 a second, and 0 of 720 runs differ
from their case's first. *A hypercall's cost*, `go tool whim guest
--bench` (a stand-in core making argv[1] calls of `host_time`), the
monitor's loop time over the exits: on amd64 here, nested under SVM, the
doorbell 16.2-18.3 µs, an `out` to a port (`--alt`: `KVM_EXIT_IO`, which
hv reports as an exception of its own class, `ECPortIO`) 14.7-17.2 µs,
about a tenth less; on arm64 in the emulated VM, the doorbell 349-361 µs
and an `hvc` forwarded by `KVM_ARM_VM_SMCCC_FILTER` (`hv.ForwardHVC`, an
SMC64 OEM function ID, reported as the framework reports an HVC, EC 0x16)
328-339 µs, a twentieth less -- under emulation, so only the ratio says
anything. The doorbell stays: it is the one trap the same on all three
targets, and the alternatives save less than the merged wait-and-read
did (*The merged wait and read*).

The wide suite in the aarch64 VM, after milestone 3: all 240 answer as the
C does there -- keys 102, its control seen by 94; Ex commands 98 (with the
C's whim-vim.c to enumerate them, `WHIM_SUITE_SRC`), seen by 1; command
lines 30; the pseudo-terminal 10, seen by 6 -- and the heavy case answers
as the C does: C 5 s, the guest 11-12 s under emulation, its 20,786 exits
9.3 s inside `KVM_RUN` and 2.0 s in the monitor.

### The second guest: the Go editor on TamaGo

*Built on amd64 and arm64, both gates met (arm64: below).* TamaGo is installed at `/root/tamago-go`
(tamago-go1.27.1, the system Go's release; `go tool dist list` has
`tamago/amd64` and `tamago/arm64`), its module
`github.com/usbarmory/tamago` v1.27.1 the runtime's `goos` overlay
(`GOOSPKG`). It is not this repository's: the builder finds it by
`TAMAGO_ROOT` (or `--tamago DIR`, `make ... TAMAGO_ROOT=...`), and without
it fails saying so; a directory without `src/runtime/os_tamago.go` or
`bin/go` is refused by name.

*The pieces.* `guest/abi` is the ABI as Go constants -- the doorbell, the
call numbers, the block -- importing nothing, so a `GOOS=tamago` build can
use it; `vmm`'s `TestABI` holds the monitor's numbers to it. `guest/tamago/`
is a module of its own (its `go.mod` replaces this one and `crefactor/` by
path), so the main module's `go build ./...` never sees it and only
TamaGo's toolchain builds it: `board/` the board, `main.go` the guest --
`editor.Main` on a `Host` whose 15 methods are calls through the doorbell
and whose `Exit` is the board's -- and `faulty/` a stand-in for the board's
own paths. `go tool whim guest --go` (`make bin/whim-guest-go`) builds the
image (`-ldflags "-T 0x201000 -R 0x1000"`: the first segment at the
monitor's 2 MiB base; 6.2 MB, about 8 s) and appends it to the same
monitor, `bin/whim-guest-go` (9.8 MB).

*The boot path.* TamaGo's own amd64 `cpuinit` (`amd64/init.s`) expects to
be entered in 32-bit protected mode (PVH, as QEMU's microvm board is) or in
a long mode whose tables are its own: it points CR3 at 0x9000 and clears
0x9000-0xd000 *before* it checks the mode, then rebuilds an identity map of
1 GiB pages there -- which works under Firecracker only because
Firecracker's 64-bit boot puts its tables at those same addresses. The
monitor enters long mode with tables of its own (at 0x10000; text
executable and read-only, data no-execute, page 0 and the doorbell
unmapped), so the board brings its own `cpuinit`, the route TamaGo leaves
open (`linkcpuinit`), and imports nothing of `tamago/amd64`: the monitor
boots a Go image exactly as the C one -- the ELF's segments loaded, 64-bit
ring 0, the GDT, TSS and IDT, the entry at the ELF's (`_rt0_amd64_tamago`)
-- and tells it apart by the symbol `runtime/goos.RamStart`. What differs
is the slot (`vmm/tamago.go`): one span read-write from the image's end to
`GoRAMBytes` (3 GiB) above its base, the runtime's heap growing up from its
`bss` (sbrk) and its first stack under the top, the vectors' stack (IST1)
above that; no guard page, since Go's stacks check themselves. And four
registers more at the entry (`guest/abi`): RDX `RamStart`, RCX `RamSize`,
R8 `RamStackOffset`, R9 the TSC's rate in kHz (`KVM_GET_TSC_KHZ`,
`hv.TSCFrequency`: amd64 has no register that says it).

*The board's hooks* (`board/board.go`, `boot_amd64.s`):

| hook | answered by |
|---|---|
| `CPUInit` | `cpuinit`: the entry's registers kept, SSE on (CR0.EM off, MP on, CR4.OSFXSR and OSXMMEXCPT -- not OSXSAVE, so the runtime finds no AVX and no extended state needs saving), the stack at `RamStart+RamSize-RamStackOffset`, `_rt0_tamago_start` |
| `RamStart`, `RamSize`, `RamStackOffset` | the monitor's, in registers |
| `Hwinit0` | nothing: the monitor set the machine up |
| `Hwinit1` | `goos.Exit` and `goos.Idle` set |
| `Nanotime` | the TSC, scaled by the rate the monitor said (32.32 fixed point) -- **in the guest**: the scheduler asks it often, and a call costs 16 µs here |
| `Printk` | a line buffered, then `Message` to stderr -- **a call** |
| `GetRandomData` | `Random`, a new call (19): the host's `crypto/rand` into guest memory -- **a call**, once a run |
| `InitRNG` | nothing |
| `Exit` | `Exit` -- **a call**, never resumed |
| `Idle` | a spin to the scheduler's next timer: there are no interrupts, and the vCPU is stopped while a call is answered, so only a timer can make a goroutine runnable; with none it returns, and a deadlocked guest spins until the watchdog ends it |
| exceptions | the monitor's IDT into `whim_vectors`, 32 stubs 16 bytes apart in the board's assembly, reporting by `Fault` as the C guest's vectors do |

So the runtime needed nothing of the monitor but the one call `Random`
and the TSC's rate: **one vCPU, no interrupts, no timer**. With TamaGo's
single CPU (`GOMAXPROCS` 1, no `sysmon` on tamago), goroutines -- the
collector's workers, the parallel `:%s`'s chunks (`editor.Chunks`) -- are
scheduled cooperatively and the vCPU stops only in a call. Signals are the
C guest's: the call's `event`, the editor's `deathtrap` run in the guest's
`Host` and a wait or read made again (`whim_hcall`, in Go).

*Gate met:* `go tool whim test --guest-go` (`make whim-test-guest-go`): 80
of 80 answer exactly as the C does, its control (the launcher with the
image's one `" INSERT"` changed in its bytes) seen by 76; `--wide
--guest-go`: 240 of 240 (keys 102, ex 98, argv 30, pty 10), the control
seen by 94, 0, 0 and 6 -- as the C guest's and the Go editor's. Exits:
317,273 over the quick suite's 5,192 bytes of keys, 61.11 a key, 6.71 in
the cases not `par_*` (the C guest's 317,193 and 6.68, and one `Random` a
run); the wide suite 29,639, 5.75 a key. The heavy case: C 434-498 ms, the
Go guest 744-780 ms (1.6-1.8x; the C guest 1.1-1.6x), every one of its 20,787
exits a call; run by hand, 0.66-0.68 s inside `KVM_RUN` and 47-48 ms in
the monitor -- the exits about 0.33 s of it, as the C guest's -- against
the native Go editor's 0.21-0.25 s on all 64 cores and 0.24-0.26 s at
`GOMAXPROCS=1`; 55 MB resident against the C's 29 and the native Go's
44-52. A start and `:q!` is 40 ms and 35 MB. `guest`'s `TestGoGuest` (the
guest on a recording host writes exactly the bytes and exit code the native
Go editor does on it) and `TestGoGuestFault` (a null store a Fault through
the board's vectors, vector 14 at address 0; a panic the runtime's report
on the console and exit 2; a spin ended by the watchdog) run when
`TAMAGO_ROOT` is set and skip otherwise. `go vet` and staticcheck, run with
TamaGo's toolchain (`GOROOT=/root/tamago-go`, `GOOS=tamago`), are clean on
the module for amd64 and arm64.

*arm64: booted, the gate met.* `board/boot_arm64.s` is the same board for
arm64 -- `cpuinit` keeping X0-X4 (argc, argv, `RamStart`, `RamSize`,
`RamStackOffset`), FP and SIMD on (`CPACR_EL1.FPEN`), the counter's rate
from `CNTFRQ_EL0` and its value from `CNTVCT_EL0`, the doorbell one `STR`
from X0 (`str x0, [x1]`), `whim_vectors` 16 entries of 128 bytes at a
2 KiB boundary -- and `vmm/tamago_arm64.go` sets the three registers past
the C guest's. `go tool whim guest --go --arch arm64` builds the launcher
(9.1 MB; `--image` the image alone, 5.6 MB: its segments at 2 MiB, as
amd64's, the entry `_rt0_arm64_tamago`, `whim_vectors` at 0x3c1000). It
booted under KVM/arm64 in the aarch64 VM as first built, nothing changed:
the entry state the C guest's (EL1t on `SP_EL0`, D, A, I and F masked, the
MMU on over a 32-bit space -- 3 GiB above 2 MiB fits under the doorbell --
`VBAR_EL1` at `whim_vectors`, the store's PC handled as the C guest's);
EL1 reads `CNTFRQ_EL0` and `CNTVCT_EL0` under KVM without a trap to the
monitor, and TamaGo's arm64 rt0 needs nothing more (`g` in R28, no TLS
register, no EL change: its own `cpuinit` drops from EL3 or EL2 and turns
the MMU *off*, which ours does not). The same seccomp filter (kill mode)
holds it.

*Gate met, arm64* (`TestGuestPrebuilt`, `WHIM_SUITE_GUEST` the Go guest's
launcher, which the test now tells from the C guest's by its control's
literal -- the Go image's `" INSERT"` has no NUL after it): inside the
aarch64 VM (milestone 3, *The aarch64 VM*) the quick suite's 80 cases answer
exactly as the C built there does, the control seen by 76; the exits are
the amd64 Go guest's to the count, 317,273, 61.11 a key, 6.71 without
`par_*`. The test took 938 s, the guest's runs 597 s, at 4 cases at once
(`taskset -c 0-3`: the suite runs as many as `runtime.NumCPU()`) and a
60 s limit for the guest and its control (`WHIM_SUITE_LIMIT=60s`; the C's
stays 10 s). At the default, 8 at once and 10 s, it failed by time alone,
never by an answer: the Go guest's runs are heavier under emulation than
the C guest's (below), and 8 of them at once slowed the whole VM until
the C itself missed 10 s on `par_branch` and `insert`; at 4 at once with
10 s, the guest missed it on `append`.

With the wide suite (`WHIM_SUITE_WIDE=1`, the Ex commands from the
whim-vim.c the C was built from), the same settings: all 320 answer as the
C does there -- keys 80, its control seen by 76; keys 102, seen by 94; Ex
commands 98 and command lines 30, seen by 0; the pseudo-terminal 10, seen
by 6 -- the amd64 Go guest's counts; 346,912 exits for 10,345 bytes of
keys, 33.53 a key, 5.96 without `par_*`; 2,533 s, the guest's runs 1,815
s. `hv`'s three arm64 tests pass there too.

*Measured in the VM* (emulated: only ratios mean anything). The heavy
case, each twice, the answers byte for byte the same: the C 5-6 s, the C
guest 11-12 s, the Go guest 17-20 s -- 3.3 times the C, against 1.6-1.8 on
amd64 -- its 20,787 exits 14.4 s inside `KVM_RUN` (the C guest's 20,786,
9.2 s) and 2.3 s in the monitor (2.2 s), the longest run between two exits
1.3 s (0.38 s). A start and `:q!`: the C 0.02-0.03 s, the C guest
0.25-0.45 s, the Go guest 1.5-1.8 s and 33-35 MB (the C guest's 17), of
which 0.34 s inside `KVM_RUN` (the C guest's 0.08 s), 42 exits (41: one
`Random`), the first run the runtime's start, 0.2 s; the rest, about
1.2 s, is the monitor's process outside the loop under emulation (the VM
made and torn down), not broken down further. On amd64 here the same start is 0.02-0.04 s
for the Go guest and 0.01 s for the C guest. So a Go guest's run costs
the emulated VM about four times a C guest's, which is what the 8-at-once
default could not carry.

*What remained* -- the parallel `:%s` on several vCPUs -- is built:
*SMP: the Go guest on several vCPUs*, below.

### The merged wait and read

*The open question answered (*Open questions*), without changing
`editor.Host`, the core or any other editor: between the guest and the
monitor only.* The wait and the read were 20,740 of the heavy case's 20,786
exits and two of the 6.7 a typed key cost. Now a wait is one call that reads.

*The ABI.* A new call, `WaitRead` (`guest/abi`, 20; `vmm`'s `TestABI` holds
the monitor's `callWaitRead` to it): `a[0]` the wait's ms, `a[1]` a buffer in
guest memory, `a[2]` its length. The monitor calls `Host.WaitForInput(ms)`
and, when it says yes, `Host.ReadInput` into the buffer at once; `ret` is the
wait's answer and, when it is 1, `a[0]` the read's -- the count, 0 at the
end, -1 for a signal that came first. The monitor still answers
`wait_for_input` and `read_input` as before; neither guest makes the first
any more, and both make the second only when they hold nothing.

*The buffer* is the guest runtime's own: `whim_input` in `guest/rt/rt.c`,
`host.input` in `guest/tamago/main.go`. It is 250 bytes, the core's
`INBUFLEN`: the core reads in one place, `fill_input_buf`, which asks
`INBUFLEN - inbufcount`, and after the wait `WaitForChar` makes -- the wait
of every key typed -- `inbufcount` is 0 (`WaitForChar` returns before
waiting when vim's `inbuf` holds anything), so the read ahead is exactly the
read the C makes there. `guest.Source` holds the C's to the core
(`static_assert(WHIM_INPUT_BYTES == INBUFLEN)`), and the Go guest's is
`[editor.INBUFLEN]byte`. While the read's answer is kept, a wait answers 1
with no exit (the C's `select` says yes to bytes in the tty), and a read is
served from it with no exit -- as many bytes as it asks, in order, or the 0
or -1 the read returned; once all of it is the core's, a read is a call of
its own again, as is one of no length.

*Signals.* A deadly one (`SIGHUP`, `SIGTERM`) that the monitor catches during
the merged call unwinds it before a byte is read -- `editor/term`'s wait and
read both drain and deliver before they touch stdin -- so it comes back in
`event`, the core's `deathtrap` runs in the guest, and the call is made
again: as before, when it was a wait. One the editor reads as input
(`SIGINT`, `SIGWINCH` and `SIGCONT`, `SIGTSTP`) is written by the host's read
as its key sequence ahead of the tty's bytes, so it reaches the core first,
as the C's `read_input` gives it first. A signal at the merged call is
delivered in the order it was.

*Input read before the core asks for it.* The core does not always read
after a wait: `mch_delay`'s `WaitForChar` (a delay that input ends:
`'showmatch'`, `ui_delay(..., FALSE)`), `'writedelay'`'s wait in
`mch_write`, `inchar_loop` returning on a changed typeahead. What the guest
read there is held until the core reads. Between, the core may stop or end:
it has no `:!` (no processes: the shell escape is not a command of this
core), so the moments are `:suspend` -- `term_stop`, `SIGTSTP`, the shell
reading the tty while the editor is stopped, `term_start` -- a deadly signal,
and the exit. The C host reads the tty only in `musl_read_input`, so bytes a
wait saw and no read took are still the kernel's: across `:suspend` the
shell reads them, and at the exit they are left to it. **But the core itself
reads ahead in just this way**: `ui_breakcheck`, which every long
computation calls, is `mch_breakcheck`: `RealWaitForChar(0)` and, when there
is input, `fill_input_buf(FALSE)` -- up to `INBUFLEN` bytes into vim's own
`inbuf`, which `vgetorpeek` then moves to its typeahead (and `char_avail`
peeks the same way); bytes there survive a `:suspend` and reach the core
after it, and are lost at the exit. The guest's buffer is that, one level
down: what it holds is what the C, with the same keys typed a moment
earlier, holds in `inbuf`. So nothing the C would give the core is lost or
given to the shell; the shell gets less, never more. A signal that arrives
between the read ahead and the core's read comes after the held bytes,
where the C's would come before them -- which is the C's order for the same
signal a moment later, after the read; the delay is bounded by the held
bytes, at most 250, which the core's next reads take. One case reads less
than was read ahead: `fill_input_buf` after `mch_breakcheck`'s wait while
`inbuf` holds `n` bytes asks `INBUFLEN - n`; the rest is held for the next
read, in order, and that read gets only it, where the C's would take more
from the tty (a resize's sequence, which the C host writes only into
a read of 32 bytes or more, could then arrive in two reads, when `n` > 218).

*Tests* (`guest/readahead_test.go`). `TestReadAheadRuntime`: a stand-in core
holds `rt.c`'s buffer to its rules call by call -- a wait reads 8 bytes, a
read takes 3, across `musl_suspend` a wait answers from the buffer and a
read takes the other 5, then the resize pending since comes from the next
wait, two bytes more, the end of input kept as the bytes are, and a read
with nothing kept is its own call: 8 exits, 4 `wait_read`. `TestReadAheadC`
and `TestReadAheadGo` run the C guest (the core from `src/whim-vim.c`) and
the Go guest on a scripted host -- chunks of keys, each handed out by one
read, one typed while `:suspend` writes `stoptermcap`'s bytes with
`'writedelay'` on -- beside the native Go editor on the same script, and
require the same screens and exit: *suspend*, the keys read ahead before
the host's `Suspend` in the guest (after it natively) and still reaching the
core after it, in order; *winch*, a signal between -- `SIGWINCH` pending once
those keys are handed out, which the guest holds across `:suspend` -- the
keys first and the resize after, as natively; *term*, `SIGTERM` at a merged
call's read, delivered before a byte of it, the call made again.

*Measured* (amd64 here). Quick suite: 80 of 80 for both guests, the
controls seen by 76; exits 317,193 -> 204,862 for the C guest (61.09 ->
39.46 a key, 6.68 -> 3.74 without `par_*`) and 317,273 -> 204,942 for the
Go guest (61.11 -> 39.47, 6.71 -> 3.78), 112,331 waits that read and no
read of its own -- every read the core made was answered in the guest. Wide
suite: 240 of 240 for both, the controls seen by 94, 0, 0 and 6; 29,409 ->
17,181 exits (5.71 -> 3.33 a key) and 29,639 -> 17,411 (5.75 -> 3.38),
12,226 waits that read and 297 reads of their own. The heavy case, its
answers byte for byte the C's: 20,786 -> 10,416 exits (20,787 -> 10,417 for
the Go guest), every `wait_for_input` and `read_input` one `wait_read`; the
C 428-456 ms, the C guest 524-562 -> 356-383 ms (1.2 -> 0.8-0.9 times the
C) and the Go guest 761-794 -> 575-612 ms (1.8 -> 1.3-1.4), the time inside
`KVM_RUN` 0.45-0.49 -> 0.30-0.32 s and 0.67-0.70 -> 0.50-0.51 s. Under
the stress test's load (48 busy loops on the 64 cores, 10 runs a case, fed
from a file): 0 of 720 quick runs and 0 of 2,160 wide runs differ from
their case's first, on either guest.
arm64: both images built and the module vetted for arm64 (`GOARCH=arm64`,
and the Go guest by TamaGo), not run in the aarch64 VM since.

### SMP: the Go guest on several vCPUs

*Built on amd64, its gate met; arm64 built and vetted, not run; the Mac's
backend compiled, not run.* The Go editor's parallel `:%s` (`editor.Chunks`:
`match_lines` in chunks, on goroutines) now runs on several vCPUs, as the
design in *The second guest* had it, with one change (the call block).

*The monitor* (`vmm/smp.go`, `vmm.go`). `Config.CPUs` vCPUs (the launcher's
`WHIM_GUEST_CPUS`, 1 to 32 or `host`; by default four, or the host's CPUs
when fewer, on Linux amd64 -- below -- and one elsewhere, where several are
built and not yet run); a C guest always has one. All are created before
the guest runs, each by `hv.VCPUCreate` on a locked thread of its own --
`hv_vcpu_create`'s rule, which is how the framework does SMP; on KVM a
`KVM_CREATE_VCPU` each -- so that the seccomp filter, which goes on every
thread at once (`TSYNC`), finds them all there and allows no more. The
boot vCPU's exit loop is `Run`'s goroutine, as before; each other has its
own, waiting until the guest starts it. Every vCPU has the boot vCPU's
tables and system registers but its own TSS (amd64: 128 bytes apart in
the TSS page, so 32 at most) or `SP_EL1` (arm64), each its own 64 KiB fault
stack: a Go guest's slot has one per vCPU at the top and the runtime's
first stack under them, so one vCPU's slot is the slot as it was. The
Host's calls are made one at a time, as the C core makes them -- a vCPU
waits for the one before to be answered -- and the vCPUs' own calls wait
for nothing. A deadly signal during a Host call comes back in that call's
block, to whichever vCPU made it. The run ends with the first end on any
vCPU -- an exit, a fault, the watchdog (per vCPU: one in a call or parked
is not running the guest) -- which stops the rest (`hv.VCPUsExit`, and a
park or a wait for the Host given up) and waits for their loops; an exit's
counts are every vCPU's, summed, with a line `cpus N started M parked T`
past one vCPU, and the host's time without the parks'. Found on the way:
an exit from an AP stopping a vCPU another thread was destroying crashed
the monitor (13 of 30 runs at 16 vCPUs), and hv's KVM backend now holds
its VM's lock across `VCPUsExit` and `VCPUDestroy`, so that a vCPU is
found whole or not at all, as the framework's would be.

*Four calls* (`guest/abi`, 21-24; `vmm`'s `TestABI` holds them):
`CPUStart` (start vCPU a[0] at the image's `whim_apentry`: its stack, the
M's `g0`, the function -- the runtime's `mstart` -- and the top of the
stack TamaGo allocated for the M, whose pages the monitor gives back),
`CPUPark` (stop until a wake, a[0] ns, or the monitor's limit),
`CPUWake` (end vCPU a[0]'s park, or its next: a token, so that a wake
before the park is not lost) and `CPUSelf` (which vCPU this is). There are
no IPIs: TamaGo's own amd64 boards start an AP by INIT and SIPI and wake
one by an NMI or an interrupt; here the monitor does both.

*The call block is on the caller's stack*, aligned to its size, one per
call -- not one per CPU found by `ProcID`, as designed: nothing has to say
which vCPU a call is made on, and the goroutine that makes the Host's
calls does move between vCPUs (whichever vCPU readies it after the last
chunk runs it next, and its calls, the exit's among them, are that vCPU's).
`Call` is `nosplit` and the doorbell's argument `noescape`, so the block
stays on the stack and does not move during the call.

*The board* (`board/smp.go`, `whim_apentry` in `boot_ISA.s`). The monitor
says the vCPUs at the entry (R10; arm64 X5); with more than one, the
board's `init` sets `goos.Task`, `goos.Wake` and `goos.ProcID` and raises
`GOMAXPROCS` to their number (TamaGo's `osinit` says one CPU). TamaGo's
runtime never drops an M and binds one to each P, so `Task` is asked once
for each AP. The AP runs on its `g0`'s own stack, 16 KiB, as Linux's
`clone` is given it, so that `g0`'s bounds are the stack's; the 8 MiB
TamaGo allocates for it -- and clears, so touches -- is unused, and given
back (`madvise`; nothing on macOS, whose `syscall` has none): resident
for a `:%s` over 200,000 lines, 408 MB at 32 vCPUs before, 153 after
(111 at one). `whim_apentry` turns SSE (FP and SIMD) on as `cpuinit` does,
points FS at the AP's slot in the board's `tls` (the runtime's g at FS
less 8, as its `settls` lays it; arm64 keeps g in R28), and calls `mstart`.
`ProcID` is a call (once per M), `Wake` one, and `Idle` a spin of 20 µs,
then a park.

*When an idle vCPU looks again.* TamaGo's scheduler never wakes an M for
new work -- its `wakep` finds no idle P, since no M gives its P up -- so an
idle M must look again by itself, for a fork's goroutines to steal or a
stop of the world to join (TamaGo's own boards spin there: their idle
governor halts only for a `semasleep` with no deadline). The monitor
counts the vCPUs running the guest; while another runs, a park lasts at
most a limit (`vmm.DefaultPark`, 1 ms; `WHIM_GUEST_PARK`), and while none
does -- all parked, or in a Host call, as when the editor waits for a key --
nothing can make work but a Host call's return or a park's end, and a park
lasts until the next of those that leaves one vCPU running, which wakes
every vCPU so parked. So an idle editor costs nothing: three seconds
waiting for a key, 0.02-0.04 s of CPU at 1, 4 or 16 vCPUs alike. The
limit measured at 100 µs, 250 µs, 1 ms and 5 ms: the heavy case and the
`:%s` below within their noise from 100 µs to 1 ms, both slower at 5 ms.

*Gate met* (amd64 here). The quick suite 80 of 80 and the wide 240 of 240
at 1, 4 and 16 vCPUs and by default (4), as the C answers, the controls
seen by 76, and 94, 0, 0 and 6. At one vCPU the exits are the Go guest's
before, to the count -- 204,942 and 17,411 -- so one is the guest as it
was. At 4: 222,497-223,187 (42.9-43.0 a key, 4.1 without `par_*`) and
19,218-19,296; at 16: 239,048 (46.0) and 19,536 -- the parks and wakes. The
C guest unchanged: 80 of 80, 204,862 exits. Under the stress test's load
(48 busy loops on the 64 cores, 10 runs a case, from a file): 0 of 720
quick runs and 0 of 2,160 wide differ from their case's first, at 4 vCPUs
and at 16. `guest`'s `TestGoGuestSMP`: a session building 4,000 lines and
running a `:%s` and a `:g` across them, on a scripted host, answers as the
native Go editor does at 1, 2 and 4 vCPUs, every AP started and parked;
`hv`'s `TestVCPUsAMD64`: two vCPUs on two threads, both stopped by one
`VCPUsExit`, one run from the other's thread refused, one destroyed named
no more.

*Measured* (amd64 here; each session's median wall time, keys from a file).
A `:%s/\v(a|b)+c/X/g` over 200,000 lines (*PARALLEL-SUBSTITUTE.md*'s eight
lines, `ggVGy24999P`), the session less the same session without it; and
the heavy case:

| vCPUs or Ps | 1 | 2 | 4 | 8 | 16 | 32 |
|---|---|---|---|---|---|---|
| `:%s`, the Go guest | 5.84 s | 3.19 | 1.56 | 0.90 | 0.56 | 0.48 |
| `:%s`, the Go editor natively (`GOMAXPROCS`) | 3.67 s | 1.86 | 0.97 | 0.51 | 0.30 | 0.16 (64) |
| heavy case, the Go guest | 602 ms | 570 | 578 | 603 | 713 | 706 |
| heavy case, the Go editor natively | 250 ms | 233 | 228 | 208 | 223 | 235 (64) |

The C: 8.75 s for the `:%s`, 455 ms for the heavy case. **The `:%s` pays**:
10.4 times as fast on 16 vCPUs as on one (the native editor 12.2 times on
16 Ps), ahead of the C (at `-O0`) already on one, and 1.6-1.9 times the
native editor's time throughout. **The heavy case does not**, as expected: its matching is
a small part of it -- 5,000 lines -- and two to four vCPUs save about 5%,
eight nothing, 16 and 32 cost 18% (not broken down: more Ps, more of the
collector's workers to start and stop, and every stop of the world waiting
for the vCPUs to look again). So the default is four. A start and `:q!`:
28 ms at one vCPU, 32 at four, 34 at 16, 69 at 32.

*arm64* (`boot_arm64.s`, `vmm/setup_arm64.go`): the same, the count in X5,
an AP's number in X2, `SP_EL0` its stack and its own `SP_EL1`; built and
vetted, not run in the aarch64 VM, and the launcher's default there is one
vCPU.

## Running on the Mac

**Milestone 4's gate is met** (2026-10-04, reported by the user from the
M2 Max: macOS with Homebrew clang 23.1.1, arm64-apple-darwin23.6.0):
`guest/mac/mac.sh` run with every argument in sequence -- `check`, the
alignment test, `hv` (the framework's own tests, signed), `build`, `hello`,
`run`, `c`, `suite` and `suite --wide` -- all passing. So the guest runs
under Hypervisor.framework through the purego backend with no cgo, the
same `hv` code and image as on KVM/arm64, and answers the quick and wide
suites as the C built there does. The quick suite on the Mac: 80 of 80,
the control seen by 76; the guest's runs 1,590 ms (1,850-1,920 ms on
nested KVM here); **317,193 exits, 317,193 of them calls (112,331 reads),
61.09 a key, 6.68 without the `par_*` cases -- the same counts as on KVM
amd64**: the guest asks its host the same things, call for call, under
either hypervisor. The heavy case on the Mac is not yet measured
(`mac.sh heavy` runs it now: *The Go guest and the heavy case on the Mac*,
below). The wide suite on the Mac first hung in its pseudo-terminal cases: Go's
poller (kqueue) does not poll a tty on macOS, so the suite stopped reading
the master, the editor stalled, and once killed could not finish exiting
with its output undrained (fixed in `internal/suite/pty_darwin.go`: the
master a blocking descriptor; and `pty.go` closes it at the limit). Then
`mac.sh suite --wide`: 80 quick and 102 keys, 98 Ex, 30 argv and 10 pty
cases, all answering as the C does, the controls seen by 76 and
94/0/0/6; 2,555 ms of guest runs; 346,602 exits, all calls (124,854 reads),
5.92 a key outside the `par_*` cases.

Milestone 4's groundwork is done here, so that on the M2 Max only building,
signing and running remain. Everything below was built for `darwin/arm64`
with cgo off and vetted here (`go build`, `go vet` and staticcheck on the
whole module, `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0`, clean); none of it
has run on macOS.

**What was ported.**

- `editor/term`, the terminal host, on macOS: `term.go` keeps what is the
  same, `term_linux.go` and `term_darwin.go` what is not -- `TIOCGETA` and
  `TIOCSETA` for `TCGETS` and `TCSETS`, `OXTABS` for `XTABS`, `pipe` and
  `fcntl` for `pipe2`, a `select` that reports a count and leaves the time
  remaining in its timeout as Linux's does (macOS's does neither, and the
  host resumes a wait with it), and `:suspend` through libc's
  `sigaction`, `pthread_sigmask` and `sigpending` (`internal/libsys`, bound
  by purego: the Go runtime installs its handlers through libc, so the
  kernel's form cannot restore them), with Linux's semantics: SIGTSTP at
  its default action, blocked on the locked thread for the kill, the wait
  bounded by a second. The signals are caught as on Linux (`os/signal`),
  SIGHUP and SIGTERM queued for `deathtrap`. It builds for `darwin/amd64`
  too; the Linux host is unchanged (the suites below).
- `vmm`: `Die` on macOS (`die_darwin.go`: SIGSEGV's action back to the
  default through libc, the signal sent to the thread); `Seccomp` is nil
  outside Linux; the watchdog was already `hv.VCPUsExit`, which is
  `hv_vcpus_exit` there.
- The image beside the monitor. `codesign` refuses a program with bytes past
  its last segment, so the Mac's launcher cannot carry the image appended:
  `whim-guest` loads `WHIM_GUEST_IMAGE` when it is set, else the image
  appended to it, else `PROGRAM.elf` beside it; `go tool whim guest
  --image` writes the image alone into `lib/whim-guest/` (`--arch arm64`:
  `whim-guest-arm64.elf`; with `--hello`, `whim-guest-hello-arm64.elf`),
  and `mac.sh` names it by `WHIM_GUEST_IMAGE`. `TestGuestPrebuilt` takes
  `WHIM_SUITE_GUEST_IMAGE`: the control is then the image changed, each run
  through a script that hands the monitor its image.
- The whole module builds for `darwin/arm64`: the tools' `Pdeathsig`, which
  only Linux has, is `internal/procattr` (none elsewhere), and the suite's
  pseudo-terminal is opened by macOS's `TIOCPTYGRANT`, `TIOCPTYUNLK` and
  `TIOCPTYGNAME` (`internal/suite/pty_darwin.go`). For `darwin/amd64` all
  but `hv` and what imports `vmm` build: there is no Hypervisor.framework
  backend for x86. `crefactor/`'s two tests that set `Pdeathsig` do not
  build for darwin; nothing on the Mac needs them.
- `hv`'s arm64 tests -- the smallest guests: a doorbell store, a store made
  again, the system registers, a spin ended by `VCPUsExit` -- skip only on a
  Linux without `/dev/kvm`, so on the Mac they are the framework's first
  test.
- **The C reference** built on the Mac by Apple's clang:
  `whim-vim.c`'s host region is POSIX but for three things gcc on Linux
  allows, which `guest/mac/shim.h`, included ahead of the file, spells for
  macOS -- `XTABS` (`OXTABS`), `pipe2` (`pipe` and `fcntl`), and
  `__builtin_setjmp`/`__builtin_longjmp`, which clang does not offer on
  arm64 at all (`clang --target=arm64-apple-macos -fsyntax-only` says *not
  supported for the current target*; `_setjmp`/`_longjmp` on a buffer of
  the shim's). There is no macOS SDK here, so the check against Apple's
  headers waits for the Mac; built with the shim here by clang (`-std=gnu23
  -O0`, dynamic: macOS links nothing statically), the C answers the quick
  and wide suites as the guest does -- the Mac's whole flow (the monitor
  with no image appended, the image beside it, the shim's C) run on KVM:
  80 of 80 and 240 of 240, the controls seen by 76 and 94/0/0/6.
  The first build on the Mac (2026-10-04, Homebrew clang 23.1.1,
  arm64-apple-darwin23.6.0) found two more: `gettimeofday`, which macOS
  declares only in `<sys/time.h>` -- the shim declares it as macOS does,
  with no header (a header included ahead of the file would define names
  the core declares as its own, `INT_MAX`, `PATH_MAX`); and the host's
  assertion that the core's `PATH_MAX`, 4096, is the header's, 1024 on
  macOS -- the host uses `PATH_MAX` nowhere else, so `mac.sh c` compiles a
  copy without that one line (`.tmp/whim-vim-mac.c`).
- **The 16 KiB alignment** `hv_vm_map` asks on Apple silicon:
  `vmm/layout_test.go` holds the layout to it -- the monitor maps one
  slot, guest-physical 0 to its size, which is a multiple of 2 MiB for any
  image; the doorbell page outside it and below a 36-bit IPA space -- and
  maps a slot as the monitor does and holds its host address to the page
  (16 KiB where that is the page: on the Mac it checks the real thing).

**The Go guest and the heavy case on the Mac.** The Go guest runs under the
same signed `bin/whim-guest` as the C guest, its image
`lib/whim-guest/whim-guest-go-arm64.elf` (`go tool whim guest --go --arch
arm64 --image`, 5.6 MB): `mac.sh` takes `--go` after
its step's name -- `hello --go` (the Go guest has no stand-in: the editor
itself, `ihello<Esc>:q!` from a file, exit 0), `run --go`, `suite --go
[--wide]`, `heavy [--go]` -- and hands the monitor that image by
`WHIM_GUEST_IMAGE`. `TestGuestPrebuilt` tells the Go image from the C's by
its control's literal as it told the launchers (`controlLiteral`: no
`" INSERT"` followed by a NUL in it). The heavy case is
`WHIM_SUITE_HEAVY` in `TestGuestPrebuilt`: `1` runs it after the cases,
`only` alone (`mac.sh heavy`); `heavy.go`'s 5,000 lines, three `:s` and a
`:g`, the C first as the reference, then the C and the guest timed one at
a time, the answers held to the C's and the guest to the suite's bound of
25 times the C's time, the times on the suite's heavy line. *Checked on
KVM here* (2026-10-04), the Mac's flow with the amd64 images standing in --
the monitor with no image appended, the images beside it, the C by clang
with the shim, `codesign` a stand-in that prints its arguments: `hello
--go` exits 0 with hello on its screen; `suite --go` 80 of 80, the control
seen by 76; `suite --go --wide` 320 of 320 (80, 102, 98, 30, 10), the
controls seen by 76, 94, 0, 0, 6; `suite` 80 of 80, seen by 76, as before;
`heavy` C 446-463 ms, the guest 359-397 ms (0.8-0.9x); `heavy --go` C
434-464 ms, the Go guest 594-608 ms (1.3-1.4x) -- the C here the shim's,
by clang at `-O0`, dynamic.

**Signing, and the sandbox.** `guest/mac/whim-guest.entitlements` holds
`com.apple.security.hypervisor` alone; `codesign --sign - --entitlements
guest/mac/whim-guest.entitlements --force bin/whim-guest` signs ad hoc.
*The app sandbox is not taken.* It applies at `exec`, not once the VM is
built as the seccomp filter does, and a sandboxed program may open no file
outside its container but those the user picks: the image beside the
monitor or in `WHIM_GUEST_IMAGE`, `WHIM_GUEST_STATS`'s file, the suite's
keys and scripts in `TMPDIR`. The terminal itself would be no obstacle --
descriptors inherited from the shell are not checked again -- but the files
are: the image would have to be in the signed binary, and the suite's
control, an image changed, signed afresh on every run. The counterpart of
the seccomp filter, should one be wanted, is `sandbox_init` with a profile
of our own, applied in `Config.Seccomp`'s place once the VM is built: a
Mac-only follow-up, not tried.

**The checklist**, in order:

1. *On Linux, here:* the three images, `make mac-images
   TAMAGO_ROOT=/root/tamago-go` (clang and `aarch64-none-elf-ld`: Apple's
   clang has no ELF linker; TamaGo for the Go guest's), into
   `lib/whim-guest/`; copy them to the same place in the Mac's checkout,
   names unchanged: `scp lib/whim-guest/*.elf mac:go-whim/lib/whim-guest/`.
   The arm64 images are the ones KVM/arm64 boots.
2. *On the Mac*, in the checkout (Go and Xcode's command-line tools; every
   Go build native and `CGO_ENABLED=0`), `guest/mac/mac.sh` runs each step:
   - `mac.sh check` -- `sw_vers`, `uname -m` (`arm64`, not Rosetta),
     `sysctl kern.hv_support` (1), `go version`, `clang --version`;
   - `CGO_ENABLED=0 go test ./vmm/` -- the alignment, checked on a 16 KiB
     page;
   - `mac.sh hv` -- `hv`'s arm64 tests, the test binary signed: the
     framework itself, with no monitor and no image;
   - `mac.sh build` -- `bin/whim-guest`, signed, its entitlements printed;
   - `mac.sh hello` -- prints `hello`, exits 3 (milestone 1 on the Mac);
   - `mac.sh run FILE` -- the editor, by hand;
   - `mac.sh c` -- the C reference, `bin/whim-vim-mac`: `clang -std=gnu23
     -O0 -w -include guest/mac/shim.h` (an older Xcode: `-std=gnu2x`);
   - `mac.sh suite`, then `mac.sh suite --wide` -- `TestGuestPrebuilt`,
     the guest held to the C with its control: milestone 4's gate, 80 and
     240 (done);
   - `mac.sh heavy` -- the heavy case, the C and the C guest timed one at a
     time: the native speed figure; paste its `heavy` line;
   - the Go guest, the same monitor (no `build` again): `mac.sh hello
     --go` (hello on its screen, exit 0), `mac.sh run --go FILE` by hand,
     `mac.sh suite --go`, `mac.sh suite --go --wide`, `mac.sh heavy --go`.
     Paste back from each suite its case lines (`N cases: ... answers all
     exactly as the C does; its control seen by M`), the exits line and
     the `PASS`/`FAIL`; from each heavy, its `heavy` line.

**What to look for when it fails.**

- `HV_DENIED` from `hv_vm_create`: the program is not signed with the
  entitlement -- or was changed after signing. `codesign --display
  --entitlements - bin/whim-guest`; `mac.sh build` again. Never append to a
  signed program.
- `Killed: 9` as it starts: the signature is invalid (the same causes); the
  reason is in Console, under `kernel` or `amfid`.
- `HV_UNSUPPORTED` or `HV_NO_DEVICE`: no virtualization here --
  `kern.hv_support` 0, a monitor built for amd64 under Rosetta, or macOS
  itself in a VM without nested virtualization.
- `HV_BAD_ARGUMENT` from `hv_vm_map`: the alignment -- `go test ./vmm/`
  says which of the address, the IPA and the size.
- `HV_NO_RESOURCES` from `hv_vm_map`: the 1.1 GiB slot refused; the slot is
  mapped lazily by `mmap`, so this would be the framework's own limit.
- The Go guest failing where the C guest passes: its slot is 3 GiB above
  its base (`vmm/tamago.go`), not 1.1 GiB -- `HV_NO_RESOURCES` there is that
  size; a start that spins until the watchdog is the runtime's clock
  (`CNTFRQ_EL0`, `CNTVCT_EL0`, read by the board at EL1), which KVM/arm64
  gave without a trap and the framework may not.
- `the guest stopped: HV_EXIT_REASON_VTIMER_ACTIVATED`: the guest's virtual
  timer fired, which it never arms; the framework would want it masked
  (`hv_vcpu_set_vtimer_mask`), a function `hv` does not bind yet.
- `the guest touched ... with no memory there` at the first instructions:
  the system registers (`setup_arm64.go`: the 4 KiB granule, `TCR_EL1`'s
  IPS against the chip's IPA size) are where KVM/arm64 and the framework
  could differ.
- A key that never arrives, `:suspend` that does not stop, a wait that does
  not end: `editor/term`'s macOS half (`term_darwin.go`), which runs for
  the first time there; `bin/whim` (the Go editor, `go build -o bin/whim
  ./editor/cmd/whim`) runs the same host without a VM, to tell the two
  apart.

**What only the Mac can do**: run any of it -- the framework under the
monitor, the terminal host on macOS, the shim's C against Apple's headers,
the signature and its entitlement -- and so milestone 4's gate: the suite,
80 and 240, and the heavy case's time against the C built there.

## Open questions

- Is one 4 GiB lazily committed slot enough, or should the allocator ask the
  VMM to map more (a sixteenth hypercall) for very large files? *As built*,
  the slot is the image, a 64 MiB stack and a 1 GiB heap, 1.1 GiB: the heap
  is the C host's arena to the byte, so the guest runs out where the C
  does, in its words. No sixteenth call while the C's arena is 1 GiB.
- Should `musl_wait_for_input` and `musl_read_input` be merged into one call
  (wait and read when ready), halving the exits per key? *Measured:* they
  are 20,740 of the heavy case's 20,786 exits and two of the 6.7 a typed
  key costs; merged, the heavy case would lose about 10,370 exits, 0.17 s
  of its 0.5 s here. Not done at first: it would change `editor.Host`,
  which every editor's host shares, for the guest's sake alone. *Done
  between the guest and the monitor only* (*The merged wait and read*): a
  wait is one call that reads into the guest runtime's buffer, `editor.Host`
  unchanged; the heavy case 10,416 exits, 0.36-0.38 s, 0.8-0.9 times the C.
- The parallel `:%s` of the translated editors on several vCPUs: a fork/join
  in shared memory with a halt-and-kick protocol, with no new hypercall --
  worth it only once a single vCPU is measured. *Measured* on the Go guest
  (*The second guest*): one vCPU, 1.6-1.8x the C in the heavy case, 0.33 s of
  it exits. *Built* (*SMP: the Go guest on several vCPUs*), with four calls
  and a park in the monitor in place of a kick: a `:%s` over 200,000 lines
  10.4 times as fast on 16 vCPUs, the heavy case no faster.
- Where does the guest's C reference come from on the Mac for the suite:
  `whim-vim.c` built there (its host region is POSIX), or outputs recorded
  here? *The aarch64 VM answered it one way:* the C built where the guest
  runs, and `TestGuestPrebuilt` cross-compiled to compare the two there.
  *The Mac the same way:* `editor/term` is ported and the C built there by
  Apple's clang with `guest/mac/shim.h` (*Running on the Mac*).
