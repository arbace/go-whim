//go:build !linux

package procattr

import "syscall"

// Child: no parent-death signal outside Linux.
func Child() *syscall.SysProcAttr { return nil }

// Group leads a process group of its own (no parent-death signal outside
// Linux).
func Group() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setpgid: true} }
