package p150

// Whim phase 150 -- the regexp stack is three typed stacks.  See GOAL.md.
//
// regmatch()'s stack held regitem_T records and, below a star's or a
// look-behind's record, its regstar_T or regbehind_T, in one byte array
// (tx/FINDINGS.md, 5 and 8).  They are three stacks of their own types, kept in
// step, and regstack_bytes keeps the byte count 'maxmempattern' is measured by.
//
// THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
// makefile flags, as $state/old beside $state/old.c, for the check.
// An edit that starts a background job waits for it before it exits (tools/phaserun.sh).

import (
	"fmt"
	"io"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim150", Edit) }

// W150Tops are the two accessors for the top of the extra-data stacks,
// exported so the check requires the identical text.
const W150Tops = `    static regstar_T *
regstack_star_top(void)
{
    return &((regstar_T *)regstack_star.ga_data)[regstack_star.ga_len - 1];
}

    static regbehind_T *
regstack_behind_top(void)
{
    return &((regbehind_T *)regstack_behind.ga_data)[regstack_behind.ga_len - 1];
}

`

func w150Grow(ga string) string {
	return "__builtin_expect(((((&" + ga + ")->ga_maxlen - (&" + ga + ")->ga_len < ((int)sizeof("
}

// Whim150 splits the backtracking engine's stack into three typed stacks.
//
// regmatch()'s stack was one byte array holding regitem_T records and, below
// the record of a star or a look-behind state, the regstar_T or regbehind_T it
// carries: pushed first, found again as `((regstar_T *)rp) - 1`, popped with a
// `ga_len -= sizeof(...)`.  Go cannot overlay structs on bytes, and the
// transpilation kept the objects in a side table and the accounting in x86-64
// sizes (tx/FINDINGS.md, 5 and 8).  Now the records, the stars and the
// look-behinds are three stacks of their own types.  The extra data is still
// pushed just before its record and popped just after it, so the three stay
// in step; regstack_bytes adds and subtracts exactly the sizes the byte array
// did, so 'maxmempattern' (E363) is reached at the same moment.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "regstack", W: w}
	s := string(text)
	var err error
	lit := func(old, new, what string, n int) {
		if err != nil {
			return
		}
		if k := strings.Count(s, old); k != n {
			err = p.Die("%s -- occurs %d times, expected %d", what, k, n)
			return
		}
		s = strings.ReplaceAll(s, old, new)
		p.Say(what)
	}
	// The canonical text writes an aggregate initialiser one element per line,
	// with the brace under the `=` and a comma after the last element, so each
	// of these declarations is eight lines and a blank line stands between two
	// of them.
	lit("static garray_T regstack =\n{\n    0,\n    0,\n    0,\n    0,\n    nullptr,\n};\n",
		"static garray_T regstack =\n{\n    0,\n    0,\n    0,\n    0,\n    nullptr,\n};\n\nstatic garray_T regstack_star =\n{\n    0,\n    0,\n    0,\n    0,\n    nullptr,\n};\n\nstatic garray_T regstack_behind =\n{\n    0,\n    0,\n    0,\n    0,\n    nullptr,\n};\n\nstatic int regstack_bytes = 0;\n",
		"the records, the stars and the look-behinds are three stacks, and the bytes they hold a count", 1)
	lit("(long)((unsigned)regstack.ga_len >> 10) >= p_mmp", "(long)((unsigned)regstack_bytes >> 10) >= p_mmp",
		"'maxmempattern' is measured against the count", 3)
	// regstack_push and regstack_pop
	lit("    if ("+w150Grow("regstack")+"regitem_T))) ? ga_grow_inner((&regstack), ((int)sizeof(regitem_T))) : OK) == FAIL), 0))\n    {\n        return nullptr;\n    }\n    rp = (regitem_T *)((char *)regstack.ga_data + regstack.ga_len);\n    rp->rs_state = state;\n    rp->rs_scan = scan;\n    regstack.ga_len += sizeof(regitem_T);\n",
		"    if (ga_grow(&regstack, 1) == FAIL)\n    {\n        return nullptr;\n    }\n\n    rp = &((regitem_T *)regstack.ga_data)[regstack.ga_len];\n    rp->rs_state = state;\n    rp->rs_scan = scan;\n\n    ++regstack.ga_len;\n    regstack_bytes += sizeof(regitem_T);\n",
		"regstack_push() pushes a record on the record stack", 1)
	lit("    rp = (regitem_T *)((char *)regstack.ga_data + regstack.ga_len) - 1;\n    *scan = rp->rs_scan;\n    regstack.ga_len -= sizeof(regitem_T);\n",
		"    rp = &((regitem_T *)regstack.ga_data)[regstack.ga_len - 1];\n    *scan = rp->rs_scan;\n\n    --regstack.ga_len;\n    regstack_bytes -= sizeof(regitem_T);\n",
		"regstack_pop() pops one", 1)
	// the star and look-behind pushes
	for _, k := range []struct{ t, ga, ind string }{{"regstar_T", "regstack_star", "                    "}, {"regbehind_T", "regstack_behind", "            "}} {
		old := k.ind + "else if (" + w150Grow("regstack") + k.t + "))) ? ga_grow_inner((&regstack), ((int)sizeof(" + k.t + "))) : OK) == FAIL), 0))\n"
		lit(old, k.ind+"else if (ga_grow(&"+k.ga+", 1) == FAIL)\n", "a "+k.t+" is pushed on its own stack", 1)
		lit(k.ind+"    regstack.ga_len += sizeof("+k.t+");\n", k.ind+"    ++"+k.ga+".ga_len;\n"+k.ind+"    regstack_bytes += sizeof("+k.t+");\n", "counting the bytes it held", 1)
	}
	lit("*(((regstar_T *)rp) - 1) = rst;", "*regstack_star_top() = rst;", "the star's data is the top of the star stack", 1)
	lit("regstar_T *rst = ((regstar_T *)rp) - 1;", "regstar_T           *rst = regstack_star_top();", "and is found there", 1)
	lit("(((regbehind_T *)rp) - 1)->", "regstack_behind_top()->", "a look-behind's data is the top of its stack", 6)
	lit("save_subexpr(((regbehind_T *)rp) - 1);", "save_subexpr(regstack_behind_top());", "saved into", 1)
	lit("restore_subexpr(((regbehind_T *)rp) - 1);", "restore_subexpr(regstack_behind_top());", "and restored from", 3)
	for _, k := range []struct{ t, ga string }{{"regbehind_T", "regstack_behind"}, {"regstar_T", "regstack_star"}} {
		for _, ind := range []string{"                ", "                    "} {
			old := "\n" + ind + "regstack.ga_len -= sizeof(" + k.t + ");\n"
			n := strings.Count(s, old)
			if n > 0 {
				lit(old, "\n"+ind+"--"+k.ga+".ga_len;\n"+ind+"regstack_bytes -= sizeof("+k.t+");\n", fmt.Sprintf("popping a %s pops its stack (%d at this depth)", k.t, n), n)
			}
		}
	}
	lit("        rp = (regitem_T *)((char *)regstack.ga_data + regstack.ga_len) - 1;\n", "        rp = &((regitem_T *)regstack.ga_data)[regstack.ga_len - 1];\n", "the loop reads the top record", 1)
	lit("rp == (regitem_T *)((char *)regstack.ga_data + regstack.ga_len) - 1)", "rp == &((regitem_T *)regstack.ga_data)[regstack.ga_len - 1])", "and asks whether it is still the top", 1)
	lit("    regstack.ga_len = 0;\n    backpos.ga_len = 0;\n", "  regstack.ga_len = 0;\n  regstack_star.ga_len = 0;\n  regstack_behind.ga_len = 0;\n  regstack_bytes = 0;\n  backpos.ga_len = 0;\n", "a match starts with the three stacks and the count empty", 1)
	lit("        ga_init2(&regstack, 1, REGSTACK_INITIAL);\n        (void)ga_grow(&regstack, REGSTACK_INITIAL);\n        regstack.ga_growsize = REGSTACK_INITIAL * 8;\n",
		"        ga_init2(&regstack, sizeof(regitem_T), REGSTACK_INITIAL / sizeof(regitem_T));\n        (void)ga_grow(&regstack, REGSTACK_INITIAL / sizeof(regitem_T));\n        regstack.ga_growsize = REGSTACK_INITIAL * 8 / sizeof(regitem_T);\n        ga_init2(&regstack_star, sizeof(regstar_T), 16);\n        ga_init2(&regstack_behind, sizeof(regbehind_T), 4);\n",
		"the record stack starts at the same bytes, the other two small", 1)
	lit("    if (regstack.ga_maxlen > REGSTACK_INITIAL)\n", "    if (regstack.ga_maxlen > (int)(REGSTACK_INITIAL / sizeof(regitem_T)))\n", "and is let go when it grew past them", 1)
	if err != nil {
		return nil, err
	}
	// the accessors, before regstack_push()
	head := "    static regitem_T *\nregstack_push("
	if strings.Count(s, head) != 1 {
		return nil, p.Die("regstack_push() is not where this phase expects it")
	}
	s = strings.Replace(s, head, W150Tops+head, 1)
	if strings.Contains(s, "((regstar_T *)rp) - 1") || strings.Contains(s, "((regbehind_T *)rp) - 1") || strings.Contains(s, "(char *)regstack.ga_data") {
		return nil, p.Die("the stack is still read as bytes somewhere")
	}
	return []byte(s), nil
}
