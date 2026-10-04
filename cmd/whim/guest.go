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
// "hello" and exits 3; --bench a stand-in that makes argv[1] calls of
// host_time, to time a hypercall; --alt the alternative trap (an out to a
// port on amd64, an HVC on arm64) in place of the doorbell.
//
//	whim guest [--arch amd64|arm64] [--hello|--bench] [--alt] [-o OUT] [FILE]
func runGuest(args []string) int {
	a := guest.Native()
	src, out, standIn := "src/whim-vim.c", "", ""
	alt := false
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--arch" && i+1 < len(args):
			i++
			var ok bool
			if a, ok = guest.Arches[args[i]]; !ok {
				fmt.Fprintf(os.Stderr, "whim guest: no arch %s (amd64, arm64)\n", args[i])
				return 2
			}
		case args[i] == "--hello" || args[i] == "--bench":
			standIn = args[i][2:]
		case args[i] == "--alt":
			alt = true
		case args[i] == "-o" && i+1 < len(args):
			i++
			out = args[i]
		case len(args[i]) > 0 && args[i][0] != '-':
			src = args[i]
		default:
			fmt.Fprintln(os.Stderr, "usage: whim guest [--arch amd64|arm64] [--hello|--bench] [--alt] [-o OUT] [FILE]")
			return 2
		}
	}
	if alt {
		a.Flags = append(append([]string{}, a.Flags...), "-DWHIM_TRAP_ALT")
	}
	if out == "" {
		out = filepath.Join("bin", "whim-guest")
		if standIn != "" {
			out += "-" + standIn
		}
		if alt {
			out += "-alt"
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
	if standIn != "" {
		err = guest.BuildStandIn(standIn, a, dir, out)
	} else {
		err = guest.Build(src, a, dir, out)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "whim guest:", err)
		return 1
	}
	what := src + "'s core"
	if standIn != "" {
		what = "rt/" + standIn + ".c"
	}
	fmt.Printf("  guest        %s: %s on %s\n", out, what, a.Name)
	return 0
}
