package vmm

// dontNeed would give b's pages back to the host; the syscall package has
// no madvise on macOS, and the pages stay (an AP's unused stack, 8 MiB
// each, touched by the guest's runtime as it cleared them).  NOT RUN
// here: built for darwin as a compile check.
func dontNeed([]byte) {}
