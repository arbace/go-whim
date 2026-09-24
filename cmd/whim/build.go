package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/arbace/go-whim/internal/build"
)

// runBuild is the pipeline in one process: slim-vim.c in, whim-vim.c out.
//
//	whim build                 write whim-vim.c
//	whim build --check         build and require the committed bytes back
//	whim build --to N          stop after phase N
//	whim build --from N --src B   start at phase N, from boundary B
//	whim build --src F --out F the input and the output
//	whim build --keep D        every boundary as D/qNNN.c, for measuring
//
// --check is the gate on internal/build's plan, and the only one there is: the
// plan was derived from the phase programs, so what holds it to them is that
// the product comes back byte for byte.
func runBuild(args []string) int {
	o := &build.Options{Src: "src/slim-vim.c", To: -1, W: os.Stdout}
	// NOTHING IS WRITTEN WITHOUT --out.  This defaulted to `whim-vim.c`, so a
	// measurement run -- `--canonical`, `--keep-going`, `--to N` -- overwrote
	// the tracked product just by being run from the repository root.  It
	// happened twice in one afternoon, to an agent that had been told to touch
	// nothing but .tmp/.  A command whose job is to answer a question does not
	// get to write the answer over the product.
	out, check := "", false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--check":
			check = true
		case "--keep-going":
			o.KeepGoing = true
		case "--from", "--to":
			flag := args[i]
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "usage: whim build [--check] [--to N] [--src F] [--out F] [--work D] [--keep D]")
				return 2
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				fmt.Fprintf(os.Stderr, "whim: %v\n", err)
				return 2
			}
			if flag == "--from" {
				o.From = n
			} else {
				o.To = n
			}
		case "--src", "--out", "--work", "--keep":
			flag := args[i]
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "usage: whim build [--check] [--to N] [--src F] [--out F] [--work D] [--keep D]")
				return 2
			}
			switch flag {
			case "--src":
				o.Src = args[i]
			case "--out":
				out = args[i]
			case "--work":
				o.Work = args[i]
			case "--keep":
				o.Keep = args[i]
			}
		default:
			fmt.Fprintf(os.Stderr, "whim build: unknown argument %q\n", args[i])
			return 2
		}
	}
	start := time.Now()
	text, err := build.Run(o)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  build        %v\n", err)
		return 1
	}
	secs := int(time.Since(start).Seconds())
	if len(o.Refused) > 0 {
		fmt.Printf("  build        %d phases refused:\n", len(o.Refused))
		for _, r := range o.Refused {
			fmt.Printf("      %s\n", r)
		}
	}
	if check {
		// --check reads the committed product and compares; it writes nothing.
		want, err := os.ReadFile("src/whim-vim.c")
		if err != nil {
			fmt.Fprintf(os.Stderr, "whim: %v\n", err)
			return 1
		}
		if !bytes.Equal(text, want) {
			fmt.Fprintf(os.Stderr,
				"  build        DIFFERS from whim-vim.c: built %d bytes, committed %d -- the plan and the phase programs disagree\n",
				len(text), len(want))
			return 1
		}
		fmt.Printf("  build        whim-vim.c byte for byte, %d lines, %ds\n",
			bytes.Count(text, []byte("\n")), secs)
		return 0
	}
	if out == "" {
		fmt.Printf("  build        %d lines, %ds -- not written anywhere (--out F to keep it)\n",
			bytes.Count(text, []byte("\n")), secs)
		return 0
	}
	if err := os.WriteFile(out, text, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "whim: %v\n", err)
		return 1
	}
	fmt.Printf("  build        %s, %d lines, %ds\n", out, bytes.Count(text, []byte("\n")), secs)
	return 0
}
