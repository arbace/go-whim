package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/arbace/go-whim/internal/build"
	"github.com/arbace/go-whim/internal/ccx"
	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/reach"
)

// runMeasure counts every boundary a `whimtools build --keep D` left in D, one
// row per qNNN.c: its lines, and the front end's count of each kind of thing
// the sweep deletes (internal/reach's entities, so "a function" means one
// definition at file scope, its prototypes merged in, on every row alike).
//
// It is how a GOAL.md's Measured table is re-measured rather than adjusted: the
// rows come from the text a build really produced.  A boundary the front end
// refuses is a row that says so -- inside a stage the text before its sweep
// need not parse -- and its counts are left empty, never estimated.
func runMeasure(args []string) int {
	compile := true
	if len(args) == 2 && args[1] == "--no-build" {
		compile, args = false, args[:1]
	}
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: whimtools measure <dir of qNNN.c> [--no-build]")
		return 2
	}
	files, _ := filepath.Glob(filepath.Join(args[0], "q[0-9][0-9][0-9].c"))
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "whimtools measure: no qNNN.c in %s -- run `whimtools build --keep %s` first\n", args[0], args[0])
		return 1
	}
	sort.Strings(files)
	rows := make([]string, len(files))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i, f := range files {
		wg.Add(1)
		go func(i int, f string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			rows[i] = measureOne(f, compile)
		}(i, f)
	}
	wg.Wait()
	fmt.Println("phase  lines    F      O     P    T     S    E    M      N      binary    nm-u  parses")
	for _, r := range rows {
		fmt.Println(r)
	}
	return 0
}

func measureOne(path string, compile bool) string {
	src, err := os.ReadFile(path)
	phase := filepath.Base(path)[1:4]
	if err != nil {
		return fmt.Sprintf("%s    %v", phase, err)
	}
	lines := bytes.Count(src, []byte("\n"))
	bin, nmu := "-", "-"
	if compile {
		bin, nmu = binaryOf(path, phase)
	}
	ast, err := ccx.Parse(path)
	if err != nil {
		return fmt.Sprintf("%s    %-8d %-58s %-9s %-5s no -- %s", phase, lines, "", bin, nmu, firstLine(err.Error()))
	}
	n := map[string]int{}
	for _, e := range reach.Analyze(ast, path, src).Entities {
		n[e.Kind]++
	}
	return fmt.Sprintf("%s    %-8d %-6d %-5d %-4d %-5d %-4d %-4d %-6d %-6d %-9s %-5s yes",
		phase, lines, n["F"], n["O"], n["P"], n["T"], n["S"], n["E"], n["M"], n["N"], bin, nmu)
}

// binaryOf builds the boundary with the compile line its phase leaves
// (build.MakefileFor -- whim.mk to 82, core.mk from 83, the stack protector
// gone from 84) and SOURCE_DATE_EPOCH=0, and returns the binary's size and
// the count of undefined symbols the gate counts (check.Symbols), or why not.
func binaryOf(path, phase string) (string, string) {
	n, _ := strconv.Atoi(phase)
	dir, err := os.MkdirTemp("", "measure-"+phase+".")
	if err != nil {
		return "err", "err"
	}
	defer os.RemoveAll(dir)
	src, _ := os.ReadFile(path)
	if err := os.WriteFile(filepath.Join(dir, "whim-vim.c"), src, 0o644); err != nil {
		return "err", "err"
	}
	if err := build.MakefileFor(n, filepath.Join(dir, "Makefile")); err != nil {
		return "no-mk", "-"
	}
	bin, nmu := "fails", "-"
	cmd := exec.Command("make", "-s", "-C", dir)
	cmd.Env = append(os.Environ(), "SOURCE_DATE_EPOCH=0")
	if cmd.Run() == nil {
		if fi, err := os.Stat(filepath.Join(dir, "whim-vim")); err == nil {
			bin = strconv.FormatInt(fi.Size(), 10)
		}
	}
	if check.Symbols(filepath.Join(dir, "whim-vim.c"), filepath.Join(dir, "sym")) == nil {
		u, _ := os.ReadFile(filepath.Join(dir, "sym", "undefined"))
		nmu = strconv.Itoa(len(strings.Fields(string(u))))
	}
	return bin, nmu
}

func firstLine(s string) string {
	if i := bytes.IndexByte([]byte(s), '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
