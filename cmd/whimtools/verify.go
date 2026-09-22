package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/arbace/go-whim/internal/verify"
)

// runVerify is the pipeline's evidence: every phase's check on the tree and the
// state it was written against, and every stage's declared delta.
//
//	whimtools verify                     the whole pipeline, from slim-vim.c
//	whimtools verify --to N              stop after the stage holding N
//	whimtools verify --from N --src B    start at N, from boundary B
//	whimtools verify --root D            keep the work tree and the states in D
//
// It has no cache and no boundaries: what it asserts is what the checks assert,
// every time, and `whimtools build --check` is what asserts the product.
func runVerify(args []string) int {
	o := verify.Options{Src: "slim-vim.c", W: os.Stdout}
	usage := "usage: whimtools verify [--from N --src BOUNDARY] [--to N] [--root D]"
	for i := 0; i < len(args); i++ {
		flag := args[i]
		switch flag {
		case "--from", "--to":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, usage)
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
		case "--src", "--root":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, usage)
				return 2
			}
			if flag == "--src" {
				o.Src = args[i]
			} else {
				o.Root = args[i]
			}
		default:
			fmt.Fprintf(os.Stderr, "whimtools verify: unknown argument %q\n", flag)
			return 2
		}
	}
	if o.From > 0 && o.Src == "slim-vim.c" {
		fmt.Fprintln(os.Stderr, "whimtools verify: --from needs --src, the boundary before that phase")
		return 2
	}
	start := time.Now()
	if err := verify.Run(o); err != nil {
		fmt.Fprintf(os.Stderr, "  verify       %v\n", err)
		return 1
	}
	fmt.Printf("  verify       every check and every declared delta held, %ds\n",
		int(time.Since(start).Seconds()))
	return 0
}
