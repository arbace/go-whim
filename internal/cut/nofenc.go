package cut

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

var nofencStubs = []struct{ name, body string }{
	{"bomb_size", "(return 0)"},
	{"add_b0_fenc", ""},
}

// NoFenc takes 'fileencoding' and 'bomb' away.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): each of the text's line cuts is
// a Cut or a CutRun by form, buf_write's target a ReplaceC, file_ff_differs'
// tail a Splice, the stubs Body (history keeps the text version).
func NoFenc(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nofenc", e, w)
	v.InFunction("buf_write", func(v *graph.Verbs) {
		v.ReplaceC("(= fenc (-> buf b_p_fenc))", `fenc = (char_u *)"";`, 1,
			"buf_write taking the buffer's 'fileencoding' as its target")
	})
	// readfile's 'fileencoding', its 'bomb' set and cleared and its BOM test went
	// with readfile, which dies at record 13 since ml_recover went at phase 1
	// (norecover, the reform's D5); so did the swap file's block-zero
	// encoding and its restore.
	v.CutRun("save_file_ff remembering the BOM and the encoding",
		"(= (-> buf b_start_bomb) (-> buf b_p_bomb))",
		"(if (|| (== (-> buf b_start_fenc) nullptr) _) (block (call vim_free (-> buf b_start_fenc)) (= (-> buf b_start_fenc) (call vim_strsave (-> buf b_p_fenc)))))")
	v.InFunction("file_ff_differs", func(v *graph.Verbs) {
		v.Splice("(if (&& (! (-> buf b_p_bin)) (!= (-> buf b_start_bomb) (-> buf b_p_bomb))) (block (return TRUE)))",
			"(return (paren (!= (call strcmp _ _) 0)))", "(return FALSE)", "file_ff_differs comparing them")
	})
	v.Cut("(if (&& enc_utf8 (paren (& (call enc_canon_props (-> curbuf b_p_fenc)) ENC_8BIT))) "+
		"(block (call convert_setup (addr vimconv) p_enc (-> curbuf b_p_fenc))))", 1,
		"g8 converting to the buffer's 'fileencoding' to find an illegal byte")
	v.Cut("(if (&& (-> buf b_p_bomb) (! write_bin) _) _)", 1, "buf_write writing a BOM it no longer makes")
	// did_set_encoding's arm for 'fileencoding', the empty test record 15 left
	// there and gvarp went with the encoding rows, dropped at phase 1
	// (optfront, the reform's D3).
	v.CutRun("freeing the remembered encoding", "(call vim_free (-> buf b_start_fenc))",
		"(= (paren (-> buf b_start_fenc)) nullptr)")
	// three: at phase 2 (the reform's D7) readfile and the recovery are
	// still in the text, and two of them are theirs
	v.Cut("(= (-> _ b_start_bomb) FALSE)", 3, "clearing the remembered BOM")
	// LOOKUPS BY NAME, and the reason this phase needed two attempts (three,
	// before readfile's and the recovered swap file's went with them).
	// set_string_option_direct((char_u *)"fenc", ...) resolves the option
	// through findoption(), which answers -1 for a row that is not there; the
	// caller does not check, so silent Ex mode exits 1 without printing
	// anything, and every recorded exit status in the harness moves at once.
	v.CutRun("`:e ++enc=` forcing one",
		"(def fenc (ptr char_u) (call enc_canonize (+ (-> eap cmd) (-> eap force_enc))))",
		`(if (!= fenc nullptr) (block (call set_string_option_direct (cast (ptr char_u) "fenc") _*)))`,
		"(call vim_free fenc)")

	for _, s := range nofencStubs {
		v.Body(s.name, s.body, s.name+" answers for a file that has no BOM")
	}
	return v.Done()
}
