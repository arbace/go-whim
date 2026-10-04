package guest

import "syscall"

// procAttr has a tool the builder runs die with it.
func procAttr() *syscall.SysProcAttr { return &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL} }
