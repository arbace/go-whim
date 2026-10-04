package vmm

// x86-64's numbers (arch/x86/entry/syscalls/syscall_64.tbl).
const (
	auditArch  = 0xc000003e // AUDIT_ARCH_X86_64
	sysIoctl   = 16
	sysSeccomp = 317
)

// read write close fstat mmap mprotect munmap brk rt_sigaction
// rt_sigprocmask rt_sigreturn select sched_yield madvise nanosleep getpid
// clone exit kill fcntl sigaltstack gettid tkill futex sched_getaffinity
// rt_sigpending restart_syscall clock_gettime clock_nanosleep exit_group epoll_wait
// epoll_ctl tgkill openat newfstatat pselect6 set_robust_list epoll_pwait
// epoll_create1 pipe2 prlimit64 getrandom membarrier rseq clone3
var allowed = []uint32{0, 1, 3, 5, 9, 10, 11, 12, 13, 14, 15, 23, 24, 28, 35, 39,
	56, 60, 62, 72, 127, 131, 186, 200, 202, 204, 219, 228, 230, 231, 232, 233, 234, 257, 262, 270, 273, 281,
	291, 293, 302, 318, 324, 334, 435}

// KVM_GET_SREGS and KVM_SET_SREGS: the system registers set up before the
// first run reach KVM at it, under the filter.
var kvmIoctls = []uint32{0x8138ae83, 0x4138ae84}
