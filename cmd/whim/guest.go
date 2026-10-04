package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/arbace/go-whim/guest"
)

// runGuest builds the editor as a virtual machine (doc/GUEST.md): the core
// of FILE (default src/whim-vim.c) compiled freestanding with the guest
// runtime into an image for ARCH (default this machine's), the monitor
// built for ARCH, and the image appended to it at OUT (default
// bin/whim-guest, or bin/whim-guest-ARCH for another ISA).  --hello builds
// milestone 1's guest instead: the runtime with a stand-in that writes
// "hello" and exits 3.
//
//	whim guest [--arch amd64|arm64] [--hello] [-o OUT] [FILE]
func runGuest(args []string) int {
	a := guest.Native()
	src, out, hello := "src/whim-vim.c", "", false
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--arch" && i+1 < len(args):
			i++
			var ok bool
			if a, ok = guest.Arches[args[i]]; !ok {
				fmt.Fprintf(os.Stderr, "whim guest: no arch %s (amd64, arm64)\n", args[i])
				return 2
			}
		case args[i] == "--hello":
			hello = true
		case args[i] == "-o" && i+1 < len(args):
			i++
			out = args[i]
		case len(args[i]) > 0 && args[i][0] != '-':
			src = args[i]
		default:
			fmt.Fprintln(os.Stderr, "usage: whim guest [--arch amd64|arm64] [--hello] [-o OUT] [FILE]")
			return 2
		}
	}
	if out == "" {
		out = filepath.Join("bin", "whim-guest")
		if hello {
			out += "-hello"
		}
		if a.Name != guest.Native().Name {
			out += "-" + a.Name
		}
	}
	dir, err := os.MkdirTemp("", "guest.")
	if err != nil {
		fmt.Fprintln(os.Stderr, "whim guest:", err)
		return 1
	}
	defer os.RemoveAll(dir)
	if hello {
		err = guest.BuildHello(a, dir, out)
	} else {
		err = guest.Build(src, a, dir, out)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "whim guest:", err)
		return 1
	}
	fmt.Printf("  guest        %s: %s's core on %s\n", out, src, a.Name)
	return 0
}
