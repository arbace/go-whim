package p079

// Whim phase 79 (formerly 156) -- the regex size pass's node is a static byte, not (char_u *) -1.  See GOAL.md.
//
// The regex compiler's size pass marks its node pointer with (char_u *) -1,
// an integer made a pointer that is only ever compared.  It becomes the
// address of a static byte, reg_calc_size_node: fourteen uses.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim79", Edit) }

// W79Sentinel is the size pass's sentinel as the input spells it.
const W79Sentinel = "((char_u *)-1)"

// Edit gives the regexp compiler's size pass a real node.
//
// bt_regcomp() compiles a pattern twice: once to count the program's bytes,
// with regcode set to JUST_CALC_SIZE, ((char_u *)-1), and once to emit them.
// In the first pass every node the compiler makes is that sentinel, returned
// and compared but never dereferenced, so one code path serves both passes.
// It is an integer made a pointer, which the Go transpilation replaced with a
// one-byte allocation of its own (internal/gen/FINDINGS.md; internal/ccx's Casts).
// Here too: the sentinel is the address of reg_calc_size_node, a static byte
// nothing reads or writes, and all fourteen uses compare with or assign it.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): the byte declared and its
// fourteen uses written by FRAG in one unit (Together), each use the
// parenthesised sentinel found by its form; history keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("calcsize", e, w)
	v.Together(func(v *graph.Verbs) {
		v.TopAfterC("regcode", "static char_u reg_calc_size_node[1];\n",
			"the size pass's node is a static byte, compared by address and never read")
		v.ReplaceC("(paren (cast (ptr char_u) (- 1)))", "reg_calc_size_node", 14,
			"its fourteen uses compare with or assign the address of that byte, not (char_u *)-1")
	})
	return v.Done()
}
