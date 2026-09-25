package p076

// Whim phase 76 -- one regexp engine, so no retry.  See GOAL.md.
//
// PROVED BY A SINGLE ASSIGNMENT.  `prog->re_engine = BACKTRACKING_ENGINE` is the only
// place re_engine is ever written, so the field can hold no other value -- and both
//
// if (rmp->regprog->re_engine == AUTOMATIC_ENGINE && result == -1)
//
// blocks, one in vim_regexec_string and one in vim_regexec_multi, are unreachable.
// They exist to recompile a pattern with the backtracking engine when the automatic
// choice failed; with one engine there is nothing to fall back to.  nfa_regengine and
// regexp_engine are already at zero mentions -- the NFA engine went in an earlier
// phase and these two blocks are what was left pointing at its corpse.
//
// WHAT GOES WITH THEM:
// * p_re entirely.  It is an ORPHAN OPTION -- no row in the option table, so it can
// never be set and reads as 0 -- and its only uses are the `< 0 || > 2`
// validation, which can therefore never fire, and the save/restore inside the two
// dead blocks.
// * AUTOMATIC_ENGINE, which has no other reader.
// * nfa_regprog_T and nfa_state_T, by cascade: their only non-type mentions are the
// two `((nfa_regprog_T *)rmp->regprog)->pattern` casts INSIDE the dead blocks.
// A husk kept alive purely by unreachable code.
//
// Audited before writing: both blocks are 25 lines, carry no break or continue that
// would rebind, contain no label, and are followed by no else -- so fold_never takes
// them without the hazards phases 71, 72 and 75 each ran into.
//
// THE DELTA: none expected.  The blocks never ran, so removing them cannot change a
// match.  Declared empty and left for the delta check to correct.

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
)

var (
	btEngineWrite = regexp.MustCompile(`(?m)^[ \t]*prog->re_engine = BACKTRACKING_ENGINE;[ \t]*$`)
	retryOther    = edit.Head("if (rmp->regprog->re_engine == AUTOMATIC_ENGINE && result == (-1))")
)

// assignsTo is writesTo with `->name` fields included -- written the careful way
// after whim75, where a first guard matched `name[^\n;]*=` and reported the `!=`
// of three predicates as writes.  An assertion that cries wolf invites being
// loosened until it passes.
func assignsTo(text []byte, name string) []int {
	var Out []int
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
	for _, m := range re.FindAllIndex(text, -1) {
		end := m[1] + 80
		if end > len(text) {
			end = len(text)
		}
		s := strings.TrimLeft(string(text[m[1]:end]), " \t\n")
		if strings.HasPrefix(s, "[") {
			depth := 0
			for i, ch := range s {
				if ch == '[' {
					depth++
				} else if ch == ']' {
					depth--
					if depth == 0 {
						s = s[i+1:]
						break
					}
				}
			}
			s = strings.TrimLeft(s, " \t\n")
		}
		if strings.HasPrefix(s, "=") && !strings.HasPrefix(s, "==") {
			Out = append(Out, 1+edit.CountNewlines(text[:m[0]]))
		}
	}
	return Out
}

// Whim76 leaves one regexp engine, having first proved there is only one.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("oneengine", text, w)

	if ws := assignsTo(text, "re_engine"); len(ws) != 1 {
		e.Refuse("re_engine is assigned in %d place(s) (lines %s), not once -- a second engine may exist and both retry blocks may be reachable",
			len(ws), edit.JoinInts(ws))
		return e.Done()
	}
	if !btEngineWrite.Match(text) {
		e.Refuse("the one assignment to re_engine is not to BACKTRACKING_ENGINE")
		return e.Done()
	}
	for _, gone := range []string{"nfa_regengine", "regexp_engine"} {
		if regexp.MustCompile(`\b` + gone + `\b`).Match(text) {
			e.Refuse("%s still exists -- the NFA engine is back and this phase is wrong", gone)
			return e.Done()
		}
	}
	e.Say("confirmed: re_engine is written once, to BACKTRACKING_ENGINE, and only there")

	e.InFunction("vim_regexec_string", func(e *edit.E) { e.FoldNever(retryOther, 1, "a failed match recompiling with the other engine") })
	e.InFunction("vim_regexec_multi", func(e *edit.E) { e.FoldNever(retryOther, 1, "and the multi-line variant of the same") })
	e.InFunction("check_num_option_bounds", func(e *edit.E) {
		e.Literal(w76lit2, "", 1, "validating an option nothing can set")
	})
	// p_re, which had no row to set it, and AUTOMATIC_ENGINE, the engine it
	// chose between, are named by nothing now; the sweep takes them.
	return e.Done()
}

func init() { edit.Register("whim76", Edit) }
