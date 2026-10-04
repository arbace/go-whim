package p004b

// Whim phase 4b (formerly 67) -- no mouse, no spell plumbing, no write-only flags.
// See GOAL.md.
//
// Three cuts, none of which changes what the editor can do, because none of it
// could happen in the first place.
//
// THE MOUSE, WHICH CANNOT ARRIVE.  There is no 'mouse' option row, and
// setmouse(), mch_setmouse(), mouse_has() and p_mouse are all gone, so
// nothing ever asks a terminal to report mouse events.  What served them
// goes: is_mouse_key() and the term in the input loop that called it,
// reset_dragwin()/reset_held_button() with dragwin and held_button,
// mouse_row/mouse_col and old_mouse_row/old_mouse_col -- a save-and-restore
// pair that nothing else reads -- the 13 mouse rows of key_names_table, the
// [MOUSE] entry of the terminal string table, and check_termcode()'s mouse
// matching.  The 26 nv_cmds rows STAY at nv_error: that table's index is a
// permutation of its rows, so a removed row renumbers the keys after it.
//
// ONE REAL CHANGE OF BEHAVIOUR IS BURIED HERE, and it is why the pty check
// below matters.  `looks_like_mouse_start` is not mouse-specific despite
// its name: it is set for ANY two-byte `ESC [` termcode whose third byte is
// not a digit, and it defers the match so that a longer code -- a mouse one
// -- can win instead.  With no mouse code able to arrive, deferring can only
// lose, so the fold makes such a code match at once.
//
// THE SPELL PLUMBING.  spellvars_T is one field, win_line()'s spv parameter is
// already __attribute__((unused)), and win_update() declares one on the
// stack only to pass its address twice.
//
// FOURTEEN WRITE-ONLY STATICS.  gcc never warns about these -- a static that is
// assigned counts as used -- which is the blind spot that hid can_cindent
// until phase 5a and struct fields until deadfields.py.  Two of them are a
// whole function body each, so state_no_longer_safe() and its two calls go
// with was_safe.
//
// vim_ignored IS NOT ONE OF THEM, though it looks identical to the detector.
// Its five sites are `vim_ignored = ftruncate(...)`, `= dup(2)` and
// `= write(1, ...)`: it exists to swallow warn_unused_result, and removing
// it ADDS warnings.  A (void) cast does not silence that attribute in gcc.
//
// THE DELTA: none.  No key, command or option changes -- every cut is code that
// nothing could reach.  The probes check the editor still starts, edits and
// writes, and the pty check is what would catch the termcode fold going wrong.

// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3a): the text version's lines and
// literals are acts on the nodes -- an operand dropped, statements cut, a
// table's rows deleted by their name (INITROW), the termcode ifs dropped and
// folded, win_line's parameter dropped with the argument at its two calls
// (PARAM) -- each counted, its report the text's (history keeps it).

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// The mouse names in key_names_table are matched ON THE NAME, a row's
// string: the text matched the 13 one-line rows (`{TRUE, ...`) whose name
// says Mouse, Drag, Release or Wheel, and the 5 written across three lines
// (`{FALSE, ...`, the key code first) whose name says Mouse.  On the graph
// a row is a node whatever its lines, and the two kinds are its first
// element, TRUE or FALSE.
var (
	mouseNameTrue  = regexp.MustCompile(`^"\w*(?:Mouse|Drag|Release|Wheel)\w*"$`)
	mouseNameFalse = regexp.MustCompile(`^"\w*Mouse\w*"$`)
)

// writeOnlyStatics are file-scope variables that are written and never read:
// their writes go here (each the value one of vals), and the declarations,
// named by nothing after that, go to the collection.  Where the write is a
// whole `if` Body or a whole function, that goes too -- see the cases below
// the loop.
var writeOnlyStatics = []struct {
	name       string
	vals       []string
	n          int
	writesWhat string
}{
	{"did_check_timestamps", []string{"FALSE"}, 3, "the three writes to did_check_timestamps"},
	{"did_emsg_syntax", []string{"TRUE", "FALSE"}, 2, "did_emsg_syntax's two writes"},
	{"typebuf_was_empty", []string{"TRUE", "FALSE"}, 2, "typebuf_was_empty's two writes"},
	{"in_mch_delay", []string{"TRUE", "FALSE"}, 2, "in_mch_delay's two writes"},
}

// cutWrites cuts the n statements `name = V;`, V one of vals, as one act.
func cutWrites(v *graph.Verbs, name string, vals []string, n int, what string) {
	got := 0
	for _, val := range vals {
		got += v.Count("(= " + name + " " + val + ")")
	}
	if got != n {
		v.Die("%s -- %d matches, expected %d", what, got, n)
		return
	}
	v.Cut("(= "+name+" _)", n, what)
}

// mouseRows deletes the rows of key_names_table whose first element is
// flag and whose name re matches, n of them, as one act; it says them.
func mouseRows(v *graph.Verbs, flag string, re *regexp.Regexp, n int, what string) {
	e := v.Editor()
	p := "(init " + flag + " _ (init (cast (ptr char_u) (paren ?name)) _) _)"
	var rows []*graph.Node
	var names []string
	pat := clisp.MustPattern(p)
	for _, r := range v.Rows() {
		if b, ok := graph.Match(pat, r); ok && re.MatchString(b["name"].Atom) {
			rows = append(rows, r)
			names = append(names, strings.Trim(b["name"].Atom, `"`))
		}
	}
	if v.Failed() {
		return
	}
	if len(rows) != n {
		v.Die("key_names_table -- %d %s mouse names, expected %d: %s", len(rows), flag, n, strings.Join(names, " "))
		return
	}
	if _, err := e.DeleteRows(v.Scope(), rows, graph.RowIndex{}); err != nil {
		v.Die("%s -- %v", what, err)
		return
	}
	v.Say(what + strings.Join(names, " "))
}

// Whim4b takes the mouse -- every key name, the deferred-match machinery in
// check_termcode and the statics that tracked a pointer -- the spell plumbing
// win_line still carried, and eleven file-scope variables written and never
// read.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("nomouse", e, w)

	v.DropOperand("(paren (&& (call is_mouse_key n) (!= n (paren (- (+ (paren KS_EXTRA) (<< (cast int (paren KE_LEFTMOUSE)) 8)))))))", 1,
		"the input loop asking whether a key is a mouse key")
	v.Cut("(call reset_dragwin)", 2, "the two calls that forgot the dragged window")
	v.Cut("(call reset_held_button)", 1, "the call that forgot the held button")
	// mouse_row/col and old_mouse_row/col are a closed loop: saved here,
	// restored there, read by nothing else.
	v.Cut("(= mouse_row old_mouse_row)", 1, "restoring the mouse row")
	v.Cut("(= mouse_col old_mouse_col)", 1, "restoring the mouse column")
	v.Cut("(= old_mouse_row mouse_row)", 1, "saving the mouse row")
	v.Cut("(= old_mouse_col mouse_col)", 1, "saving the mouse column")

	v.InTable("key_names_table", func(v *graph.Verbs) {
		mouseRows(v, "TRUE", mouseNameTrue, 13, "the mouse key names: ")
		mouseRows(v, "FALSE", mouseNameFalse, 5, "the terminal-specific mouse names: ")
	})
	v.InTable("builtin_debug", func(v *graph.Verbs) {
		v.DeleteRows(`(init (paren (- (+ (paren KS_MOUSE) (<< (cast int (paren (paren 'X'))) 8)))) "[MOUSE]")`, 1,
			graph.RowIndex{}, "the [MOUSE] entry of the terminal string table")
	})

	v.InFunction("check_termcode", func(v *graph.Verbs) {
		// The whole `slen == 2 && ESC [` block existed to set that flag, and its
		// only other arm counted the semicolons of a DEC mouse report.
		v.DropIf("(&& (== slen 2) (> len 2) (== (index (. (index termcodes idx) code) 0) ESC) (== (index (. (index termcodes idx) code) 1) '['))", 1,
			"deferring an ESC [ code in case a mouse code is longer")
		v.FoldNever("looks_like_mouse_start", 1, "a deferred match winning over a real one")
		v.DropOperand("(< mouse_index_found 0)", 1, "the modifier scan waiting for a deferred mouse match")
		v.FoldNever("(&& (== idx tc_len) (>= mouse_index_found 0))", 1, "falling back to the deferred mouse match")
		v.Cut("(if (|| (== (index key_name 0) KS_MOUSE) (== (index key_name 0) KS_SGR_MOUSE) (== (index key_name 0) KS_SGR_MOUSE_RELEASE)) (block))", 1,
			"a mouse report being handled by an empty block")
	})

	// the spell plumbing: the parameter, and the argument at both calls
	if !v.Failed() {
		calls := len(v.Find("(call win_line wp lnum srow (-> wp w_height) _ (addr spv))"))
		v.Expect(calls == 2, "win_line -- %d calls passing &spv, expected 2", calls)
	}
	v.DropParam("win_line", "spv", "win_line's unused spell parameter")
	if !v.Failed() {
		v.Say("the first win_line call")
		v.Say("the second win_line call")
	}

	// the write-only statics
	for _, s := range writeOnlyStatics {
		cutWrites(v, s.name, s.vals, s.n, s.writesWhat)
	}
	v.Cut("(post++ frame_locked)", 1, "the lock it took")
	v.Cut("(post-- frame_locked)", 1, "the lock it released")
	v.Cut("(= swap_exists_did_quit TRUE)", 1, "its one write")
	v.Cut("(= did_swapwrite_msg FALSE)", 1, "its one write")
	v.Cut("(= autocmd_nested (-> ac nested))", 1, "its one write")
	v.Cut("(= oldtitle_outdated TRUE)", 1, "its one write")
	v.Cut("(= deadly_signal sigarg)", 1, "the signal number it recorded")
	// mr_patternlen's two writes are a whole if/else, so the test goes with them.
	v.Cut("(if (== mr_pattern nullptr) (block (= mr_patternlen 0)) (block (= mr_patternlen patlen)))", 1,
		"mr_patternlen's if/else")
	// was_safe is a whole function Body, and that function has two callers.
	if !v.Failed() {
		n := v.Count(`(call state_no_longer_safe "ins_typebuf()")`) + v.Count(`(call state_no_longer_safe "key typed")`)
		v.Expect(n == 2, "the two calls that declared the state unsafe -- %d matches, expected 2", n)
	}
	v.Cut("(call state_no_longer_safe _)", 2, "the two calls that declared the state unsafe")
	v.DeleteDefinition("state_no_longer_safe", "state_no_longer_safe, whose body was one write")
	if !v.Failed() {
		n := v.Count("(= was_safe is_safe)") + v.Count("(= was_safe FALSE)")
		v.Expect(n == 2, "its remaining writes -- %d matches, expected 2", n)
	}
	v.Cut("(= was_safe _)", 2, "its remaining writes")
	return v.Done()
}

func init() { phase.RegisterGraph("whim4b", Edit) }
