// Package xform is the generic C transformations the pipeline's phases were
// written as: each one a Step built from its knobs by a constructor, on the
// canonical text of one translation unit.  Every phase runs on the graph
// now (doc/GRAPH-MIGRATION.md; R3 moved the last two, the seed's spelling
// and phase 88's headers), and what is left here is FallOut, the text's
// fall-out closure, which crefactor/graph's FoldX tests hold FoldX to, and
// Flags, the steps' argument parser.
//
// A transformation knows C and nothing of the code base it is run on: what a
// code base calls its truth constants, its no-op functions, its allocators or
// where its core ends is handed in, and so is every count a phase refuses
// under, as an argument (`--at-least N`), so that a foreign code base passes
// nothing and gets no floor.  No identifier of any one code base appears in a
// string literal here, and nothing here imports the code base's own packages.
//
// The tree LOCATES (cc.Parse); the text is what is rewritten.
package xform

import (
	"fmt"
	"io"
	"strconv"

	"github.com/arbace/go-whim/crefactor/cc"
)

// A Step is one transformation: the tree in, the tree out, its report on w.
// It is internal/steps' Step, and the shape internal/build runs.
type Step func(text []byte, args []string, w io.Writer) ([]byte, error)

// file is the name the text is parsed under; a node is the text's when its
// position names it.
const file = "tu.c"

// parse parses text as one translation unit.
func parse(text []byte) (*cc.AST, error) {
	cfg, err := cc.NewConfig("linux", "amd64")
	if err != nil {
		return nil, err
	}
	return cc.Parse(cfg, []cc.Source{
		{Name: "<predefined>", Value: cfg.Predefined},
		{Name: "<builtin>", Value: cc.Builtin},
		{Name: file, Value: text},
	})
}

// flags reads a step's arguments: each `--name N` a count, and nothing else.
// A name not in want refuses, and so does a count that is not a number.
func flags(tag string, args []string, want ...string) (map[string]int, error) {
	out := map[string]int{}
	for i := 0; i < len(args); i++ {
		ok := false
		for _, n := range want {
			ok = ok || args[i] == n
		}
		if !ok || i+1 == len(args) {
			return nil, fmt.Errorf("%s: unexpected argument %q (want %v, each with a count)", tag, args[i], want)
		}
		n, err := strconv.Atoi(args[i+1])
		if err != nil || n < 0 {
			return nil, fmt.Errorf("%s: %s wants a count, not %q", tag, args[i], args[i+1])
		}
		out[args[i]] = n
		i++
	}
	return out, nil
}

// Flags is flags for a step written elsewhere -- on the graph -- that takes
// the same counted arguments as the text steps here.
func Flags(tag string, args []string, want ...string) (map[string]int, error) {
	return flags(tag, args, want...)
}
