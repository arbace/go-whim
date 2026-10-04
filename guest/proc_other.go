//go:build !linux

package guest

import "syscall"

// procAttr: no parent-death signal outside Linux.
func procAttr() *syscall.SysProcAttr { return nil }
