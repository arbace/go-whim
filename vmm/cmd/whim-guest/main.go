// Command whim-guest is the editor as a virtual machine: the monitor
// (package vmm) with the guest image appended to it (go tool whim guest),
// run on the terminal host, editor/term, as bin/whim runs the Go editor.
//
// WHIM_GUEST_STATS=FILE writes the run's counts to FILE when it ends;
// WHIM_GUEST_WATCHDOG=DURATION (default 60s, 0 off) ends a guest that runs
// that long without a hypercall .
package main

import (
	"errors"
	"fmt"
	"os"
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
	img, err := vmm.Appended(exe)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg := vmm.Config{Image: img, Host: term.New(), Args: os.Args, Watchdog: 60 * time.Second}
	if p := os.Getenv("WHIM_GUEST_STATS"); p != "" {
		f, err := os.Create(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, "whim-guest:", err)
			os.Exit(1)
		}
		cfg.Stats = f
	}
	if w := os.Getenv("WHIM_GUEST_WATCHDOG"); w != "" {
		d, err := time.ParseDuration(w)
		if err != nil {
			fmt.Fprintln(os.Stderr, "whim-guest: WHIM_GUEST_WATCHDOG:", err)
			os.Exit(1)
		}
		cfg.Watchdog = d
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
