// Package procattr is the attributes a tool gives the programs it starts:
// on Linux they die with it (Pdeathsig), so a tool or a test killed from
// outside leaves nothing running; elsewhere there is no parent-death signal
// and they are started plainly.
package procattr

import "syscall"

// Child dies with its parent.
func Child() *syscall.SysProcAttr { return &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL} }

// Group dies with its parent and leads a process group of its own.
func Group() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
}
