package p075

// Whim phase 75 (formerly 150) -- the regexp stack is three typed stacks.  See GOAL.md.
//
// regmatch()'s stack held regitem_T records and, below a star's or a
// look-behind's record, its regstar_T or regbehind_T, in one byte array
// (internal/gen/FINDINGS.md, 5 and 8).  They are three stacks of their own types, kept in
// step, and regstack_bytes keeps the byte count 'maxmempattern' is measured by.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim75", Edit) }

// W75Tops are the two accessors for the top of the extra-data stacks,
// exported so the check requires the identical text.
const W75Tops = `    static regstar_T *
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

// w75Grow is ga_grow()'s expansion growing ga by one T, as the printer writes
// the condition alone.
func w75Grow(ga, t string) string {
	return "__builtin_expect(((((&" + ga + ")->ga_maxlen - (&" + ga + ")->ga_len < ((int)sizeof(" + t + "))) ? ga_grow_inner((&" + ga + "), ((int)sizeof(" + t + "))) : OK) == FAIL), 0)"
}

// Edit splits the backtracking engine's stack into three typed stacks.
//
// regmatch()'s stack was one byte array holding regitem_T records and, below
// the record of a star or a look-behind state, the regstar_T or regbehind_T it
// carries: pushed first, found again as `((regstar_T *)rp) - 1`, popped with a
// `ga_len -= sizeof(...)`.  Go cannot overlay structs on bytes, and the
// transpilation kept the objects in a side table and the accounting in x86-64
// sizes (internal/gen/FINDINGS.md, 5 and 8).  Now the records, the stars and the
// look-behinds are three stacks of their own types.  The extra data is still
// pushed just before its record and popped just after it, so the three stay
// in step; regstack_bytes adds and subtracts exactly the sizes the byte array
// did, so 'maxmempattern' (E363) is reached at the same moment.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): every literal is a FRAG act of
// one unit (Together) -- runs of whole items by their C (LiteralC), the
// expressions by their form or by their C (LiteralExprC), the accessors
// before regstack_push()'s definition (FragAt) -- and the assertions are
// counts of forms.  History keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("regstack", e, w)
	v.Together(func(v *graph.Verbs) {
		v.TopAfterC("regstack", "static garray_T regstack_star = {0, 0, 0, 0, nullptr};\nstatic garray_T regstack_behind = {0, 0, 0, 0, nullptr};\nstatic int regstack_bytes = 0;\n",
			"the records, the stars and the look-behinds are three stacks, and the bytes they hold a count")
		var lens []*graph.Node
		for _, c := range v.Find("(cast unsigned (. regstack ga_len))") {
			lens = append(lens, c.Kids[2])
		}
		v.ReplaceEachC(lens, "regstack_bytes", 3, "'maxmempattern' is measured against the count")
		v.InFunction("regstack_push", func(v *graph.Verbs) {
			v.LiteralC("    if ("+w75Grow("regstack", "regitem_T")+")\n    {\n        return nullptr;\n    }\n    rp = (regitem_T *)((char *)regstack.ga_data + regstack.ga_len);\n    rp->rs_state = state;\n    rp->rs_scan = scan;\n    regstack.ga_len += sizeof(regitem_T);\n",
				"    if (ga_grow(&regstack, 1) == FAIL)\n    {\n        return nullptr;\n    }\n\n    rp = &((regitem_T *)regstack.ga_data)[regstack.ga_len];\n    rp->rs_state = state;\n    rp->rs_scan = scan;\n\n    ++regstack.ga_len;\n    regstack_bytes += sizeof(regitem_T);\n",
				1, "regstack_push() pushes a record on the record stack")
		})
		v.InFunction("regstack_pop", func(v *graph.Verbs) {
			v.LiteralC("    rp = (regitem_T *)((char *)regstack.ga_data + regstack.ga_len) - 1;\n    *scan = rp->rs_scan;\n    regstack.ga_len -= sizeof(regitem_T);\n",
				"    rp = &((regitem_T *)regstack.ga_data)[regstack.ga_len - 1];\n    *scan = rp->rs_scan;\n\n    --regstack.ga_len;\n    regstack_bytes -= sizeof(regitem_T);\n",
				1, "regstack_pop() pops one")
		})
		v.InFunction("regmatch", func(v *graph.Verbs) {
			// the star and look-behind pushes
			for _, k := range []struct{ t, ga string }{{"regstar_T", "regstack_star"}, {"regbehind_T", "regstack_behind"}} {
				v.LiteralExprC("(call __builtin_expect _ _)", w75Grow("regstack", k.t), "ga_grow(&"+k.ga+", 1) == FAIL", 1, "a "+k.t+" is pushed on its own stack")
				v.LiteralC("regstack.ga_len += sizeof("+k.t+");", "++"+k.ga+".ga_len;\nregstack_bytes += sizeof("+k.t+");\n", 1, "counting the bytes it held")
			}
			v.LiteralC("*(((regstar_T *)rp) - 1) = rst;", "*regstack_star_top() = rst;", 1, "the star's data is the top of the star stack")
			v.LiteralC("regstar_T *rst = ((regstar_T *)rp) - 1;", "regstar_T *rst = regstack_star_top();", 1, "and is found there")
			v.ReplaceC("(- (paren (cast (ptr regbehind_T) rp)) 1)", "regstack_behind_top()", 10,
				"a look-behind's data is the top of its stack: read (6), saved into (1) and restored from (3)")
			v.LiteralC("regstack.ga_len -= sizeof(regbehind_T);", "--regstack_behind.ga_len;\nregstack_bytes -= sizeof(regbehind_T);\n", 3,
				"popping a regbehind_T pops its stack (3)")
			v.LiteralC("regstack.ga_len -= sizeof(regstar_T);", "--regstack_star.ga_len;\nregstack_bytes -= sizeof(regstar_T);\n", 2,
				"popping a regstar_T pops its stack (2)")
			v.LiteralC("rp = (regitem_T *)((char *)regstack.ga_data + regstack.ga_len) - 1;", "rp = &((regitem_T *)regstack.ga_data)[regstack.ga_len - 1];", 1, "the loop reads the top record")
			v.LiteralExprC("(== rp _)", "rp == (regitem_T *)((char *)regstack.ga_data + regstack.ga_len) - 1", "rp == &((regitem_T *)regstack.ga_data)[regstack.ga_len - 1]", 1, "and asks whether it is still the top")
		})
		v.LiteralC("    regstack.ga_len = 0;\n    backpos.ga_len = 0;\n", "  regstack.ga_len = 0;\n  regstack_star.ga_len = 0;\n  regstack_behind.ga_len = 0;\n  regstack_bytes = 0;\n  backpos.ga_len = 0;\n", 1, "a match starts with the three stacks and the count empty")
		v.LiteralC("        ga_init2(&regstack, 1, REGSTACK_INITIAL);\n        (void)ga_grow(&regstack, REGSTACK_INITIAL);\n        regstack.ga_growsize = REGSTACK_INITIAL * 8;\n",
			"        ga_init2(&regstack, sizeof(regitem_T), REGSTACK_INITIAL / sizeof(regitem_T));\n        (void)ga_grow(&regstack, REGSTACK_INITIAL / sizeof(regitem_T));\n        regstack.ga_growsize = REGSTACK_INITIAL * 8 / sizeof(regitem_T);\n        ga_init2(&regstack_star, sizeof(regstar_T), 16);\n        ga_init2(&regstack_behind, sizeof(regbehind_T), 4);\n",
			1, "the record stack starts at the same bytes, the other two small")
		v.ReplaceC("(> (. regstack ga_maxlen) REGSTACK_INITIAL)", "regstack.ga_maxlen > (int)(REGSTACK_INITIAL / sizeof(regitem_T))", 1, "and is let go when it grew past them")
		if f := e.Defn("regstack_push"); f != nil {
			v.FragAt(e.SpotBefore(f), W75Tops, "the accessors for the top of the two stacks go before regstack_push()")
		} else {
			v.Die("regstack_push is not defined")
		}
	})
	// EVERY TYPED POP AND PUSH IS GONE, and that is asserted as well as counted.
	for _, t := range []string{"regbehind_T", "regstar_T"} {
		k := v.Count("(-= (. regstack ga_len) (sizeof-type " + t + "))")
		v.Expect(k == 0, "%d pops of a %s still take it off the record stack", k, t)
		k = v.Count("(+= (. regstack ga_len) (sizeof-type " + t + "))")
		v.Expect(k == 0, "%d pushes of a %s still put it on the record stack", k, t)
	}
	k := v.Count("(cast (ptr regstar_T) rp)") + v.Count("(cast (ptr regbehind_T) rp)") + v.Count("(cast (ptr char) (. regstack ga_data))")
	v.Expect(k == 0, "the stack is still read as bytes somewhere -- %d times", k)
	return v.Done()
}
