package cut

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
)

// OneBuffer leaves one buffer: the old one is wiped, nothing is hidden,
// nothing is the alternate.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3a): the text version's line
// regexps and literals are acts on the nodes -- a case's run dropped, ifs
// dropped and folded, operands dropped, expressions rewritten, the CTRL-^
// row's handler pointed at nv_error -- each counted, its report the text's
// (history keeps it).  The leftover counts are the text's own, on the C
// view.
func OneBuffer(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("onebuffer", e, w)
	v.InFunction("parse_command_modifiers", func(v *graph.Verbs) {
		v.One(`(if (|| (!= p (-> eap cmd)) (! (call checkforcmd_noparen (addr p) "hide" 3)) (== (deref p) NUL) (call ends_excmd (deref p))) (block (break)))`,
			"the :hide modifier is not where this expects")
		v.DropCaseRun("(case 'h')", 1, "the :hide modifier")
		v.DropIf(`(call checkforcmd_noparen (addr (-> eap cmd)) "keepalt" 5)`, 1, "the :keepalt modifier")
	})
	v.InFunction("set_context_by_cmdname", func(v *graph.Verbs) {
		q := graph.NewVerbs("onebuffer", e, io.Discard)
		q.In(v.Scope(), func(q *graph.Verbs) {
			q.Cut("(case CMD_hide)", 1, "")
			q.Cut("(case CMD_keepalt)", 1, "")
		})
		if q.Err != nil {
			v.Err = q.Err
			return
		}
		v.Say("completion for :hide and :keepalt")
	})

	// do_argfile, :next, can_abandon, alist_add and alist_add_list died with
	// the argument-list and buffer commands, retired at phase 1 (exfront, the
	// reform's D2)

	// set_curbuf, which hid the buffer it left, went with ex_quit's refusal
	// at phase 1 (quitfront, the reform's move of phase 33)

	v.InFunction("getfile", func(v *graph.Verbs) {
		v.DropOperand("(! (call buf_hide curbuf))", 1, "getfile writing before it leaves")
		v.Rewrite("(+ (? (call buf_hide curbuf) ECMD_HIDE 0) ?rest)", "(paren ?rest)", 1,
			"getfile hiding the buffer")
	})

	// :quit's refusal, and with it its buf_hide test, folded at phase 1
	// (quitfront, phase 33's move)
	v.InFunction("ex_quit", func(v *graph.Verbs) {
		v.Rewrite("(call win_close wp (|| (! (call buf_hide (-> wp w_buffer))) (-> eap forceit)))",
			"(call win_close wp TRUE)", 1, ":quit freeing the buffer")
	})

	// ex_exit, do_exedit, rename_buffer, ex_read and do_write went with :exit,
	// :edit, :file, :read and :write at phase 1 (filefront, the reform's D4)

	v.InFunction("nv_gotofile", func(v *graph.Verbs) {
		v.DropOperand("(! (call buf_hide curbuf))", 1, "gf refusing a changed buffer")
		v.Rewrite("(? (call buf_hide curbuf) ECMD_HIDE 0)", "0", 1, "gf hiding the buffer")
	})
	if v.Failed() {
		return v.Done()
	}
	if n := len(edit.CallsNotAfterWord(v.Text(), "buf_hide")); n != 2 {
		return fmt.Errorf("onebuffer: buf_hide is still called -- %d mentions, expected "+
			"its prototype and definition", n)
	}

	v.InFunction("close_buffer", func(v *graph.Verbs) {
		for _, b := range []struct{ ch, what string }{
			{"d", "delete"}, {"w", "wipe"}, {"u", "unload"},
		} {
			v.FoldNever("(== (index (-> buf b_p_bh) 0) '"+b.ch+"')", 1,
				"close_buffer's bufhidden="+b.what)
		}
	})

	v.InFunction("do_ecmd", func(v *graph.Verbs) {
		v.Rewrite("(? (paren (& flags ECMD_HIDE)) 0 DOBUF_UNLOAD)", "DOBUF_WIPE", 1,
			"do_ecmd wiping the buffer it leaves")
		v.DropIf("(&& (== fnum 0) other_file (!= ffname nullptr))", 1,
			"do_ecmd naming a refused file the alternate")
		v.DropIf("(== (& (. cmdmod cmod_flags) CMOD_KEEPALT) 0)", 1,
			"do_ecmd making the old buffer the alternate")
		v.Cut("(if (!= oldwin nullptr) (block (call buflist_altfpos oldwin)))", 1,
			"do_ecmd saving the old window position")
		v.DropIf("(&& (== (-> curwin w_alt_fnum) (-> buf b_fnum)) (!= prev_alt_fnum 0))", 1,
			"do_ecmd restoring the alternate")
	})

	// set_curbuf went with ex_quit's refusal at phase 1 (quitfront)

	v.InFunction("win_init", func(v *graph.Verbs) {
		v.Cut("(= (-> newp w_alt_fnum) (-> oldp w_alt_fnum))", 1, "win_init copying the alternate")
	})
	v.InFunction("buflist_findnr", func(v *graph.Verbs) {
		v.DropIf("(== nr 0)", 1, "buffer 0 meaning the alternate")
	})
	v.InFunction("buflist_findpat", func(v *graph.Verbs) {
		v.Rewrite("(= match (-> curwin w_alt_fnum))", "(= match 0)", 1, "'#' finding the alternate")
	})

	// A ROW IS NEVER DELETED FROM nv_cmds[], IT IS POINTED AT nv_error.
	v.InTable("nv_cmds", func(v *graph.Verbs) {
		v.RewriteAt("(init Ctrl_HAT ?h NV_NCW 0)", "h", "nv_error", 1, "CTRL-^'s row points at nv_error")
	})
	if v.Failed() {
		return v.Done()
	}

	// setaltfname() and buf_hide() have no caller now and still name the
	// alternate and the two modifier flags; the collection takes them, and
	// record 42's program counted again after it.  Everything else is
	// counted here, by the text's own question on the C view.
	text := v.Text()
	blanked := edit.Blank(text)
	var dying [][2]int
	for _, n := range []string{"setaltfname", "buf_hide"} {
		if a, z, ok := edit.FindDefinition(text, blanked, n); ok {
			dying = append(dying, [2]int{a, z})
		}
	}
	count := func(pattern string) int {
		n := 0
		for _, m := range edit.AllIndex(regexp.MustCompile(pattern), text) {
			in := false
			for _, sp := range dying {
				if sp[0] <= m[0] && m[0] < sp[1] {
					in = true
					break
				}
			}
			if !in {
				n++
			}
		}
		return n
	}
	var left []string
	for _, c := range oneBufferLeft {
		if n := count(c.pattern); n != c.want {
			left = append(left, fmt.Sprintf("(%s, %d)", edit.PyRepr(c.what), n))
		}
	}
	if len(left) > 0 {
		return fmt.Errorf("onebuffer: still present: [%s]", strings.Join(left, ", "))
	}

	v.Say("one buffer: the old one is wiped, nothing is hidden, nothing is the alternate")
	return v.Done()
}

// oneBufferLeft is what OneBuffer counts outside the two dying functions,
// and how many of each it requires.
var oneBufferLeft = []struct {
	what, pattern string
	want          int
}{
	{"w_alt_fnum outside its field", `\bw_alt_fnum\b`, 2},
	{"CMOD_KEEPALT or CMOD_HIDE outside their enumerators", `\bCMOD_(?:KEEPALT|HIDE)\b`, 2},
	{"buflist_altfpos called", `\bbuflist_altfpos\(curwin\)|\bbuflist_altfpos\(oldwin\)`, 0},
	{"nv_hat in the key table", `\{Ctrl_HAT, nv_hat`, 0},
}
