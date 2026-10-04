package vmm

import "syscall"

// dontNeed gives b's pages back to the host: read as zeros after.
func dontNeed(b []byte) { syscall.Madvise(b, syscall.MADV_DONTNEED) }
