package cut

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// newDispatch is the one case mb_init's encoding dispatch has left.
const newDispatch = `if (strcmp((char *)(p_enc), (char *)("utf-8")) != 0)
{
    return e_invalid_argument;
}
enc_unicode = 0;
enc_utf8 = TRUE;
enc_dbcs = 0;
has_mbyte = TRUE;
enc_latin1like = TRUE;
`

var noencStubs = []struct{ name, stub string }{
	{"my_iconv_open", "return (void *)(iconv_t)-1;"},
	{"convert_setup", "vcp->vc_type = CONV_NONE;\n" +
		"vcp->vc_factor = 1;\n" +
		"vcp->vc_fail = FALSE;\n" +
		"return OK;"},
	{"string_convert", "return nullptr;"},
	{"check_for_bom", "*lenp = 0;\nreturn nullptr;"},
	{"make_bom", "return 0;"},
	{"convert_input_safe", "if (restp != nullptr)\n{\n*restp = nullptr;\n}\nreturn len;"},
}

// noencIconvBlocks are the last two symbols, and the only place this phase
// touches readfile() or buf_write().  my_iconv_open() now always fails, so
// every one of these blocks is a branch that can no longer be taken -- but the
// calls inside them are what keep `iconv` and `iconv_close` in the symbol
// table, and a dependency that is linked in and never reached is exactly what
// this pipeline exists to remove.
//
// EVERY PATTERN NAMES THE BODY, not just the condition, and has no else:
// `if (fio_flags == 0)` occurs twice in readfile() and the first one has an
// `else` after it.  Two blocks are the same form, the first readfile's
// close and the read loop giving up on a conversion: the first in the file
// is the one, as the text's first match was (first).
var noencIconvBlocks = []struct {
	pat, what string
	first     bool
}{
	{"(if (!= (-> ip bw_iconv_fd) (cast iconv_t (- 1))) (block (def from (ptr (const char))) _*))",
		"buf_write's conversion", false},
	{"(if (&& converted (== wb_flags 0)) (block (= (. write_info bw_iconv_fd) (cast iconv_t (call my_iconv_open _ _))) _*))",
		"buf_write's iconv open", false},
	{"(if (!= (. write_info bw_iconv_fd) (cast iconv_t (- 1))) (block (call iconv_close _) _*))",
		"buf_write's iconv close", false},
	{"(if (== fio_flags 0) (block (= iconv_fd (cast iconv_t (call my_iconv_open _ _))) _*))",
		"readfile's iconv open", false},
	{"(if (!= iconv_fd (cast iconv_t (- 1))) (block (call iconv_close iconv_fd) (= iconv_fd (cast iconv_t (- 1)))))",
		"readfile's iconv close", true},
	{"(if (!= iconv_fd (cast iconv_t (- 1))) (block (def fromp (ptr (const char))) _*))",
		"readfile's conversion loop", false},
	// Two more closes, nested deeper: one where the read loop gives up on a
	// conversion, one in readfile's exit path.
	{"(if (!= iconv_fd (cast iconv_t (- 1))) (block (call iconv_close iconv_fd) (= iconv_fd (cast iconv_t (- 1)))))",
		"the read loop giving up on a conversion", false},
	{"(if (!= iconv_fd (cast iconv_t (- 1))) (block (call iconv_close iconv_fd)))",
		"readfile's exit path", false},
}

// NoEnc leaves one encoding: nothing calls iconv any more.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): mb_init's dispatch -- the else
// of `if (p_enc == nullptr)` cut, and the run from the DBCS test to
// enc_latin1like written anew (FRAG) -- and its function-pointer table's
// utf-8 arm kept (KeepThen); the 'fileencodings' defaults by form, the
// stubs BodyC, the iconv blocks Cut by form (history keeps the text
// version).
func NoEnc(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("noenc", e, w)
	v.InFunction("mb_init", func(v *graph.Verbs) {
		v.Muted(func(v *graph.Verbs) {
			v.Cut(`(if (|| (== (call strncmp _ (cast (ptr char) (paren "8bit-")) _) 0) _) _ _)`, 1,
				"mb_init's encoding dispatch")
			v.SpliceRunC("(if (!= enc_dbcs_new 0) _)", "(= enc_latin1like _)", newDispatch,
				"mb_init's encoding dispatch")
		})
		v.Say("mb_init accepts utf-8 and rejects every other value")
		// The function-pointer table: keep the utf-8 arm, drop the other two.
		v.Muted(func(v *graph.Verbs) {
			v.KeepThen("(if enc_utf8 (block (= mb_ptr2len utfc_ptr2len) _*) (if (!= enc_dbcs 0) _ _))", 1,
				"the utf-8 arm")
		})
		v.Say("the latin1 and DBCS character paths lose their only caller")
		// mb_init() installs a default 'fileencodings' whenever the
		// encoding is unicode and the user has not set one.  With enc_utf8
		// now always true that fires at every startup, and it reaches the
		// option BY NAME -- set_string_option_direct("fencs", ...) -- so
		// dropping the row turns it into E685 and then a segfault before
		// the first keystroke.
		v.Cut(`(if (&& enc_utf8 (! (call option_was_set (cast (ptr char_u) "fencs")))) (block (call set_fencs_unicode)))`, 1,
			"mb_init stops installing a default 'fileencodings'")
	})
	// 'fileencodings''s row, whose default this emptied, is dropped at phase 1
	// with every option the product has not (optfront, the reform's D3).

	// And a second caller: set_option_default() special-cases
	// 'fileencodings' so that RESETTING it picks the unicode list rather
	// than the compiled default.
	v.InFunction("set_option_default", func(v *graph.Verbs) {
		v.FoldNever("(&& (== (. (index options opt_idx) var) (cast (ptr char_u) (addr p_fencs))) enc_utf8)", 1,
			"resetting 'fileencodings' stops picking a unicode list")
	})

	total := 0
	for _, s := range noencStubs {
		was := 0
		v.InFunction(s.name, func(v *graph.Verbs) { was = bodyLines(v.Text()) })
		v.Muted(func(v *graph.Verbs) { v.BodyC(s.name, s.stub, s.name) })
		if v.Failed() {
			return v.Done()
		}
		total += was
		v.Sayf("%-20s was %3d lines, is now a constant answer", s.name, was)
	}

	for _, b := range noencIconvBlocks {
		if !b.first {
			v.Cut(b.pat, 1, b.what+", a branch that can no longer be taken")
			continue
		}
		ns := v.Find(b.pat)
		if len(ns) != 2 {
			v.Die("%s -- %d matches, expected 2", b.what, len(ns))
			break
		}
		if err := e.Delete(ns[0]); err != nil {
			v.Die("%s -- %v", b.what, err)
			break
		}
		v.Sayf("%s, a branch that can no longer be taken", b.what)
	}
	if v.Failed() {
		return v.Done()
	}
	v.Sayf("%d lines stubbed; nothing calls iconv any more", total)
	return v.Done()
}
