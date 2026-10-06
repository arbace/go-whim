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
// port on amd64, an HVC on arm64) in place of the doorbell.  --image writes
// the image alone (default lib/whim-guest/whim-guest[-STANDIN][-alt][-ARCH].elf),
// for a monitor built elsewhere: the Mac's, told it by WHIM_GUEST_IMAGE
// (doc/GUEST.md, *Running on the Mac*).  --go builds the second guest
// instead: the Go editor, editor/ as it stands, built with TamaGo
// (guest/tamago/; the distribution $TAMAGO_ROOT or --tamago DIR) and
// appended to the same monitor, at bin/whim-guest-go (with --image, the
// image alone).  --joker
// builds Joker, a Clojure dialect, as a REPL on the console with TamaGo
// (guest/joker; doc/LISP-SANDBOX.md), at bin/whim-guest-joker: the graph
// views in Joker loaded (guest/joker/gview) and the monitor's store the
// namespace box: the data the image no longer holds, in a store `whim
// store` writes (WHIM_GUEST_STORE=DIR).
//
//	whim guest [--arch amd64|arm64] [--hello|--bench|--go [--tamago DIR]|--joker [--tamago DIR]] [--alt] [--image] [-o OUT] [FILE]
func runGuest(args []string) int {
	a := guest.Native()
	src, out, standIn := "src/whim-vim.c", "", ""
	alt, imageOnly, goGuest, jokerGuest, tamago := false, false, false, false, ""
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
		case args[i] == "--image":
			imageOnly = true
		case args[i] == "--go":
			goGuest = true
		case args[i] == "--joker":
			jokerGuest = true
		case args[i] == "--tamago" && i+1 < len(args):
			i++
			tamago = args[i]
		case args[i] == "-o" && i+1 < len(args):
			i++
			out = args[i]
		case len(args[i]) > 0 && args[i][0] != '-':
			src = args[i]
		default:
			fmt.Fprintln(os.Stderr, "usage: whim guest [--arch amd64|arm64] [--hello|--bench|--go [--tamago DIR]|--joker [--tamago DIR]] [--alt] [--image] [-o OUT] [FILE]")
			return 2
		}
	}
	if alt {
		a.Flags = append(append([]string{}, a.Flags...), "-DWHIM_TRAP_ALT")
	}
	if out == "" {
		out = filepath.Join("bin", "whim-guest")
		if goGuest {
			out += "-go"
		}
		if jokerGuest {
			out += "-joker"
		}
		if standIn != "" {
			out += "-" + standIn
		}
		if alt {
			out += "-alt"
		}
		if a.Name != guest.Native().Name {
			out += "-" + a.Name
		}
		if imageOnly {
			out = filepath.Join("lib", "whim-guest", filepath.Base(out)+".elf")
		}
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "whim guest:", err)
		return 1
	}
	dir, err := os.MkdirTemp("", "guest.")
	if err != nil {
		fmt.Fprintln(os.Stderr, "whim guest:", err)
		return 1
	}
	defer os.RemoveAll(dir)
	switch {
	case jokerGuest && imageOnly:
		err = guest.BuildJokerImage(tamago, a, dir, out)
	case jokerGuest:
		err = guest.BuildJoker(tamago, a, dir, out)
	case goGuest && imageOnly:
		err = guest.BuildGoImage(tamago, a, dir, out)
	case goGuest:
		err = guest.BuildGo(tamago, a, dir, out)
	case imageOnly:
		err = guest.BuildImage(src, standIn, a, dir, out)
	case standIn != "":
		err = guest.BuildStandIn(standIn, a, dir, out)
	default:
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
	if goGuest {
		what = "the Go editor (editor/), with TamaGo,"
	}
	if jokerGuest {
		what = "Joker's REPL (guest/joker), with TamaGo, the views and the store,"
	}
	fmt.Printf("  guest        %s: %s on %s\n", out, what, a.Name)
	return 0
}
