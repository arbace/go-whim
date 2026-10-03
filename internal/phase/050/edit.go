package p050

// Whim phase 50 (formerly 120) -- the degenerate unions go.
// See GOAL.md, whose charter is that the core is what a transpiler reads, and
// GOALS.md II.4c, whose rule is that the core's meaning must be on the page.
//
// THIS FILE HAS THIRTEEN `union` KEYWORDS AND SIX OF THEM UNION NOTHING WITH ANYTHING.
// They are not a style that was always there: they are LEFTOVERS of cuts this pipeline
// and whim's already made.  `u_header`'s four link fields were a union of a pointer and
// a swapfile block number, and the arm that named a block went with the swapfile;
// `typval_S.vval` was a union of nine arms -- a string, a list, a dictionary, a funcref,
// a float, a blob, a job, a channel and a number -- and the eval layer took eight of
// them; `estack_T.es_info` was a union of a `ufunc_T *` and a `sctx_T *`, and both went
// with the script stack.  What is left in each case is a variant type with ONE variant,
// which is a value with a longer spelling, and an EMPTY union, which is a value with no
// spelling at all.
//
// WHICH SIX IS COMPUTED AND NOT LISTED.  The edit scans the file for `union`, matches the
// braces, counts the member declarations at depth 1 and takes every union with FEWER THAN
// TWO as degenerate.  It must find both kinds -- at least one degenerate and at least one
// genuine -- so a scanner that stopped matching cannot pass by finding nothing.  Measured
// on the input: 1 member for `uh_next`, `uh_prev`, `uh_alt_next`, `uh_alt_prev` and
// `vval`, 0 for `es_info`, and 2 or 3 for `ae_u`, `lv_u`, `os_oldval`, `os_newval`,
// `rs_u`, `se_u` and `rs_un`, which stay exactly as they are.  Thirteen keywords become
// seven, and the seven that remain are the ones that are doing the job a union is for.
//
// THE EMPTY ONE IS THE ONE WITH A DIALECT ARGUMENT.  `union { } es_info;` is a GNU C
// extension: ISO C requires a struct-declaration-list to be non-empty, and gcc accepts it
// only because it accepts empty structs and unions as an extension -- `-Wpedantic` says
// so, and the check measures that the input draws exactly one such diagnostic and the
// output none.  GOALS.md's core is meant to be readable by something that is not gcc,
// and a construct the C standard forbids is exactly the kind of latent exotic that costs
// a reader later.  It is also the cheapest possible removal: the field has ZERO uses, one
// mention in the whole file, its own declaration -- which is why the sweep, deleting an
// unused member, takes it before this phase runs.  So the input holds five degenerate
// unions, every one of ONE member, and an empty union here refuses: nothing names it,
// so it is the sweep's, not this edit's.
//
// WHAT THE REWRITE IS, and it is the same rule twice.  A single-member union becomes its
// member, keeping the UNION's name:
//
// union {                              u_header_T *uh_next;
// u_header_T *ptr;         ->
// } uh_next;
//
// and every `uh_next.ptr` becomes `uh_next`.  The replacement text is the member's OWN
// declaration with the member's name replaced by the union's, so the type, the pointer
// stars and the internal spacing are the input's and not this program's.
//
// A PARTITION AND NOT A COUNT (CLAUDE.md, *Rename a name across the whole file*).  For
// each of the six names, EVERY mention outside a literal must classify as either its own
// declaration or a `.member` access on it, and a mention that is neither REFUSES.  That is
// what makes the rewrite safe rather than merely mechanical: a `uh_next` assigned or
// compared as a whole, a `sizeof(vval)`, a designated initialiser `.vval = `, or another
// struct with a field of the same name and a different member would all land in the
// leftover class and stop the phase.  The counts are read off the text here and nowhere
// written down, so this stays true of a file the phase has never seen -- which is the
// lesson phase 49b was taught when phase 49a moved its counted anchors.
//
// LITERAL-AWARE AND SINGLE-PASS, for the reason phase 0a measured.  The file has no
// preprocessor and no comments, so a string or character literal is exactly a quote and
// the escaped bytes to its match, and the scan is exact; no literal in this file holds any
// of the six names, which is asserted rather than assumed.  And every span -- six
// declarations and every accessor -- is computed against the ORIGINAL text and applied in
// ONE pass, because a second pass would index spans computed on the first pass's output
// and every offset after the first replacement is shifted.
//
// THE BINARY MUST NOT MOVE, AND THAT IS THE WHOLE OF THIS PHASE'S EVIDENCE.  A union of
// one member has the size and alignment of that member and its offset is the union's; an
// empty union contributes no storage.  So no structure layout changes, no expression
// changes value, and `uh_next.ptr` and `uh_next` name the same object at the same address.
// The check rebuilds both sides with SOURCE_DATE_EPOCH=0 and the boundary's own flags and
// requires THE SAME BYTES -- tier 1 of CLAUDE.md's verification table, which subsumes
// every screen case, every Ex-command row, every command line and every pty scenario at
// once, because the program that would be run is literally the same program.  The control
// that makes that `cmp` mean something is in the check and is a layout change of the same
// shape, in the same struct.
// The flags are read out of the boundary's makefile rather than written here a second
// time: the core's compile line is the boundary's (GOALS.md core rule 8).

//
// ON THE GRAPH (doc/GRAPH.md, step 5; doc/GRAPH-MIGRATION.md, B3d).  The rule
// is crefactor/graph's DegenerateUnions (b3db_unions.go), which replaced
// crefactor/xform's Unions: a union is a `(union ...)` form, an access a use
// of the member by edge, each `x.m.only` the inner selection moved into the
// outer's place, and the member RETYPEd to the one member's type.

import (
	"fmt"
	"io"
	"strconv"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim50", Edit) }

// Edit is the phase on the graph: `--degenerate N --genuine M`, the least
// the scan must find of each.
func Edit(e *graph.Editor, w io.Writer, args []string) error {
	f := map[string]int{"--degenerate": -1, "--genuine": -1}
	for i := 0; i < len(args); i += 2 {
		if _, ok := f[args[i]]; !ok || i+1 >= len(args) {
			return fmt.Errorf("unions: unexpected argument %q (want --degenerate N --genuine M)", args[i])
		}
		n, err := strconv.Atoi(args[i+1])
		if err != nil {
			return fmt.Errorf("unions: %s %q is not a number", args[i], args[i+1])
		}
		f[args[i]] = n
	}
	for k, n := range f {
		if n < 0 {
			return fmt.Errorf("unions: %s N is required", k)
		}
	}
	v := graph.NewVerbs("unions", e, w)
	v.DegenerateUnions(f["--degenerate"], f["--genuine"])
	return v.Done()
}
