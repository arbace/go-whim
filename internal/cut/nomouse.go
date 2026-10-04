package cut

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
)

// ke is a `case (-((KS_EXTRA) + ((int)(NAME) << 8))) :` label, as the macro
// expander leaves it, as a form.
func ke(name string) string {
	return "(case (paren (- (+ (paren KS_EXTRA) (<< (cast int (paren " + name + ")) 8)))))"
}

func kes(names ...string) []string {
	var out []string
	for _, n := range names {
		out = append(out, ke(n))
	}
	return out
}

var insMouseKeys = []string{
	"KE_LEFTMOUSE", "KE_LEFTMOUSE_NM", "KE_LEFTDRAG", "KE_LEFTRELEASE",
	"KE_LEFTRELEASE_NM", "KE_MOUSEMOVE", "KE_MIDDLEMOUSE",
	"KE_MIDDLEDRAG", "KE_MIDDLERELEASE", "KE_RIGHTMOUSE",
	"KE_RIGHTDRAG", "KE_RIGHTRELEASE", "KE_X1MOUSE", "KE_X1DRAG",
	"KE_X1RELEASE", "KE_X2MOUSE", "KE_X2DRAG", "KE_X2RELEASE",
}

// mouseKeyNames are the key-name table's mouse rows, by their name.
var mouseKeyNames = map[string]bool{}

func init() {
	for _, n := range []string{"LeftDrag", "LeftMouse", "LeftRelease", "LeftReleaseNM",
		"MiddleMouse", "MouseMove", "RightMouse", "ScrollWheelDown", "ScrollWheelLeft",
		"ScrollWheelRight", "ScrollWheelUp", "X1Mouse", "X2Mouse", "Mouse"} {
		mouseKeyNames[`"`+n+`"`] = true
	}
}

var anyMouse = regexp.MustCompile(`(?i)mouse`)

// NoMouse removes the mouse.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the text's regexps are forms --
// the nv_cmds handlers Rewrite to nv_error, the key-name rows DeleteRows
// (INITROW), the case runs CutRun, the statements Cut, the condition's term
// DropOperand, set_termname()'s block a run of three items, and
// WaitForCharOrMouse's body FRAG'd into WaitForChar; the line count and
// the leftover mentions the text's, on the C view (history keeps the text
// version).  Acts the text reported once for several, or not at all, run
// quiet.
func NoMouse(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nomouse", e, w)
	q := graph.NewVerbs("nomouse", e, io.Discard)

	// POINTED AT nv_error, NEVER DELETED.  nv_cmd_idx[] is a sorted index into
	// nv_cmds[], computed once and written into the C; deleting these rows
	// left it 22 entries longer than the table, and every key found past the
	// first hole -- the arrows among them -- resolved to the wrong row.
	q.InTable("nv_cmds", func(q *graph.Verbs) {
		a, b := q.Count("(init _ nv_mouse _ _)"), q.Count("(init _ nv_mousescroll _ _)")
		if a+b != 22 {
			q.Die("expected 22 nv_cmds mouse rows, matched %d", a+b)
			return
		}
		q.Rewrite("nv_mouse", "nv_error", a, "the nv_mouse rows")
		q.Rewrite("nv_mousescroll", "nv_error", b, "the nv_mousescroll rows")
	})
	if q.Err != nil {
		return q.Err
	}
	v.Sayf("%d rows of nv_cmds answer nv_error", 22)

	q.InTable("key_names_table", func(q *graph.Verbs) {
		p := clisp.MustPattern("(init TRUE _ (init (cast _ (paren ?s)) _) _)")
		// the rows kept, as one arrangement typed as the import types it
		var keep []*graph.Node
		gone := 0
		for _, r := range graph.TableInit(q.Scope()).Args() {
			if b, ok := graph.Match(p, r); ok && mouseKeyNames[b["s"].Atom] {
				gone++
			} else {
				keep = append(keep, r)
			}
		}
		if gone != 14 {
			q.Die("expected 14 key-name rows, matched %d", gone)
			return
		}
		if _, err := e.ArrangeRowsTyped(q.Scope(), keep, graph.RowIndex{}); err != nil {
			q.Die("the key-name rows -- %v", err)
		}
	})
	if q.Err != nil {
		return q.Err
	}
	v.Sayf("%d rows of the key-name table", 14)

	q.InFunction("edit", func(q *graph.Verbs) {
		q.CutRun("edit()'s mouse cases", append(kes(insMouseKeys...), "(call ins_mouse c)", "(break)")...)
		for _, k := range []string{"KE_MOUSEDOWN", "KE_MOUSEUP", "KE_MOUSELEFT", "KE_MOUSERIGHT"} {
			q.CutRun("edit()'s "+k+" case", ke(k), "(call ins_mousescroll _)", "(break)")
		}
	})
	if q.Err != nil {
		return q.Err
	}
	v.Say("edit()'s mouse and scroll cases")

	const notChanged = "(goto cmdline_not_changed)"
	q.InFunction("getcmdline_int", func(q *graph.Verbs) {
		q.CutRun("getcmdline_int()'s middle drag and release",
			append(kes("KE_MIDDLEDRAG", "KE_MIDDLERELEASE"), notChanged)...)
		q.CutRun("getcmdline_int()'s middle-click paste", ke("KE_MIDDLEMOUSE"),
			"(if (! (call mouse_has MOUSE_COMMAND)) (block (goto cmdline_not_changed)))",
			"(call cmdline_paste 0 TRUE TRUE)", "(call redrawcmd)", "(goto cmdline_changed)")
		q.CutRun("getcmdline_int()'s click and drag", append(append(
			kes("KE_LEFTDRAG", "KE_LEFTRELEASE", "KE_RIGHTDRAG", "KE_RIGHTRELEASE"),
			"(if ignore_drag_release (block (goto cmdline_not_changed)))",
			"(attributed (std-attr fallthrough))"),
			append(kes("KE_LEFTMOUSE", "KE_RIGHTMOUSE"),
				"(call cmdline_left_right_mouse c (addr ignore_drag_release))", notChanged)...)...)
		q.CutRun("getcmdline_int()'s scroll cases",
			append(kes("KE_MOUSEDOWN", "KE_MOUSEUP", "KE_MOUSELEFT", "KE_MOUSERIGHT"), notChanged)...)
		q.CutRun("getcmdline_int()'s side-button cases",
			append(kes("KE_X1MOUSE", "KE_X1DRAG", "KE_X1RELEASE", "KE_X2MOUSE", "KE_X2DRAG",
				"KE_X2RELEASE", "KE_MOUSEMOVE"), notChanged)...)
		// Its declaration is the sweep's once nothing else names it.
		q.Cut("(= ignore_drag_release TRUE)", 1, "ignore_drag_release's other assignment")
	})
	if q.Err != nil {
		return q.Err
	}
	v.Say("getcmdline_int()'s six mouse case runs")

	q.InFunction("nv_brackets", func(q *graph.Verbs) {
		q.Cut("(cast void (call do_mouse (-> cap oap) (-> cap nchar) _ (-> cap count1) PUT_FIXINDENT))", 1,
			"nv_brackets()'s ]<LeftMouse>")
	})
	q.InFunction("nv_g_cmd", func(q *graph.Verbs) {
		q.Cut("(cast void (call do_mouse oap (-> cap nchar) (paren (- 1)) (-> cap count1) 0))", 1,
			"nv_g_cmd()'s g<LeftMouse>")
	})
	if q.Err != nil {
		return q.Err
	}
	v.Say("]<LeftMouse> and g<LeftMouse>")

	q.InFunction("wait_return", func(q *graph.Verbs) {
		q.Cut("(cast void (call jump_to_mouse MOUSE_SETPOS nullptr 0))", 1, "wait_return()'s jump_to_mouse")
		q.DropOperand("(paren (&& (! (call mouse_has MOUSE_RETURN)) (< mouse_row msg_row) _))", 1,
			"wait_return()'s mouse_has term")
	})
	if q.Err != nil {
		return q.Err
	}
	v.Say("the click that dismissed a `Press ENTER` prompt")

	q.InFunction("check_termcode", func(q *graph.Verbs) {
		q.DropIf("(== (call check_termcode_mouse tp (addr slen) key_name modifiers_start idx (addr modifiers)) (- 1))",
			1, "check_termcode's mouse")
	})
	if q.Err != nil {
		return q.Err
	}

	// set_termname() decides which mouse protocol the terminal speaks --
	// reading the 1006 capability, setting 'ttymouse' from it, and installing
	// the termcodes.  Forty lines, from `did_set_ttym` to the end of the block
	// that calls check_mouse_termcode(): three items.
	lines := 0
	q.InFunction("set_termname", func(q *graph.Verbs) {
		run := q.Run("set_termname()'s mouse block", "(def did_set_ttym int FALSE)", "(if _ _)", "(block (def p _ _) _ _ _)")
		if q.Err != nil {
			return
		}
		if !graph.Contains(run[2], clisp.MustPattern("(call check_mouse_termcode)")) {
			q.Die("set_termname()'s mouse block is not where this expects")
			return
		}
		t := q.Text()
		k := bytes.Index(t, []byte("    int did_set_ttym = FALSE;\n"))
		if k < 0 {
			q.Die("set_termname()'s mouse block is not where this expects")
			return
		}
		b := edit.Blank(t)
		pAt := k + bytes.Index(t[k:], []byte(`char_u *p = (char_u *)"";`)) - 40
		o := pAt + bytes.IndexByte(b[pAt:], '{')
		c := edit.Match(b, o)
		end := c + bytes.IndexByte(t[c:], '\n') + 1
		lines = bytes.Count(t[k:end], []byte{'\n'})
		q.CutRun("set_termname()'s mouse block", "(def did_set_ttym int FALSE)", "(if _ _)", "(block (def p _ _) _ _ _)")
	})
	if q.Err != nil {
		return q.Err
	}
	v.Sayf("set_termname()'s %d lines of protocol negotiation", lines)
	v.Say("the escape sequences that carried a click")

	// 32 at phase 2's front (the reform's D8), where what phases 3-11 took is
	// still there; 'mouse''s handler's went with its row (optfront, the
	// reform's D3)
	q.Cut("(call setmouse)", 32, "setmouse() calls")
	if q.Err != nil {
		return fmt.Errorf("nomouse: expected 32 setmouse() calls -- %v", q.Err)
	}
	// Every mch_setmouse() call is a bare statement too.  The count is NOT
	// hardcoded: what is asserted is that afterwards only the definition and
	// its forward declaration are left, which the sweep then takes.
	for _, b := range []string{"TRUE", "FALSE"} {
		pat := "(call mch_setmouse " + b + ")"
		q.Cut(pat, q.Count(pat), "mch_setmouse() calls")
	}
	if q.Err != nil {
		return q.Err
	}
	if left := v.Mentions("mch_setmouse"); left != 2 {
		return fmt.Errorf("nomouse: mch_setmouse has %d mentions left, expected the "+
			"definition and its declaration", left)
	}
	v.Sayf("%d setmouse() calls, every one a bare statement", 32)

	q.InFunction("draw_tabline", func(q *graph.Verbs) {
		q.Cut("(if (&& (> tabcount 1) (call mouse_has_any)) (block (call screen_putchar 'X' 0 _ attr_nosel) (= (index TabPageIdxs _) (- 999))))",
			1, "draw_tabline()'s close button")
	})
	if q.Err != nil {
		return q.Err
	}
	v.Say("the tabline stops drawing a button to click")

	// ex_behave's two 'mousemodel' lines died with :behave, retired at
	// phase 1 (exfront, the reform's D2)

	// An option row is a ROOT for reachability, so did_set_ttymouse -- and
	// through it check_mouse_termcode() -- survives the sweep, and
	// did_set_string_option() still asks whether the option being set was
	// 'mouse'.  Both read p_mouse, so --strict refuses to drop the row; and
	// the row is what keeps them reachable.  The circle is broken here, by
	// hand, which is the honest place for it.
	q.DropIf("(== varp (addr p_mouse))", 1, "the p_mouse reader")
	if q.Err != nil {
		return q.Err
	}
	// did_set_ttymouse's call to check_mouse_termcode went with its row,
	// dropped at phase 1 (optfront, the reform's D3)
	v.Say("the p_mouse reader an option row kept reachable")

	q.DropIf(`(&& (! (call option_was_set (cast _ "ttym"))) (|| (== (. (index term_props TPR_MOUSE) tpr_status) _) _))`,
		1, "the mouse-protocol reply")
	if q.Err != nil {
		return q.Err
	}
	v.Say("the terminal's mouse-protocol reply stops setting 'ttymouse'")

	q.Cut("(cast void (call opt_strings_flags p_ttym p_ttym_values (addr ttym_flags) FALSE))", 1,
		"didset_string_options' p_ttym line")
	if q.Err != nil {
		return q.Err
	}
	v.Say("didset_string_options stops reading 'ttymouse'")

	// The rows of 'mouse', 'mousemodel' and 'ttymouse', which kept their six
	// handlers reachable, are dropped at phase 1 (optfront, the reform's D3).

	// WaitForCharOrMouse() has no mouse in it: the name is left over from the
	// GUI build, where it also polled for motion events.  CHECKED rather than
	// assumed, and folded into WaitForChar(), its only caller, rather than
	// left telling a lie.
	if e.Defn("WaitForCharOrMouse") == nil || e.Defn("WaitForChar") == nil {
		return fmt.Errorf("nomouse: WaitForCharOrMouse or WaitForChar is not defined at file scope")
	}
	var inner []byte
	q.InFunction("WaitForCharOrMouse", func(q *graph.Verbs) {
		t := q.Text()
		o, c, _, _ := edit.Body(t, "WaitForCharOrMouse")
		inner = t[o+bytes.IndexByte(t[o:], '\n')+1 : bytes.LastIndexByte(t[:c], '\n')+1]
	})
	if anyMouse.Match(inner) {
		return fmt.Errorf("nomouse: WaitForCharOrMouse does mention the mouse after all")
	}
	q.InFunction("WaitForChar", func(q *graph.Verbs) {
		if q.Count("(call WaitForCharOrMouse _ _ _)") == 0 {
			q.Die("WaitForChar does not forward to WaitForCharOrMouse")
		}
	})
	q.BodyC("WaitForChar", string(inner), "WaitForChar")
	q.DeleteDefinition("WaitForCharOrMouse", "WaitForCharOrMouse")
	if q.Err != nil {
		return q.Err
	}
	v.Say("WaitForCharOrMouse has no mouse in it; folded into its one caller")

	v.Sayf("%d mouse mentions left for the sweep", len(anyMouse.FindAll(v.Text(), -1)))
	return v.Done()
}
