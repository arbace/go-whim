package vmm

// arm64's numbers (include/uapi/asm-generic/unistd.h).
const (
	auditArch  = 0xc00000b7 // AUDIT_ARCH_AARCH64
	sysIoctl   = 29
	sysSeccomp = 277
)

// read write close fstat mmap mprotect munmap brk rt_sigaction
// rt_sigprocmask rt_sigreturn sched_yield madvise nanosleep getpid clone
// exit kill fcntl sigaltstack gettid tkill futex sched_getaffinity
// rt_sigpending restart_syscall clock_gettime clock_nanosleep exit_group epoll_ctl tgkill
// openat newfstatat pselect6 set_robust_list epoll_pwait epoll_create1
// pipe2 prlimit64 getrandom membarrier rseq clone3
var allowed = []uint32{63, 64, 57, 80, 222, 226, 215, 214, 134, 135, 139, 124, 233, 101, 172,
	220, 93, 129, 25, 132, 136, 178, 130, 98, 123, 128, 113, 115, 94, 21, 131, 56, 79, 72, 99, 22, 20,
	59, 261, 278, 283, 293, 435}

// KVM_GET_ONE_REG and KVM_SET_ONE_REG: X0 and PC read, and PC moved, at a
// doorbell's exit.
var kvmIoctls = []uint32{0x4010aeab, 0x4010aeac}
