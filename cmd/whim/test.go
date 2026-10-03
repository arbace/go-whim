package main

import (
	"fmt"
	"os"
	"time"

	"github.com/arbace/go-whim/internal/suite"
)

// testUsage is runTest's usage line.
const testUsage = "usage: whim test [--wide] [--java] [--clojure] [--clojure-editor editor.clj] [--haskell] [--haskell-bin PROGRAM] [--rust] [--scheme] [--scheme-debug] [--limit DURATION] [--ref REV] [FILE]"

// runTest is the minimal behaviour check (internal/suite): every session in
// internal/suite/cases.md on the editor built from src/whim-vim.c at REV
// (default HEAD) and from FILE (default src/whim-vim.c), which must behave
// the same, with a control the corpus must see.  --java adds the Java editor
// (braaam/, doc/JAVA.md), built from FILE, to the quick suite or the wide
// one: the same cases, required to answer as the C does, with a control of
// its own.  --clojure adds the Clojure editor (vijure/, doc/CLOJURE.md)
// the same way; --clojure-editor F adds it with the namespace in F rather
// than one generated from FILE.  --haskell adds the Haskell editor
// (caprice/, doc/HASKELL.md) the same way; --haskell-bin P adds it as the
// program P, built already (doc/GHC-LISP.md's), its control P with its one
// " INSERT" changed.  --rust adds the Rust editor (whimsy/, doc/RUST.md),
// and --scheme the Scheme editor (whimsical/, doc/SCHEME.md); --scheme-debug
// adds it as its debugging build (optimize-level 2, safe, the inspector's
// information kept).  --limit D is how long one run of a case may take on
// the editors these add and their controls (default 10s; the C and the Go
// editor keep 10s), for a build slow on purpose.
//
//	whim test [--wide] [--java] [--clojure] [--clojure-editor editor.clj] [--haskell] [--haskell-bin PROGRAM] [--rust] [--scheme] [--scheme-debug] [--limit DURATION] [--ref REV] [FILE]
func runTest(args []string) int {
	o, err := parseTest(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, testUsage)
		return 2
	}
	if o.wide {
		// the wide suite, on demand: 200-odd cases the quick one does not reach
		if err := suite.Wide(os.Stdout, o.rev, o.file, o.jvm); err != nil {
			fmt.Fprintf(os.Stderr, "  wide         %v\n", err)
			return 1
		}
		return 0
	}
	if err := suite.Check(os.Stdout, o.rev, o.file, o.jvm); err != nil {
		fmt.Fprintf(os.Stderr, "  test         %v\n", err)
		return 1
	}
	return 0
}

// testOpts is what runTest's arguments ask for.
type testOpts struct {
	rev, file string
	wide      bool
	jvm       suite.JVM
}

// parseTest reads runTest's arguments.
func parseTest(args []string) (testOpts, error) {
	o := testOpts{rev: "HEAD", file: "src/whim-vim.c"}
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--ref" && i+1 < len(args):
			i++
			o.rev = args[i]
		case args[i] == "--wide":
			o.wide = true
		case args[i] == "--java":
			o.jvm.Java = javaGen
		case args[i] == "--clojure":
			o.jvm.Clojure = cljGen
		case args[i] == "--haskell":
			o.jvm.Haskell = hsGen
		case args[i] == "--haskell-bin" && i+1 < len(args):
			i++
			o.jvm.HaskellBin = args[i]
		case args[i] == "--rust":
			o.jvm.Rust = rsGen
		case args[i] == "--scheme":
			o.jvm.Scheme = scmGen
		case args[i] == "--scheme-debug":
			o.jvm.Scheme, o.jvm.SchemeDebug = scmGen, true
		case args[i] == "--clojure-editor" && i+1 < len(args):
			i++
			o.jvm.Clojure = copyGen(args[i])
		case args[i] == "--limit" && i+1 < len(args):
			i++
			d, err := time.ParseDuration(args[i])
			if err != nil || d <= 0 {
				return o, fmt.Errorf("whim test: --limit %s is not a positive duration (10s, 2m)", args[i])
			}
			o.jvm.Limit = d
		case len(args[i]) > 0 && args[i][0] != '-':
			o.file = args[i]
		default:
			return o, fmt.Errorf("whim test: %s is not an argument it takes", args[i])
		}
	}
	return o, nil
}
