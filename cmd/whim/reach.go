package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/arbace/go-whim/crefactor/ccx"
	"github.com/arbace/go-whim/crefactor/reach"
	"github.com/arbace/go-whim/internal/whim"
)

// runReach reports what in a translation unit nothing reaches, as a partition
// of every entity (internal/reach), and runs gcc as its control.  It changes
// nothing: the text is read, and the copies the control compiles go under
// TMPDIR.  The status is 0 only when no class is left over -- nothing is
// unreachable and gcc agrees -- so on a text with dead code it refuses, by
// name; that is the report.
func runReach(args []string) int {
	control := true
	var path string
	for _, a := range args {
		switch {
		case a == "--no-control":
			control = false
		case strings.HasPrefix(a, "-") || path != "":
			fmt.Fprintln(os.Stderr, "usage: whim reach <file.c> [--no-control]")
			return 2
		default:
			path = a
		}
	}
	if path == "" {
		fmt.Fprintln(os.Stderr, "usage: whim reach <file.c> [--no-control]")
		return 2
	}
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim reach: %v\n", err)
		return 1
	}
	ast, err := ccx.Parse(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim reach: %s does not parse, and a closure has nothing to say about it: %v\n", path, err)
		return 1
	}
	c := reach.Analyze(ast, path, src, whim.Reach)
	kinds := map[string][2]int{}
	for _, e := range c.Entities {
		k := kinds[e.Kind]
		k[0]++
		if !e.Reachable() {
			k[1]++
		}
		kinds[e.Kind] = k
	}
	fmt.Printf("%s: %d entities", path, len(c.Entities))
	for _, k := range []string{"F", "O", "P", "T", "S", "E", "M", "N"} {
		fmt.Printf(", %s %d/%d", k, kinds[k][1], kinds[k][0])
	}
	fmt.Printf(" unreachable\n")
	cut := "none"
	if c.Cut >= 0 {
		cut = fmt.Sprintf("byte %d", c.Cut)
	}
	fmt.Printf("roots (a root can be in more than one class; the core/host cut at %s):\n", cut)
	for _, cl := range []string{reach.ClassExternal, reach.ClassAssert, reach.ClassCut} {
		fmt.Printf("  %6d  %s: %s\n", len(c.Roots[cl]), cl, strings.Join(c.Roots[cl], " "))
	}
	fmt.Printf("  %6d  %s: %s\n", len(c.Pun.Held), reach.ClassCast, strings.Join(c.Pun.Held, " "))
	fmt.Printf("pointer casts between struct types (every member of both sides held): %d pairs: %s\n",
		len(c.Pun.Pairs), strings.Join(c.Pun.Pairs, "; "))
	fmt.Printf("through void * (NOT held -- the reader of a void * is no cast when it is implicit):\n")
	for _, v := range []struct {
		n  string
		ts []string
	}{{"cast from void *, not an allocator", c.Pun.FromVoid}, {"cast from an allocator", c.Pun.FromAllocator},
		{"cast to void *", c.Pun.ToVoid}, {"both to and from void *", c.Pun.RoundTrip}} {
		fmt.Printf("  %6d struct types %s: %s\n", len(v.ts), v.n, strings.Join(v.ts, " "))
	}
	ok := c.Partition().Print(os.Stdout)
	if !control {
		return status(ok)
	}
	dir, err := os.MkdirTemp("", "reach-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim reach: %v\n", err)
		return 1
	}
	defer os.RemoveAll(dir)
	agree, unused, err := reach.Agreement(c, ast, path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim reach: %v\n", err)
		return 1
	}
	ok = agree.Print(os.Stdout) && ok
	planted, err := reach.Planted(c, src, unused, dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim reach: %v\n", err)
		return 1
	}
	ok = planted.Print(os.Stdout) && ok
	pos, err := reach.Positional(c, src, dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim reach: %v\n", err)
		return 1
	}
	ok = pos.Print(os.Stdout) && ok
	return status(ok)
}

func status(ok bool) int {
	if ok {
		return 0
	}
	return 1
}
