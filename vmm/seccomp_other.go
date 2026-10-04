//go:build !linux

package vmm

// Seccomp is nil outside Linux: there is no system-call filter to put on.
// On macOS the monitor's hardening is the app sandbox, which its signature's
// entitlements ask for (doc/GUEST.md, *Running on the Mac*).
var Seccomp func() error
