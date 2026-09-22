package edit

import (
	"bytes"
	"fmt"
	"io"
)

func init() { register("whim135", Whim135) }

const w135Two = `typedef struct regprog
{
    regengine_T         *engine;
    unsigned            regflags;
    unsigned            re_engine;
    unsigned            re_flags;
    int                 re_in_use;
} regprog_T;

typedef struct
{
    regengine_T         *engine;
    unsigned            regflags;
    unsigned            re_engine;
    unsigned            re_flags;
    int                 re_in_use;

    int                 regstart;
    char_u              reganch;
    char_u              *regmust;
    int                 regmlen;
    char_u              program[1];
} bt_regprog_T;
`

// W135One is the one program type this phase leaves, exported for the check.
const W135One = `typedef struct regprog
{
    regengine_T         *engine;
    unsigned            regflags;
    unsigned            re_engine;
    unsigned            re_flags;
    int                 re_in_use;

    int                 regstart;
    char_u              reganch;
    char_u              *regmust;
    int                 regmlen;
    char_u              program[1];
} regprog_T;
`

// Whim135 makes regprog_T and bt_regprog_T one type.
//
// vim had two regexp engines, and regprog_T was the header both programs
// began with: the backtracking engine's bt_regprog_T repeated its five fields
// and added its own, and the code cast between the two.  Whim kept one engine,
// so every regprog_T is a bt_regprog_T and the casts are casts to itself.  The
// Go transpilation could not cast a struct to the larger one it heads and kept
// a registry to find one from the other (tx/FINDINGS.md, 4).  regprog_T takes
// the backtracking fields, the five casts go, and bt_regprog_T is not a name
// any more.  The layout of every field is what it was, so the code is too.
func Whim135(text []byte, w io.Writer) ([]byte, error) {
	p := ph{tag: "regprog", w: w}
	var err error
	steps := []struct{ old, new, what string }{
		{w135Two, W135One, "regprog_T is the backtracking program: its five fields, then the engine's own"},
		{"(((bt_regprog_T *)prog)->program)", "((prog)->program)", "prog_magic_wrong() reads the program without a cast"},
		{"    return (regprog_T *)r;\n", "    return r;\n", "bt_regcomp() returns its program without one"},
		{"prog = (bt_regprog_T *)rex.reg_mmatch->regprog;", "prog = rex.reg_mmatch->regprog;", "bt_regexec_both() takes the multi-line match's program as it is"},
		{"prog = (bt_regprog_T *)rex.reg_match->regprog;", "prog = rex.reg_match->regprog;", "and the single-line match's"},
	}
	for _, s := range steps {
		if text, err = p.literal(text, s.old, s.new, s.what, 1); err != nil {
			return nil, err
		}
	}
	n := bytes.Count(text, []byte("bt_regprog_T"))
	if n != 4 {
		return nil, p.die("bt_regprog_T has %d mentions left to rename, and this phase was written against 4", n)
	}
	text = bytes.ReplaceAll(text, []byte("bt_regprog_T"), []byte("regprog_T"))
	p.say(fmt.Sprintf("the %d declarations that still said bt_regprog_T say regprog_T", n))
	return text, nil
}
