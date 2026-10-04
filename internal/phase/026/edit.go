package p026

// Whim phase 26 (formerly 80) -- the Ex command table, cut to the commands that exist.
// See GOAL.md.
//
// 600 rows in `enum CMD_index` and `cmdnames[]`, and 489 of them are ex_ni or
// ex_script_ni: every phase that removed a command pointed its row at the stub and
// left the row, because the row still did one job -- it held the command's NAME, and
// a name in the table decides what every abbreviation of every other name means.
// Delete `buffer` and `:b` means something else.  So the rows stayed, and with them
// the two-level prefix index generated from them.
//
// THIS PHASE DELETES THE ROWS AND KEEPS WHAT THEY WERE FOR.  The lookup used to be
// "the first row, in table order, whose name starts with what was typed", so a
// name's shortest abbreviation was implied by every row above it.  Measured on q025:
// removing the 489 rows in place would have handed 15 prefixes that used to hit a
// stub to a live command -- :n to nmap, :o to omap, :h to highlight, :sa to saveas,
// :la to later, :en to enew, :ve to verbose.  No live command would have lost an
// abbreviation or gained another's, but an error becoming a mapping listing is not
// a thing to do to anyone.
//
// So each surviving row CARRIES its shortest abbreviation, computed here from the
// 600-row table before a row is touched, in the field that held the name's length
// (whose one reader was the Vim9 whole-name check, dead since phase 25).  A typed
// word names a command when it is a prefix of the name and at least that long.
// That makes a match unique, which makes row order irrelevant, which makes the
// index pointless: cmdidxs1, cmdidxs2, command_count and E943 go, and the lookup is
// a scan of 111 rows.  Every typed word resolves exactly as it did.  That is PROVED
// in step 1 rather than argued: the old lookup, index and all, and the new one are
// both modelled over every prefix of every one of the 600 names, and they must
// agree wherever the old answer survives and find nothing wherever it did not.
// Then step 9 runs every one of those words through both BINARIES.
//
// WHAT GOES WITH THE ROWS, each proved dead by the rows going:
//
// 26 CMD_ tests of commands that no longer exist (wincmd, if/endif, try, the
// filename-escaping exceptions for grep/make/terminal, new/split/sview in
// do_exedit, the Vim9 final/horizontal/mode quirks, and the index's two start
// points CMD_Next and CMD_bang);
// the `ni` flag in do_one_cmd, which exempted stub commands from range, bang,
// count and argument checks, and can no longer be true;
// the user-command test `(int)cmdidx < 0` -- nothing assigns a negative index;
// the py3 and vim9 digit rules in find_ex_command -- no row starts with py or vim;
// seven address types that only stub rows used -- argument list, buffers, loaded
// buffers, tab pages twice, quickfix twice -- and their arms in five switches;
// :if.  It was an ex_ni row that do_one_cmd special-cased to raise if_level, and
// if_level is reset at the end of every do_cmdline while :if swallows the rest
// of its line.  No command could ever run with it raised, so `ea.skip` was
// already constantly FALSE, and its nineteen readers fold here.
//
// THE DELTA, declared.  Each removed name now gives E492 "Not an editor command"
// instead of E319 "not available in this version".  Both are errors with the same
// exit status, and the sweep cannot see the text.  Two things can see a difference,
// and both were agreed before this was written:
// :if    was silently accepted (exit 0) and is now an error (exit 1);
// `stub|cmd`  ran `cmd` after the stub's error, because a stub row with EX_TRLBAR
// split its line at the bar; an unknown name takes the whole line, so `cmd`
// no longer runs.  Probed below in both directions.
// And every removed name leaves the command sweep, which dispatches the names in
// the table: 489 rows, listed in REMOVED and required to be exactly the stub rows.
// The rows to cut are the stubs: phase 1 retires every command it declares in
// internal/phase/001/delta.md (the reform's D2), the one place the list is kept.
// The binary this phase is compared against, built from its input before a byte
// of it moves.  In the background: the edits below do not wait for it, but this
// part does before it exits, so the check finds $state/old whole.

// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3a): the text version's heads,
// literals and line surgery are acts on the nodes -- the folds told plain
// ifs from else-if arms as the heads did (HeadFold), operands dropped from
// the conditions that named them, the stub flag and the skip flag folded,
// the two definitions deleted, the dead address types' case labels deleted
// from their runs and whole arms with them, the one message respelled
// whole (RespellString) -- each counted, its report the text's.  The
// remaining-name checks ask the C view the text's own questions (history
// keeps the text version).

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// w26DeadAddr are the seven address types that only stub rows used.
var w26DeadAddr = map[string]bool{
	"ADDR_ARGUMENTS": true, "ADDR_BUFFERS": true, "ADDR_LOADED_BUFFERS": true,
	"ADDR_QUICKFIX": true, "ADDR_QUICKFIX_VALID": true, "ADDR_TABS": true,
	"ADDR_TABS_RELATIVE": true,
}

// isUser is `!IS_USER_CMDIDX(x)`, as the expansion left it.
func notUser(x string) string { return "(! (< (cast int (paren " + x + ")) 0))" }

// dropIn drops from the one node holder matches, in the scope, the operands
// pats match, each once, as one act.
func dropIn(v *graph.Verbs, holder string, pats []string, what string) {
	h := v.One(holder, what)
	if h == nil {
		return
	}
	q := graph.NewVerbs(v.Tag, v.Editor(), io.Discard)
	q.In(h, func(q *graph.Verbs) {
		for _, p := range pats {
			q.DropOperandAsText(p, 1, what)
		}
	})
	if q.Err != nil {
		v.Err = q.Err
		return
	}
	v.Say(what)
}

// Whim26 cuts the Ex command table to the commands that exist, and gives every
// surviving row the shortest abbreviation the 600-row table implied for it.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("cmdtable", e, w)

	// ---- 1-2: the table, the index, the lookup ---------------------------------
	// Cut at phase 1 (extable, the reform's D2b): the stub rows are gone, each
	// row carries its shortest abbreviation, the lookup scans them.  What is
	// left of the stubs here is their enumerators, after CMD_SIZE, which the
	// edits below take the last uses of; the enumerators go now.  Those
	// nothing else names go now; the commands that name a file are still
	// named until phases 28-32 take their last uses, and the collection
	// takes them then.
	var size *graph.Node
	for _, d := range v.Find("(CMD_SIZE)") {
		if e.IsEnumerator(d) {
			size = d
		}
	}
	if size == nil {
		return refuse(v, "enum CMD_index has no enumerators after CMD_SIZE")
	}
	text := v.Text()
	var dead []*graph.Node
	kept := 0
	for s := e.Sibling(size, 1); s != nil; s = e.Sibling(s, 1) {
		if edit.MentionCount(text, graph.EnumeratorName(s)) > 1 {
			kept++
		} else {
			dead = append(dead, s)
		}
	}
	if kept+len(dead) == 0 {
		return refuse(v, "enum CMD_index has no enumerators after CMD_SIZE")
	}
	if len(dead) > 0 {
		if _, err := e.DeleteEnumerators(dead, graph.Renumber); err != nil {
			return refuse(v, "the enumerators past CMD_SIZE -- %v", err)
		}
	}
	v.Sayf("%d enumerators past CMD_SIZE, of rows deleted at phase 1; %d still named stay", len(dead), kept)
	var live []string
	v.InTable("cmdnames", func(v *graph.Verbs) {
		for _, r := range v.Rows() {
			if b, ok := graph.Match(clispRow, r); ok {
				live = append(live, strings.Trim(b["name"].Atom, `"`))
			}
		}
	})

	// ---- 3: find_ex_command ----------------------------------------------------
	v.InFunction("find_ex_command", func(v *graph.Verbs) {
		// the flag's declaration goes once its readers have: said first, as
		// the text cut it first
		vim9 := v.One("(def vim9 int FALSE)", "the Vim9 flag nothing sets")
		if vim9 != nil {
			v.Say("the Vim9 flag nothing sets")
		}
		v.FoldNever("(&& vim9 (!= (-> eap cmdidx) CMD_SIZE))", 1,
			"the Vim9 whole-name check, the one reader of the name length")
		dropIn(v, "(&& (! vim9) (== (deref (-> eap cmd)) 'd') _)", []string{"(! vim9)"}, ":dl and :dp outside Vim9, which is everywhere")
		v.FoldNever("(&& (== (-> eap cmdidx) CMD_final) (== (- p (-> eap cmd)) 4) (! vim9))", 1,
			":final is not a command")
		if !v.Failed() {
			if err := e.Delete(vim9); err != nil {
				v.Die("the Vim9 flag nothing sets -- %v", err)
			}
		}
		v.FoldNever("(&& (== (-> eap cmdidx) CMD_horizontal) (== (- p (-> eap cmd)) 2))", 1,
			":horizontal is not a command")
		if v.Failed() {
			return
		}
		for _, n := range live {
			if strings.HasPrefix(n, "py") || strings.HasPrefix(n, "vim") {
				v.Die("a live command starts with py or vim, and its name may need a digit")
				return
			}
		}
		v.FoldNever("(&& (== (index (-> eap cmd) 0) 'p') (== (index (-> eap cmd) 1) 'y'))", 1,
			"no command left is spelled with a digit: not :py3")
		v.FoldNever(`(&& (== (deref p) '9') (== (call strncmp (cast (ptr char) (paren "vim9")) (cast (ptr char) (paren (-> eap cmd))) (paren 4)) 0))`, 1,
			"and not :vim9cmd")
		if !v.Failed() && v.Mentions("vim9") > 0 {
			v.Die("vim9 survives in find_ex_command")
		}
	})

	// ---- 4: do_one_cmd ---------------------------------------------------------
	v.InFunction("do_one_cmd", func(v *graph.Verbs) {
		ea := notUser("(. ea cmdidx)")
		v.FoldNever("(&& (== (. ea cmdidx) CMD_wincmd) (!= p nullptr))", 1, ":wincmd has no address type to find")
		v.HeadFold("always", false, ea, 3, "a command index is never a user command")
		dropIn(v, "(&& (== (deref p) '!') (== (index (. ea cmd) 1) 0151) (== (index (. ea cmd) 0) 78) _)",
			[]string{ea}, "nor in the Ni! test")
		dropIn(v, "(&& (! (& (. ea argt) (| EX_CMDWIN EX_LOCK_OK))) (!= (. ea cmdidx) CMD_checktime) _*)",
			[]string{"(!= (. ea cmdidx) CMD_checktime)", ea}, "nor in the locked-buffer exemptions, which lose :checktime")
		dropIn(v, "(&& (paren (& (. ea argt) EX_REGSTR)) (!= (deref (. ea arg)) NUL) (|| "+ea+" (!= (deref (. ea arg)) '=')) _*)",
			[]string{"(|| " + ea + " _)"}, "nor in the register argument test")
		dropIn(v, "(paren (&& "+ea+" (!= (. ea cmdidx) CMD_put) (!= (. ea cmdidx) CMD_iput)))",
			[]string{ea}, "nor in which registers may be written")
	})
	v.HeadFold("never", false, "(paren (< (cast int (paren (-> eap cmdidx))) 0))", 1, "nor in a % range over windows")

	v.InFunction("do_one_cmd", func(v *graph.Verbs) {
		v.Cut("(= ni (paren (&& "+notUser("(. ea cmdidx)")+" _)))", 1, "the stub flag, which no row can raise")
		// do_one_cmd's `int ni;` is named by nothing after the three terms
		// below, and the collection takes it.  The text told them apart by
		// what stood before and after `!ni`: first in its condition, between
		// two others, and last after getargopt()'s.
		if v.Failed() {
			return
		}
		var first, mid, last []*graph.Node
		for _, x := range v.Find("(! ni)") {
			p := e.Parent(x)
			if p == nil || !p.Is("&&") {
				continue
			}
			args := p.Args()
			switch {
			case args[0] == x:
				first = append(first, x)
			case args[len(args)-1] == x:
				last = append(last, x)
			default:
				mid = append(mid, x)
			}
		}
		for _, g := range []struct {
			xs   []*graph.Node
			n    int
			what string
		}{
			{first, 4, "range, bang, extra-argument and required-argument checks apply to every command"},
			{mid, 2, "and the range and count checks"},
			{last, 1, "and ++opt parsing"},
		} {
			if len(g.xs) != g.n {
				v.Die("%s -- occurs %d times, expected %d", g.what, len(g.xs), g.n)
				return
			}
			q := graph.NewVerbs(v.Tag, e, io.Discard)
			for _, x := range g.xs {
				q.In(e.Parent(x), func(q *graph.Verbs) { q.DropOperandAsText("(! ni)", 1, g.what) })
			}
			if q.Err != nil {
				v.Err = q.Err
				return
			}
			v.Say(g.what)
		}

		v.HeadFold("drop", false, "(== (. ea cmdidx) CMD_if)", 1, ":if and the level it raised")
		v.HeadFold("never", false, "if_level", 1, "the level is never raised")
		v.Cut("(= (. ea skip) (paren (> if_level 0)))", 1, "so nothing is skipped")
	})
	v.Cut("(= if_level 0)", 1, "the reset")
	// and the level itself, named by nothing now, goes to the collection

	v.InFunction("do_one_cmd", func(v *graph.Verbs) {
		v.HeadFold("never", false, "(== (. ea cmdidx) CMD_bang)", 1, ":! keeps no leading space")
		dropIn(v, "(|| (== (. ea cmdidx) CMD_bang) (== (. ea cmdidx) CMD_terminal) (== (. ea cmdidx) CMD_global) _*)",
			[]string{"(== (. ea cmdidx) CMD_bang)", "(== (. ea cmdidx) CMD_terminal)"},
			"the commands that take the whole line are :g and :v")
		dropIn(v, "(&& (== (deref p) '\\n') (! (& (. ea argt) EX_EXPR_ARG)))",
			[]string{"(! (& (. ea argt) EX_EXPR_ARG))"}, "and none takes an expression")
		dropIn(v, "(&& (paren (& (. ea argt) EX_COUNT)) _ (|| (! (& (. ea argt) EX_BUFNAME)) _*))",
			[]string{"(|| (! (& (. ea argt) EX_BUFNAME)) _*)"}, "a count is never a buffer name")
		v.HeadFold("never", false, "(&& (== (. ea cmdidx) CMD_try) (> (. cmdmod cmod_did_esilent) 0))", 1, ":try is not a command")
	})
	if v.Failed() {
		return v.Done()
	}

	// ---- 5: ea.skip, which only :if ever raised --------------------------------
	q := graph.NewVerbs(v.Tag, e, io.Discard)
	for _, fn := range []string{"ex_ni", "ex_script_ni"} {
		if e.Defn(fn) == nil {
			return refuse(v, "%s is not defined", fn)
		}
		q.DeleteDefinition(fn, fn)
	}
	if q.Err != nil {
		return q.Err
	}
	v.Say("ex_ni and ex_script_ni, which no row names")
	v.HeadFold("never", false, "(. ea skip)", 1, "an empty command line is never skipped")
	v.HeadFold("always", false, "(! (. ea skip))", 3, "do_one_cmd: nothing is skipped")
	v.Rewrite("(&& (! (. ea skip)) (paren ?x))", "?x", 1, "nor a range check")
	if !v.Failed() {
		var at []*graph.Node
		for _, x := range v.Find("(-> eap skip)") {
			p := e.Parent(x)
			if p != nil && p.Is("call") {
				prev, next := e.Sibling(x, -1), e.Sibling(x, 1)
				if prev != nil && graph.Matches(clispAddrType, prev) && next != nil && !next.IsList() && next.Atom == "silent" {
					at = append(at, x)
				}
			}
		}
		if len(at) != 1 {
			v.Die("nor an address -- occurs %d times, expected 1", len(at))
		} else if with, err := e.Build(at[0], "FALSE", nil); err != nil {
			v.Die("nor an address -- %v", err)
		} else if err := e.Replace(at[0], with...); err != nil {
			v.Die("nor an address -- %v", err)
		} else {
			v.Say("nor an address")
		}
	}
	v.HeadFold("never", false, "(-> eap skip)", 2, ":substitute is never skipped")
	v.HeadFold("always", false, "(! (-> eap skip))", 6, "nor its pattern, a range, or :match")
	if !v.Failed() {
		// `else if (!eap->skip) X` is `else X`
		arms := v.Query("(if _ _ ?arm:(if (! (-> eap skip)) _))", "arm")
		if len(arms) != 1 {
			v.Die("nor :substitute's previous pattern -- occurs %d times, expected 1", len(arms))
		} else if err := e.Replace(arms[0], arms[0].Kids[2]); err != nil {
			v.Die("nor :substitute's previous pattern -- %v", err)
		} else {
			v.Say("nor :substitute's previous pattern")
		}
	}
	dropIn(v, "(&& (<= i 0) (! (-> eap skip)) (. subflags do_error))", []string{"(! (-> eap skip))"}, "nor its count")
	if v.Failed() {
		return v.Done()
	}
	if skipRe.Match(v.Text()) {
		return refuse(v, "a read of skip survives")
	}

	// ---- 6: the filename and bar parsers ---------------------------------------
	cmds := []string{"CMD_bang", "CMD_grep", "CMD_grepadd", "CMD_hardcopy", "CMD_lgrep", "CMD_lgrepadd", "CMD_lmake", "CMD_make", "CMD_terminal"}
	var ne []string
	for _, c := range cmds {
		ne = append(ne, "(!= (-> eap cmdidx) "+c+")")
	}
	dropIn(v, "(&& (! (-> eap usefilter)) (! escaped) "+strings.Join(ne, " ")+")", ne,
		"expanded filenames are escaped for every command left")
	v.Rewrite("(|| (-> eap usefilter) (== (-> eap cmdidx) CMD_bang) (== (-> eap cmdidx) CMD_terminal))", "(-> eap usefilter)", 1,
		"and '!' only for a filter")
	// separate_nextcmd's :redir @" exception went with its comment test at
	// phase 1 (onecmdfront, record 81's move)
	if v.Failed() {
		return v.Done()
	}

	// ---- 7: do_exedit went with :edit at phase 1 (filefront, the reform's D4)

	// ---- 8: the address types only stub rows had -------------------------------
	v.HeadFold("never", false, "(== addr_type ADDR_TABS_RELATIVE)", 1, "no relative tab page offset")
	v.HeadFold("never", false, "(|| (== addr_type ADDR_LOADED_BUFFERS) (== addr_type ADDR_BUFFERS))", 1,
		"no buffer-number offset")
	if v.Failed() {
		return v.Done()
	}
	// What the assertion is FOR is that no SURVIVING ROW carries one of the
	// seven address types this phase removes: the table as it is.
	var bad []string
	v.InTable("cmdnames", func(v *graph.Verbs) {
		graph.Walk(v.Scope(), func(n *graph.Node) bool {
			if !n.IsList() && w26DeadAddr[n.Atom] && !edit.Contains(bad, n.Atom) {
				bad = append(bad, n.Atom)
			}
			return true
		})
	})
	if len(bad) > 0 {
		sort.Strings(bad)
		return refuse(v, "a live row has one of the address types being removed: %v", bad)
	}
	labelsGone, groupsGone, err := deadAddrLabels(e)
	if err != nil {
		return refuse(v, "%v", err)
	}
	v.RespellString(`"Cannot use EX_DFLALL with ADDR_NONE, ADDR_UNSIGNED or ADDR_QUICKFIX"`,
		`"Cannot use EX_DFLALL with ADDR_NONE or ADDR_UNSIGNED"`, 1,
		"the internal error that named the quickfix address type")
	if v.Failed() {
		return v.Done()
	}
	var left []string
	for a := range w26DeadAddr {
		if v.Count("(case "+a+")") > 0 {
			left = append(left, a)
		}
	}
	if len(left) > 0 {
		sort.Strings(left)
		return refuse(v, "case labels survive: %v", left)
	}
	v.Sayf("%d case labels for the seven address types, %d whole arms", labelsGone, groupsGone)
	return v.Done()
}

// deadAddrLabels takes the dead address types' case labels from every run
// of labels in the file: a label alone where its run keeps another, the run
// with the statements it heads, up to the next label, where every label of
// it goes -- refused where the statement before that run could fall into it
// (the text's test: a break, a goto, a return, or the start of the block).
func deadAddrLabels(e *graph.Editor) (labels, groups int, err error) {
	isLabel := func(n *graph.Node) bool { return n.Is("case") || n.Is("default") || n.Is("case-range") }
	dead := func(n *graph.Node) bool {
		return n.Is("case") && len(n.Kids) == 2 && !n.Kids[1].IsList() && w26DeadAddr[n.Kids[1].Atom]
	}
	type cut struct{ first, last *graph.Node }
	var lone []*graph.Node
	var runs []cut
	var walkErr error
	for _, f := range e.Graph().Forms {
		graph.Walk(f, func(l *graph.Node) bool {
			if walkErr != nil || !l.Is("block") {
				return walkErr == nil
			}
			ks := l.Kids
			for i := 1; i < len(ks); {
				if !isLabel(ks[i]) {
					i++
					continue
				}
				j := i
				for j < len(ks) && isLabel(ks[j]) {
					j++
				}
				k := j
				for k < len(ks) && !isLabel(ks[k]) {
					k++
				}
				var gone []*graph.Node
				for _, x := range ks[i:j] {
					if dead(x) {
						gone = append(gone, x)
					}
				}
				switch {
				case len(gone) == 0:
				case len(gone) < j-i:
					lone = append(lone, gone...)
					labels += len(gone)
				default:
					if i > 1 {
						p := ks[i-1]
						if !(p.Is("break") || p.Is("goto") || p.Is("return")) {
							walkErr = fmt.Errorf("a removed case group can be fallen into from a %s", p.Head())
							return false
						}
					}
					runs = append(runs, cut{ks[i], ks[k-1]})
					labels += j - i
					groups++
				}
				i = k
			}
			return true
		})
	}
	if walkErr != nil {
		return 0, 0, walkErr
	}
	for _, x := range lone {
		if err := e.Delete(x); err != nil {
			return 0, 0, err
		}
	}
	for _, r := range runs {
		if err := e.ReplaceRun(r.first, r.last); err != nil {
			return 0, 0, err
		}
	}
	return labels, groups, nil
}

func init() { phase.RegisterGraph("whim26", Edit) }

// refuse stops the phase with the text's refusal.
func refuse(v *graph.Verbs, format string, a ...any) error {
	v.Die(format, a...)
	return v.Done()
}
