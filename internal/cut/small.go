package cut

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/graph"
)

// NoIntro cuts the splash screen.
//
// The count is asserted, not hoped for: exactly TWO maybe_intro_message()
// call sites.  A different number means the redraw path
// has moved under the phase, and a partial cut would leave one splash behind.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B1a): the two calls are deleted as
// items, counted; the text version cut their lines (history keeps it).
func NoIntro(e *graph.Editor, w io.Writer) error {
	// :intro and :version point at ex_ni from phase 1 (exfront, D2)
	v := graph.NewVerbs("nointro", e, w)
	n := v.Count("(call maybe_intro_message)")
	if n != 2 {
		return fmt.Errorf("nointro: expected two splash call sites, removed %d -- "+
			"the redraw path has moved under this phase", n)
	}
	v.Cut("(call maybe_intro_message)", 2, "2 splash call sites cut")
	return v.Done()
}

// NoGlob replaces gen_expand_wildcards' body with one that treats every
// pattern as a name.
//
// The extent was found by BRACE MATCHING from the definition's head, because
// replacing a function by guesswork is how an editor stops opening files; on
// the graph (doc/GRAPH-MIGRATION.md, B3a) the body is the definition's
// items, replaced by its one statement (Body), the old body's length the
// text's number on the C view (history keeps the text version).
func NoGlob(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("noglob", e, w)
	if e.Defn("gen_expand_wildcards") == nil {
		return fmt.Errorf("noglob: gen_expand_wildcards is not defined at file scope " +
			"any more, and replacing a function by guesswork is how an editor stops " +
			"opening files")
	}
	was := 0
	v.InFunction("gen_expand_wildcards", func(v *graph.Verbs) { was = bodyLines(v.Text()) })
	q := graph.NewVerbs("noglob", e, io.Discard)
	q.Body("gen_expand_wildcards", "(return (call save_patterns num_pat pat num_file file))", "gen_expand_wildcards")
	if err := q.Done(); err != nil {
		return err
	}
	v.Sayf("gen_expand_wildcards was %d lines, is now one; every pattern names a file", was)
	return v.Done()
}

// NoWild makes every delegation to the shell expander return the pattern
// unexpanded.
//
// THREE delegations, asserted.  gen_expand_wildcards has been reshaped before,
// and rewriting it by guesswork is how an editor stops opening files -- which
// is the same sentence NoGlob carries, for the same function, from the other
// side.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3a): the shell expander's
// prototype becomes save_patterns' (FRAG: a declaration in C), and each
// call is save_patterns' with the flags left out, rebuilt from its own
// arguments; one unit for both.  The mentions left are the text's count,
// on the C view (history keeps the text version).
func NoWild(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nowild", e, w)
	const call = "(call mch_expand_wildcards num_pat pat num_file file _)"
	calls := v.Count(call)
	if calls != 3 {
		return fmt.Errorf("nowild: expected 3 delegations to the shell expander, "+
			"found %d -- gen_expand_wildcards has been reshaped and rewriting it by "+
			"guesswork is how an editor stops opening files", calls)
	}
	var protos []*graph.Node
	for _, d := range e.FileDecls("mch_expand_wildcards") {
		if d.Is("def") {
			protos = append(protos, d)
		}
	}
	if len(protos) != 1 {
		return fmt.Errorf("nowild: expected one declaration of the shell expander "+
			"to reuse, found %d", len(protos))
	}
	q := graph.NewVerbs("nowild", e, io.Discard)
	q.Together(func(q *graph.Verbs) {
		q.In(protos[0], func(q *graph.Verbs) {
			q.ReplaceC("(def static mch_expand_wildcards _)",
				"static int save_patterns(int num_pat, char_u **pat, int *num_file, char_u ***file);", 1, "the prototype")
		})
		q.ReplaceC(call, "save_patterns(num_pat, pat, num_file, file)", calls, "the delegations")
	})
	if q.Err != nil {
		return q.Err
	}
	left := bytes.Count(v.Text(), []byte("mch_expand_wildcards"))
	v.Sayf("%d delegations now return the pattern unexpanded; "+
		"%d mch_expand_wildcards mentions left for the sweep", calls, left)
	return v.Done()
}

var equiMentions = regexp.MustCompile(`\b(?:reg_equi_class|get_equi_class)\b`)

// NoEquiClass makes [= in a bracket expression no longer an equivalence
// class.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the bracket parser's
// statement cut and its if folded never (its else-if arm stays), and
// skip_regexp's operand dropped, each counted once; the text version
// matched exact text (history keeps it).
func NoEquiClass(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("noequiclass", e, w)
	q := graph.NewVerbs("noequiclass", e, io.Discard)
	q.Cut("(= c_class (call get_equi_class (addr regparse)))", 1, "the bracket parser's equivalence class")
	q.FoldNever("(if (!= c_class 0) (block (call reg_equi_class c_class)) _)", 1, "the bracket parser's test")
	if q.Failed() {
		return fmt.Errorf("noequiclass: the bracket parser is not where this expects -- %v", q.Err)
	}
	v.Say("[= in a bracket expression is no longer an equivalence class")
	q.DropOperand("(== (call get_equi_class (addr p)) 0)", 1, "skip_regexp's scan")
	if q.Failed() {
		return fmt.Errorf("noequiclass: skip_regexp's scan is not where this expects -- %v", q.Err)
	}
	v.Say("skip_regexp stops asking the same question")
	v.Sayf("%d mentions left for the sweep", len(equiMentions.FindAll(v.Text(), -1)))
	return v.Done()
}
