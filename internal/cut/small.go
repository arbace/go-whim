package cut

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/edit"
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
// The extent is found by BRACE MATCHING from the definition's head, because
// replacing a function by guesswork is how an editor stops opening files.
func NoGlob(text []byte, w io.Writer) ([]byte, error) {
	blanked := edit.Blank(text)
	head := regexp.MustCompile(`(?m)^gen_expand_wildcards\([^\n]*\n`)
	m := head.FindIndex(text)
	if m == nil {
		return nil, fmt.Errorf("noglob: gen_expand_wildcards is not defined at file scope " +
			"any more, and replacing a function by guesswork is how an editor stops " +
			"opening files")
	}
	rel := bytes.IndexByte(blanked[m[1]:], '{')
	if rel < 0 {
		return nil, fmt.Errorf("noglob: gen_expand_wildcards is unbalanced")
	}
	opening := m[1] + rel
	close := edit.Match(blanked, opening)
	if close < 0 {
		return nil, fmt.Errorf("noglob: gen_expand_wildcards is unbalanced")
	}
	was := bytes.Count(text[opening:close], []byte{'\n'})
	var buf []byte
	buf = append(buf, text[:opening]...)
	buf = append(buf, "{\n    return save_patterns(num_pat, pat, num_file, file);\n}"...)
	buf = append(buf, text[close+1:]...)
	fmt.Fprintf(w, "  noglob       gen_expand_wildcards was %d lines, is now one; every "+
		"pattern names a file\n", was)
	return buf, nil
}

var (
	wildCall  = regexp.MustCompile(`\bmch_expand_wildcards\((num_pat, pat, num_file, file)[^)]*\)`)
	wildProto = regexp.MustCompile(`(?m)^static int mch_expand_wildcards\([^;\n]*\);$`)
)

// NoWild makes every delegation to the shell expander return the pattern
// unexpanded.
//
// THREE delegations, asserted.  gen_expand_wildcards has been reshaped before,
// and rewriting it by guesswork is how an editor stops opening files -- which
// is the same sentence NoGlob carries, for the same function, from the other
// side.
func NoWild(text []byte, w io.Writer) ([]byte, error) {
	calls := len(wildCall.FindAll(text, -1))
	if calls != 3 {
		return nil, fmt.Errorf("nowild: expected 3 delegations to the shell expander, "+
			"found %d -- gen_expand_wildcards has been reshaped and rewriting it by "+
			"guesswork is how an editor stops opening files", calls)
	}
	text = wildCall.ReplaceAll(text, []byte(`save_patterns($1)`))

	n := len(wildProto.FindAll(text, -1))
	if n != 1 {
		return nil, fmt.Errorf("nowild: expected one declaration of the shell expander "+
			"to reuse, found %d", n)
	}
	text = wildProto.ReplaceAll(text, []byte(
		"static int save_patterns(int num_pat, char_u **pat, int *num_file, char_u ***file);"))

	left := bytes.Count(text, []byte("mch_expand_wildcards"))
	fmt.Fprintf(w, "  nowild       %d delegations now return the pattern unexpanded; "+
		"%d mch_expand_wildcards mentions left for the sweep\n", calls, left)
	return text, nil
}

const equiOld = `                            c_class = get_equi_class(&regparse);
                            if (c_class != 0)
                            {
                                reg_equi_class(c_class);
                            }
                            else if ((c_class = get_coll_element(&regparse)) != 0)
`

const equiNew = `                                if ((c_class = get_coll_element(&regparse)) != 0)
`

var equiMentions = regexp.MustCompile(`\b(?:reg_equi_class|get_equi_class)\b`)

// NoEquiClass makes [= in a bracket expression no longer an equivalence
// class.
//
// Both edits are exact text, which is a dependency on spelling and is
// deliberate here: the bracket parser is a shape a regex would match in more
// places than it should.
func NoEquiClass(text []byte, w io.Writer) ([]byte, error) {
	if !bytes.Contains(text, []byte(equiOld)) {
		return nil, fmt.Errorf("noequiclass: the bracket parser is not where this expects")
	}
	text = bytes.Replace(text, []byte(equiOld), []byte(equiNew), 1)
	fmt.Fprintln(w, "  noequiclass  [= in a bracket expression is no longer an "+
		"equivalence class")

	skip := regexp.MustCompile(
		`get_char_class\(&p\) == CLASS_NONE && get_equi_class\(&p\) == 0 && `)
	locs := skip.FindAllIndex(text, -1)
	if len(locs) < 1 {
		return nil, fmt.Errorf("noequiclass: skip_regexp's scan is not where this expects")
	}
	l := locs[0]
	var buf []byte
	buf = append(buf, text[:l[0]]...)
	buf = append(buf, "get_char_class(&p) == CLASS_NONE && "...)
	buf = append(buf, text[l[1]:]...)
	text = buf
	fmt.Fprintln(w, "  noequiclass  skip_regexp stops asking the same question")
	fmt.Fprintf(w, "  noequiclass  %d mentions left for the sweep\n",
		len(equiMentions.FindAll(text, -1)))
	return text, nil
}
