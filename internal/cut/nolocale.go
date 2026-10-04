package cut

import (
	"bytes"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// NoLocale stops the editor asking the locale anything.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): each of the text's line cuts is
// a Cut, a CutRun or a DropCase by form, the rows DeleteRows, and the
// `(void)mb_init();` a Rewrite (history keeps the text version).
func NoLocale(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nolocale", e, w)
	v.Cut("(call init_locale)", 1, "the setlocale at startup")
	// NOT a deletion.  set_init_default_encoding() did three things: ask the
	// locale, re-initialise the multibyte layer for whatever it answered, and
	// write that back as the option's default.  Only the first is locale.  The
	// second is load-bearing and invisible: p_enc is set from the option
	// table's default, and nothing acts on it until mb_init() runs.  Delete
	// the call outright and 'encoding' reports utf-8 while enc_utf8 is still
	// FALSE -- the editor says UTF-8 and behaves like latin1, which is worse
	// than either, and which five multibyte behaviour cases caught.
	v.ReplaceC("(call set_init_default_encoding)", "(void)mb_init();", 1,
		"deriving 'encoding' from the locale, keeping the mbyte init it also did")
	// 'encoding''s row, whose default was latin1, is dropped at phase 1 with
	// every option the product has not (optfront, the reform's D3)
	// :sort's locale-aware collation died with :sort, retired at phase 1
	// (exfront, the reform's D2)
	v.CutRun("the $LANG-gated maintainer line in :messages",
		`(= s (cast (ptr char_u) (call getenv (cast (ptr char) (paren (cast (ptr char_u) "LANG"))))))`,
		"(if (&& (!= s nullptr) (!= (deref s) NUL)) (block (call msg_attr _ _)))")
	v.DropCase("(case CMD_language)", 1, "the :language completion case")
	v.InFunction("ExpandOther", func(v *graph.Verbs) {
		tab := v.One("(def static tab _ _)", "the completion dispatch table")
		if tab == nil {
			return
		}
		v.In(tab, func(v *graph.Verbs) {
			deleteRowsTyped(v, []string{"(init EXPAND_LANGUAGE get_lang_arg TRUE FALSE)",
				"(init EXPAND_LOCALES get_locales TRUE FALSE)"},
				"the two locale rows of the completion dispatch table")
		})
	})
	v.InTable("command_complete_tab", func(v *graph.Verbs) {
		deleteRowTyped(v, "(init (paren EXPAND_LOCALES) _)",
			"-complete=locale as a name :command accepts")
	})
	// dropDbcsConversion was the one that kept `setlocale` alive after
	// everything else had gone: mb_init() asks the locale what a DBCS
	// terminal is speaking so it can convert messages into it; the block is
	// guarded by `if (enc_dbcs)`, which made it easy to miss and impossible
	// to reach for any encoding this build has.  Only the block: `vimconv`
	// and its CONV_NONE initialiser STAY, since mb_init tests vimconv.vc_type
	// again further down.
	v.InFunction("mb_init", func(v *graph.Verbs) {
		v.Muted(func(v *graph.Verbs) {
			v.Run("mb_init no longer sets up a conversion here",
				"(= (. vimconv vc_type) CONV_NONE)", "(if enc_dbcs _)")
			v.Cut("(if enc_dbcs _)", 1, "the enc_dbcs block")
		})
	})
	v.Say("the DBCS locale conversion in mb_init, and its local")
	if v.Failed() {
		return v.Done()
	}
	v.Sayf("%d setlocale calls left for the sweep", bytes.Count(v.Text(), []byte("setlocale(")))
	return v.Done()
}
