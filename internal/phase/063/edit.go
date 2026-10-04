package p063

// Whim phase 63 (formerly 135) -- one regexp program type.  See GOAL.md.
//
// With one engine every regprog_T is a bt_regprog_T, and the casts between the
// two are casts to itself (internal/gen/FINDINGS.md, 4).  regprog_T takes the
// backtracking fields, the casts go, and bt_regprog_T is not a name any more.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim63", Edit) }

const w63Two = `typedef struct regprog
{
    regengine_T *engine;
    unsigned regflags;
    unsigned re_engine;
    unsigned re_flags;
    int re_in_use;
} regprog_T;

typedef struct
{
    regengine_T *engine;
    unsigned regflags;
    unsigned re_engine;
    unsigned re_flags;
    int re_in_use;
    int regstart;
    char_u reganch;
    char_u *regmust;
    int regmlen;
    char_u program[1];
} bt_regprog_T;
`

// W63One is the one program type this phase leaves, exported for the check.
const W63One = `typedef struct regprog
{
    regengine_T *engine;
    unsigned regflags;
    unsigned re_engine;
    unsigned re_flags;
    int re_in_use;
    int regstart;
    char_u reganch;
    char_u *regmust;
    int regmlen;
    char_u program[1];
} regprog_T;
`

// Edit makes regprog_T and bt_regprog_T one type.
//
// vim had two regexp engines, and regprog_T was the header both programs
// began with: the backtracking engine's bt_regprog_T repeated its five fields
// and added its own, and the code cast between the two.  Whim kept one engine,
// so every regprog_T is a bt_regprog_T and the casts are casts to itself.  The
// Go transpilation could not cast a struct to the larger one it heads and kept
// a registry to find one from the other (internal/gen/FINDINGS.md, 4).  regprog_T takes
// the backtracking fields, the five casts go, and bt_regprog_T is not a name
// any more.  The layout of every field is what it was, so the code is too.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): the casts rewritten by form;
// the three declarations naming bt_regprog_T pointed at regprog_T
// (RetargetAs); then the two typedefs are one, written by FRAG in their
// place, with the offsetof in bt_regcomp() that named bt_regprog_T in a
// macro's text, in one unit -- which resolves every member selected through
// a regprog_T again, on the one struct; history keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("regprog", e, w)
	v.Rewrite("(cast (ptr bt_regprog_T) prog)", "(paren prog)", 1,
		"prog_magic_wrong() reads the program without a cast")
	v.Rewrite("(return (cast (ptr regprog_T) r))", "(return r)", 1, "bt_regcomp() returns its program without one")
	v.Rewrite("(cast (ptr bt_regprog_T) (-> (. rex reg_mmatch) regprog))", "(-> (. rex reg_mmatch) regprog)", 1,
		"bt_regexec_both() takes the multi-line match's program as it is")
	v.Rewrite("(cast (ptr bt_regprog_T) (-> (. rex reg_match) regprog))", "(-> (. rex reg_match) regprog)", 1,
		"and the single-line match's")
	one := v.One("(typedef regprog_T _)", "regprog_T")
	two := v.One("(typedef bt_regprog_T _)", "bt_regprog_T")
	off := v.One(`(macro "__builtin_offsetof(bt_regprog_T, program)")`, "bt_regcomp()'s offsetof")
	if v.Failed() {
		return v.Done()
	}
	if c, err := graph.FormsC([]*graph.Node{one, two}); err != nil || strings.Join(strings.Fields(string(c)), " ") != strings.Join(strings.Fields(w63Two), " ") {
		v.Die("the two program types are not the ones this phase was written against")
		return v.Done()
	}
	// the declarations that say bt_regprog_T say regprog_T
	n := 0
	for _, u := range e.Uses(two) {
		if u == off {
			continue
		}
		for i, r := range u.Refs {
			if r == two {
				if err := e.RetargetAs(u, i, one); err != nil {
					v.Die("the declarations that said bt_regprog_T -- %v", err)
					return v.Done()
				}
				n++
			}
		}
	}
	v.Expect(n == 3, "%d declarations said bt_regprog_T; this phase was written against 3", n)
	if v.Failed() {
		return v.Done()
	}
	if _, err := e.SpliceC(graph.Frag{At: e.SpotRun(one, two), Src: W63One},
		graph.Frag{At: e.SpotOf(off), Src: "__builtin_offsetof(regprog_T, program)"}); err != nil {
		v.Die("regprog_T is the backtracking program -- %v", err)
		return v.Done()
	}
	v.Say("regprog_T is the backtracking program: its five fields, then the engine's own")
	v.Say("the 4 declarations that still said bt_regprog_T say regprog_T")
	return v.Done()
}
