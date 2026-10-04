package vmm

import (
	"syscall"
	"unsafe"
)

// Die ends the process by SIGSEGV at its default action, as the C editor
// dies of a fault: the Go runtime's own handler is taken off first, since
// it would turn the signal into a crash report and exit status 2.
func Die() {
	var dfl [4]uint64 // struct kernel_sigaction: SIG_DFL, no flags
	syscall.RawSyscall6(syscall.SYS_RT_SIGACTION, uintptr(syscall.SIGSEGV), uintptr(unsafe.Pointer(&dfl)), 0, 8, 0, 0)
	var set uint64 = 1 << (syscall.SIGSEGV - 1)
	syscall.RawSyscall6(syscall.SYS_RT_SIGPROCMASK, 1 /* SIG_UNBLOCK */, uintptr(unsafe.Pointer(&set)), 0, 8, 0, 0)
	syscall.Tgkill(syscall.Getpid(), syscall.Gettid(), syscall.SIGSEGV)
	select {}
}
