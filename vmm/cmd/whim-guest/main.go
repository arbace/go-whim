// Command whim-guest is the editor as a virtual machine: the monitor
// (package vmm) with the guest image appended to it (go tool whim guest),
// run on the terminal host, editor/term, as bin/whim runs the Go editor.
//
// The image is WHIM_GUEST_IMAGE=FILE when that is set, else the one appended
// to the program, else the file beside it named as it is with .elf added
// (bin/whim-guest.elf): on macOS, whose codesign refuses a program with bytes
// past its last segment, the image is a file of its own.
//
// WHIM_GUEST_STATS=FILE appends the run's counts to FILE when it ends;
// WHIM_GUEST_WATCHDOG=DURATION (default 60s, 0 off) ends a guest that runs
// that long without a hypercall; WHIM_GUEST_SECCOMP=0 leaves the system-call
// filter off, =log logs what it would deny.  WHIM_GUEST_CPUS=N gives a Go
// guest N vCPUs (doc/GUEST.md, *SMP*; at most vmm.MaxCPUs, "host" the
// host's CPUs so capped; by default defaultCPUs), WHIM_GUEST_PARK=DURATION
// the longest a parked one waits while another runs (vmm.DefaultPark).
package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/arbace/go-whim/editor/term"
	"github.com/arbace/go-whim/vmm"
)

func main() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "whim-guest:", err)
		os.Exit(1)
	}
	img, err := image(exe)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg := vmm.Config{Image: img, Host: term.New(), Args: os.Args, Watchdog: 60 * time.Second}
	if p := os.Getenv("WHIM_GUEST_STATS"); p != "" {
		f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
		if err != nil {
			fmt.Fprintln(os.Stderr, "whim-guest:", err)
			os.Exit(1)
		}
		cfg.Stats = f
	}
	if os.Getenv("WHIM_GUEST_SECCOMP") != "0" {
		cfg.Seccomp = vmm.Seccomp
	}
	if w := os.Getenv("WHIM_GUEST_WATCHDOG"); w != "" {
		d, err := time.ParseDuration(w)
		if err != nil {
			fmt.Fprintln(os.Stderr, "whim-guest: WHIM_GUEST_WATCHDOG:", err)
			os.Exit(1)
		}
		cfg.Watchdog = d
	}
	cfg.CPUs = defaultCPUs()
	if c := os.Getenv("WHIM_GUEST_CPUS"); c != "" {
		n, err := strconv.Atoi(c)
		if c == "host" {
			n, err = min(runtime.NumCPU(), vmm.MaxCPUs), nil
		}
		if err != nil || n < 1 || n > vmm.MaxCPUs {
			fmt.Fprintf(os.Stderr, "whim-guest: WHIM_GUEST_CPUS=%s: not 1 to %d, or host\n", c, vmm.MaxCPUs)
			os.Exit(1)
		}
		cfg.CPUs = n
	}
	if p := os.Getenv("WHIM_GUEST_PARK"); p != "" {
		d, err := time.ParseDuration(p)
		if err != nil || d <= 0 {
			fmt.Fprintln(os.Stderr, "whim-guest: WHIM_GUEST_PARK: not a positive duration:", p)
			os.Exit(1)
		}
		cfg.Park = d
	}
	code, err := vmm.Run(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		var f *vmm.Fault
		if errors.As(err, &f) {
			vmm.Die()
		}
		os.Exit(1)
	}
	os.Exit(code)
}

// defaultCPUs is a Go guest's vCPUs when WHIM_GUEST_CPUS does not say:
// four, or the host's CPUs when fewer, on Linux amd64, where they were
// measured (doc/GUEST.md, *SMP*: a :%s over 200,000 lines 3.7 times as
// fast as on one, the heavy case no slower, more costing the heavy case
// more than its matching gains); one elsewhere, where several are built
// and not yet run.
func defaultCPUs() int {
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		return min(runtime.NumCPU(), 4)
	}
	return 1
}

// image is the guest: WHIM_GUEST_IMAGE's file, the image appended to exe,
// or exe.elf.
func image(exe string) ([]byte, error) {
	if p := os.Getenv("WHIM_GUEST_IMAGE"); p != "" {
		return os.ReadFile(p)
	}
	img, err := vmm.Appended(exe)
	if !errors.Is(err, vmm.ErrNoImage) {
		return img, err
	}
	if b, rerr := os.ReadFile(exe + ".elf"); rerr == nil {
		return b, nil
	}
	return nil, err
}
