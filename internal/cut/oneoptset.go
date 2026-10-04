package cut

import (
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
)

var (
	optModeline = regexp.MustCompile(`\bOPT_MODELINE\b`)
	doModelines = regexp.MustCompile(`\bdo_modelines\(`)
)

// OneOptSet leaves :set as the only way to give an option a value.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3a): the text version's heads and
// literals are acts on the nodes -- folds, an else-if arm each in three
// functions, calls and a store cut, operands dropped as the text's cuts
// left their neighbours, and the one string literal respelled whole
// (RespellString, RENAME's rule for strings) -- each counted, its report
// the text's.  The last counts are the text's own, on the C view (history
// keeps the text version).
func OneOptSet(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("oneoptset", e, w)

	v.InFunction("ex_set", func(v *graph.Verbs) {
		v.FoldNever("(== (-> eap cmdidx) CMD_setlocal)", 1, ":setlocal choosing OPT_LOCAL")
		v.FoldNever("(== (-> eap cmdidx) CMD_setglobal)", 1, ":setglobal choosing OPT_GLOBAL")
	})

	// completion for :setglobal and :setlocal went with
	// set_context_by_cmdname() at phase 4 (whim4f, phase 4f's program, which
	// runs before this phase now)

	v.InFunction("do_set_option", func(v *graph.Verbs) {
		v.RespellString(`"?=:!&<"`, `"?=:!&"`, 1, ":set accepting the < suffix")
	})
	for _, f := range []struct{ name, kind string }{
		{"do_set_option_bool", "a boolean"},
		{"do_set_option_numeric", "a number"},
		{"stropt_get_newval", "a string"},
	} {
		v.InFunction(f.name, func(v *graph.Verbs) {
			v.FoldNever("(== nextchar '<')", 1, ":set opt< copying the global value of "+f.kind)
		})
	}

	// THE ORDER IS THE PYTHON'S, and it is load-bearing: each edit prints a
	// line as it succeeds, so grouping these into loops by shape -- which they
	// invite -- would emit the same lines in a different order and the
	// comparison would differ on every input that cuts.
	// at the depth phase 5d's fold leaves it: whim5d runs at phase 5, before
	// this phase now
	v.InFunction("open_buffer", func(v *graph.Verbs) {
		v.Cut("(call do_modelines 0)", 1, "reading a buffer applying its modelines")
	})

	// do_write went with :write at phase 1 (filefront, the reform's D4)

	v.InFunction("do_ecmd", func(v *graph.Verbs) {
		v.Cut("(call do_modelines OPT_WINONLY)", 1, "editing a file applying its window modelines")
	})
	v.InFunction("set_rw_fname", func(v *graph.Verbs) {
		v.DropIf("(== (deref (-> curbuf b_p_ft)) NUL)", 1, "naming a buffer applying modelines")
	})
	v.InFunction("validate_opt_idx", func(v *graph.Verbs) {
		v.FoldNever("(& opt_flags OPT_MODELINE)", 1, "the options a modeline may not set")
	})
	v.InFunction("do_set", func(v *graph.Verbs) {
		v.DropOperandAsText("(! (& opt_flags OPT_MODELINE))", 2, ":set all and :set termcap refused in a modeline")
	})
	v.InFunction("do_set_option_string", func(v *graph.Verbs) {
		v.DropOperandAsText("(paren (& opt_flags OPT_MODELINE))", 1, "a modeline string option run securely")
	})
	v.InFunction("did_set_option", func(v *graph.Verbs) {
		v.DropOperandAsText("(paren (& opt_flags OPT_MODELINE))", 1, "a modeline value marked insecure")
	})
	v.InFunction("do_filetype_autocmd", func(v *graph.Verbs) {
		v.FoldNever("(&& (paren (& opt_flags OPT_MODELINE)) (! value_changed))", 1, "a modeline's unchanged 'filetype'")
	})

	// set_options_bin(), where 'binary' saved and restored 'modeline', went
	// with 'binary' at phase 5 (lfonly, record 50's cut, which runs before
	// this phase now)

	// b_p_ml_nobin is where 'binary' kept 'modeline' while it was off.  It is
	// not an option, so it has no get_varp() case and droplocal does not know
	// its shape: its one copy goes here, and the field and p_ml_nobin, with
	// no reader left, go to the collection.
	v.InFunction("buf_copy_options", func(v *graph.Verbs) {
		v.Cut("(= (-> buf b_p_ml_nobin) p_ml_nobin)", 1, "a new buffer copying the saved 'modeline'")
	})
	if v.Failed() {
		return v.Done()
	}

	text := v.Text()
	blanked := edit.Blank(text)
	var dying [][2]int
	for _, n := range []string{"chk_modeline", "do_modelines"} {
		if a, z, ok := edit.FindDefinition(text, blanked, n); ok {
			dying = append(dying, [2]int{a, z})
		}
	}
	live := 0
	for _, m := range optModeline.FindAllIndex(text, -1) {
		inDying := false
		for _, sp := range dying {
			if sp[0] <= m[0] && m[0] < sp[1] {
				inDying = true
				break
			}
		}
		if !inDying {
			live++
		}
	}
	if live != 1 {
		return fmt.Errorf("oneoptset: OPT_MODELINE outside its enumerator and the "+
			"dying modeline code -- %d, expected 1", live)
	}
	if n := len(doModelines.FindAll(text, -1)); n != 2 {
		return fmt.Errorf("oneoptset: do_modelines is still called")
	}

	v.Say(":set is the only way to give an option a value")
	return v.Done()
}
