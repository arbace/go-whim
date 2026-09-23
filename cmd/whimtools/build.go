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
//	whimtools build                 write whim-vim.c
//	whimtools build --check         build and require the committed bytes back
//	whimtools build --to N          stop after phase N
//	whimtools build --from N --src B   start at phase N, from boundary B
//	whimtools build --src F --out F the input and the output
//
// --check is the gate on internal/build's plan, and the only one there is: the
// plan was derived from the phase programs, so what holds it to them is that
// the product comes back byte for byte.
func runBuild(args []string) int {
	o := &build.Options{Src: "slim-vim.c", W: os.Stdout}
	out, check := "whim-vim.c", false
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
				fmt.Fprintln(os.Stderr, "usage: whimtools build [--check] [--to N] [--src F] [--out F] [--work D]")
				return 2
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				fmt.Fprintf(os.Stderr, "whimtools: %v\n", err)
				return 2
			}
			if flag == "--from" {
				o.From = n
			} else {
				o.To = n
			}
		case "--src", "--out", "--work":
			flag := args[i]
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "usage: whimtools build [--check] [--to N] [--src F] [--out F] [--work D]")
				return 2
			}
			switch flag {
			case "--src":
				o.Src = args[i]
			case "--out":
				out = args[i]
			case "--work":
				o.Work = args[i]
			}
		default:
			fmt.Fprintf(os.Stderr, "whimtools build: unknown argument %q\n", args[i])
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
		want, err := os.ReadFile(out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "whimtools: %v\n", err)
			return 1
		}
		if !bytes.Equal(text, want) {
			fmt.Fprintf(os.Stderr,
				"  build        DIFFERS from %s: built %d bytes, committed %d -- the plan and the phase programs disagree\n",
				out, len(text), len(want))
			return 1
		}
		fmt.Printf("  build        %s byte for byte, %d lines, %ds\n",
			out, bytes.Count(text, []byte("\n")), secs)
		return 0
	}
	if err := os.WriteFile(out, text, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "whimtools: %v\n", err)
		return 1
	}
	fmt.Printf("  build        %s, %d lines, %ds\n", out, bytes.Count(text, []byte("\n")), secs)
	return 0
}
