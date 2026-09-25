package p080

// Whim phase 80 -- the Ex command table, cut to the commands that exist.
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
// name's shortest abbreviation was implied by every row above it.  Measured on q79:
// removing the 489 rows in place would have handed 15 prefixes that used to hit a
// stub to a live command -- :n to nmap, :o to omap, :h to highlight, :sa to saveas,
// :la to later, :en to enew, :ve to verbose.  No live command would have lost an
// abbreviation or gained another's, but an error becoming a mapping listing is not
// a thing to do to anyone.
//
// So each surviving row CARRIES its shortest abbreviation, computed here from the
// 600-row table before a row is touched, in the field that held the name's length
// (whose one reader was the Vim9 whole-name check, dead since phase 79).  A typed
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
// The rows to cut are the commands this phase declares in internal/phase/080/delta.md: the
// declared delta and the cut are one list, kept in one place.
// The binary this phase is compared against, built from its input before a byte
// of it moves.  In the background: the edits below do not wait for it, but this
// part does before it exits, so the check finds $state/old whole.

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

// w80OldChars is the set of one-character command names q79 still recognises.
const w80OldChars = "@*!=><&~#}"

// w80DeadAddr are the seven address types that only stub rows used.
var w80DeadAddr = map[string]bool{
	"ADDR_ARGUMENTS": true, "ADDR_BUFFERS": true, "ADDR_LOADED_BUFFERS": true,
	"ADDR_QUICKFIX": true, "ADDR_QUICKFIX_VALID": true, "ADDR_TABS": true,
	"ADDR_TABS_RELATIVE": true,
}

// Whim80 cuts the Ex command table to the commands that exist, and gives every
// surviving row the shortest abbreviation the 600-row table implied for it.
func Edit(text []byte, w io.Writer, args []string) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("  cmdtable     usage: edit whim80 <file> <words-out>")
	}
	wordsOut := args[0]
	e := edit.New("cmdtable", text, w)
	t := string(text)

	// ---- 1: the table, the index, and the proof --------------------------------
	mt := tableRe.FindStringSubmatchIndex(t)
	if mt == nil {
		return nil, e.Refused("cmdnames[] definition not found")
	}
	tabStart, tabEnd := mt[2], mt[3]
	rowName := map[string]string{}
	rowHandler := map[string]string{}
	var rowOrder []string
	for _, line := range strings.Split(t[tabStart:tabEnd], "\n") {
		if line == "" {
			continue
		}
		r := rowRe.FindStringSubmatch(line)
		if r == nil || r[2] != r[3] {
			z := line
			if len(z) > 90 {
				z = z[:90]
			}
			return nil, e.Refused("a cmdnames[] row does not have the expected shape: %s", edit.PyRepr(z))
		}
		rowName[r[1]] = r[2]
		rowHandler[r[1]] = r[4]
		rowOrder = append(rowOrder, r[1])
	}

	me := enumRe.FindStringSubmatchIndex(t)
	if me == nil {
		return nil, e.Refused("enum CMD_index not found")
	}
	enumStart, enumEnd := me[2], me[3]
	var ids []string
	for _, m := range idRe.FindAllStringSubmatch(t[enumStart:enumEnd], -1) {
		ids = append(ids, m[1])
	}
	if len(ids) != 600 || !sameSet(ids, rowOrder) {
		return nil, e.Refused("enum CMD_index has %d names, and they are not the %d rows", len(ids), len(rowName))
	}
	if strings.Join(ids, ",") != strings.Join(rowOrder, ",") {
		return nil, e.Refused("the rows are not written in enumerator order")
	}

	// THE LOOKUP SCANS BY INDEX, so the model uses enumerator order.
	names := make([]string, len(ids))
	handler := map[string]string{}
	for i, id := range ids {
		names[i] = rowName[id]
		handler[rowName[id]] = rowHandler[id]
	}
	var dead, live []string
	for _, n := range names {
		if handler[n] == "ex_ni" || handler[n] == "ex_script_ni" {
			dead = append(dead, n)
		} else {
			live = append(live, n)
		}
	}
	removed := strings.Fields(os.Getenv("REMOVED"))
	if !sameSet(dead, removed) {
		return nil, e.Refused("the stub rows are not REMOVED: extra %v, missing %v",
			minus(dead, removed), minus(removed, dead))
	}
	e.Say(fmt.Sprintf("confirmed: %d rows, %d of them stubs -- exactly REMOVED -- and %d live",
		len(names), len(dead), len(live)))

	// The old index, read Out of the file rather than regenerated.
	m1 := idx1Re.FindStringSubmatch(t)
	m2 := idx2Re.FindStringSubmatch(t)
	mc := countRe.FindStringSubmatch(t)
	if m1 == nil || m2 == nil || mc == nil {
		return nil, e.Refused("the ex_cmdidxs block is not where it was")
	}
	idx1, idx2 := w80Ints(m1[1]), w80Ints(m2[1])
	if len(idx1) != 26 || len(idx2) != 676 || mc[1] != "600" {
		return nil, e.Refused("the ex_cmdidxs block has an unexpected shape")
	}

	mch := charsRe.FindStringSubmatch(t)
	if mch == nil || mch[1] != w80OldChars {
		return nil, e.Refused("the one-character command set is not %s", edit.PyRepr(w80OldChars))
	}
	liveSet := map[string]bool{}
	for _, n := range live {
		liveSet[n] = true
	}
	newChars := ""
	for _, c := range w80OldChars {
		if liveSet[string(c)] {
			newChars += string(c)
		}
	}

	startNext, startBang := edit.IndexOf(ids, "Next"), edit.IndexOf(ids, "bang")
	oldLookup := func(wd string) string {
		var start int
		c0 := wd[0]
		switch {
		case isAlpha(c0):
			for i := 0; i < len(wd); i++ {
				if !isAlpha(wd[i]) && !isDigit(wd[i]) {
					return ""
				}
			}
			if isLower(c0) {
				start = idx1[int(c0)-97]
				if len(wd) > 1 && isLower(wd[1]) {
					start += idx2[(int(c0)-97)*26+int(wd[1])-97]
				}
			} else {
				start = startNext
			}
		case strings.IndexByte(w80OldChars, c0) >= 0:
			if len(wd) != 1 {
				return ""
			}
			start = startBang
		default:
			return ""
		}
		for _, n := range names[start:] {
			if strings.HasPrefix(n, wd) {
				return n
			}
		}
		return ""
	}

	minlen := map[string]int{}
	for _, n := range live {
		if oldLookup(n) != n {
			return nil, e.Refused("%s does not resolve to itself in the old table", edit.PyRepr(n))
		}
		for i := 1; i <= len(n); i++ {
			if oldLookup(n[:i]) == n {
				minlen[n] = i
				break
			}
		}
	}

	newLookup := func(wd string) string {
		if !isAlpha(wd[0]) {
			if strings.IndexByte(newChars, wd[0]) < 0 || len(wd) != 1 {
				return ""
			}
		} else {
			wd = wordRe.FindString(wd)
		}
		for _, n := range live {
			if len(wd) >= minlen[n] && strings.HasPrefix(n, wd) {
				return n
			}
		}
		return ""
	}

	wordset := map[string]bool{}
	for _, n := range names {
		for i := 1; i <= len(n); i++ {
			wordset[n[:i]] = true
		}
	}
	for _, c := range w80OldChars + "{+-" {
		wordset[string(c)] = true
	}
	words := make([]string, 0, len(wordset))
	for k := range wordset {
		words = append(words, k)
	}
	sort.Strings(words)

	var moved []string
	for _, wd := range words {
		o := oldLookup(wd)
		want := ""
		if _, ok := minlen[o]; ok {
			want = o
		}
		if got := newLookup(wd); got != want {
			moved = append(moved, fmt.Sprintf("(%s, %s, %s)", edit.PyRepr(wd), pyOrNone(o), pyOrNone(got)))
		}
	}
	if len(moved) > 0 {
		return nil, e.Refused("the new lookup disagrees with the old one on %d words: [%s]",
			len(moved), strings.Join(edit.First(moved, 8), ", "))
	}
	var unique []string
	for _, wd := range words {
		k := 0
		for _, n := range live {
			if len(wd) >= minlen[n] && strings.HasPrefix(n, wd) {
				k++
			}
		}
		if k > 1 {
			unique = append(unique, edit.PyRepr(wd))
		}
	}
	if len(unique) > 0 {
		return nil, e.Refused("a word matches more than one row: [%s]", strings.Join(edit.First(unique, 8), ", "))
	}
	e.Say(fmt.Sprintf("proved: all %d prefixes of the 600 names resolve as before, each to at most one row", len(words)))

	// What step 9 dispatches, with what the old table made of each word.
	var fh strings.Builder
	for _, wd := range words {
		o := oldLookup(wd)
		if o == "" {
			o = "-"
		}
		fmt.Fprintf(&fh, w80lit10, wd, o)
	}
	if err := os.WriteFile(wordsOut, []byte(fh.String()), 0o644); err != nil {
		return nil, e.Refused("%v", err)
	}

	// ---- 2: rewrite the two lists ----------------------------------------------
	var Body []string
	for _, line := range strings.Split(t[tabStart:tabEnd], "\n") {
		if line == "" {
			Body = append(Body, "")
			continue
		}
		r := rowRe.FindStringSubmatch(line)
		name := r[2]
		if n, ok := minlen[name]; ok {
			Body = append(Body, strings.Replace(line,
				fmt.Sprintf("sizeof(%q) - 1", name), strconv.Itoa(n), 1))
		}
	}
	tabBody := strings.Join(Body, "\n")
	var enumBody []string
	for _, line := range strings.Split(t[enumStart:enumEnd], "\n") {
		r := idRe.FindStringSubmatch(line)
		if r != nil {
			if _, ok := minlen[rowName[r[1]]]; !ok {
				continue
			}
		}
		enumBody = append(enumBody, line)
	}
	enumText := strings.Join(enumBody, "\n")
	if !(enumEnd < tabStart) {
		return nil, e.Refused("the enum is not above the table")
	}
	e.Set([]byte(t[:enumStart] + enumText + t[enumEnd:tabStart] + tabBody + t[tabEnd:]))
	e.Say(fmt.Sprintf("%d rows and %d enumerators kept, each row with its shortest abbreviation",
		len(minlen), len(minlen)))

	e.Literal(w80lit3, w80lit4, 1, "the row field that held the name length holds the shortest abbreviation")
	if loc := bannerRe.FindIndex(e.Text()); loc == nil {
		return nil, e.Refused("the ex_cmdidxs block is gone")
	} else {
		e.Set(append(append([]byte{}, e.Text()[:loc[0]]...), e.Text()[loc[1]:]...))
	}
	e.Say("the prefix index and its count")
	// Three notes this used to reword (on the two lists, in mch_dirname and in
	// add_time) were comments, and the canonical form has none.

	// ---- 3: find_ex_command ----------------------------------------------------
	e.Cut(edit.Line("int vim9 = FALSE;"), 1, "the Vim9 flag nothing sets")
	e.FoldNever(edit.Head("if (vim9 && eap->cmdidx != CMD_SIZE)"), 1,
		"the Vim9 whole-name check, the one reader of the name length")
	e.Literal("if (!vim9 && *eap->cmd == 'd' && ", "if (*eap->cmd == 'd' && ", 1,
		":dl and :dp outside Vim9, which is everywhere")
	e.FoldNever(edit.Head("if (eap->cmdidx == CMD_final && p - eap->cmd == 4 && !vim9)"), 1,
		":final is not a command")
	e.FoldNever(edit.Head("if (eap->cmdidx == CMD_horizontal && p - eap->cmd == 2)"), 1,
		":horizontal is not a command")
	if e.Failed() {
		return e.Done()
	}
	for _, n := range live {
		if strings.HasPrefix(n, "py") || strings.HasPrefix(n, "vim") {
			return nil, e.Refused("a live command starts with py or vim, and its name may need a digit")
		}
	}
	e.FoldNever(edit.Head("if (eap->cmd[0] == 'p' && eap->cmd[1] == 'y')"), 1,
		"no command left is spelled with a digit: not :py3")
	e.FoldNever(edit.Head(`if (*p == '9' && strncmp((char *)("vim9"), (char *)(eap->cmd), (4)) == 0)`), 1,
		"and not :vim9cmd")
	if e.Failed() {
		return e.Done()
	}
	t = string(e.Text())
	fx := strings.Index(t, w80lit11)
	if fx >= 0 && vim9Re.MatchString(t[fx:min80(fx+6000, len(t))]) {
		return nil, e.Refused("vim9 survives in find_ex_command")
	}

	if k := strings.Count(t, w80Head); k != 1 {
		return nil, e.Refused("the index lookup head occurs %d times", k)
	}
	a := strings.Index(t, w80Head)
	loop := strings.Index(t[a:], "        for (; (int)eap->cmdidx < (int)CMD_SIZE;")
	if loop < 0 {
		return nil, e.Refused("the lookup span does not contain its for loop")
	}
	loop += a
	b := edit.Blank([]byte(t))
	lb := strings.Index(string(b[loop:]), "{")
	if lb < 0 {
		return nil, e.Refused("the lookup span has no body")
	}
	lb += loop
	z := strings.Index(t[edit.Match(b, lb):], "\n") + edit.Match(b, lb) + 1
	oldSpan := t[a:z]
	for _, need := range []string{"cmdidxs1", "cmdidxs2", "command_count", "CMD_Next", "CMD_bang", "strncmp"} {
		if !strings.Contains(oldSpan, need) {
			return nil, e.Refused("the lookup span does not contain %s -- it is not the block it was", need)
		}
	}
	// Thirty lines, re-measured on the canonical text: the span's ends are the
	// head and its for loop's matching brace, and the six words above say it is
	// the block it was.  The count is the tree's, and the canonical form writes
	// the same block without the three blank lines the residue had.
	if k := strings.Count(oldSpan, "\n"); k != 30 {
		return nil, e.Refused("the lookup span is %d lines, expected 30", k)
	}
	e.Set([]byte(t[:a] + w80lit9 + t[z:]))
	e.Say(fmt.Sprintf("the lookup: a prefix at least as long as the row says, over %d rows", len(minlen)))
	e.Literal(fmt.Sprintf(`vim_strchr((char_u *)"%s", *p)`, w80OldChars),
		fmt.Sprintf(`vim_strchr((char_u *)"%s", *p)`, newChars), 1,
		fmt.Sprintf("the one-character commands that exist: %s", newChars))

	// ---- 4: do_one_cmd ---------------------------------------------------------
	e.FoldNever(edit.Head("if (ea.cmdidx == CMD_wincmd && p != NULL)"), 1, ":wincmd has no address type to find")
	e.FoldAlways(edit.Head("if (!((int)(ea.cmdidx) < 0))"), 3, "a command index is never a user command")
	e.Literal("ea.cmd[0] == 78 && !((int)(ea.cmdidx) < 0))", "ea.cmd[0] == 78)", 1, "nor in the Ni! test")
	e.Literal("ea.cmdidx != CMD_checktime && ea.cmdidx != CMD_edit && ea.cmdidx != CMD_file && !((int)(ea.cmdidx) < 0) && curbuf_locked()",
		"ea.cmdidx != CMD_edit && ea.cmdidx != CMD_file && curbuf_locked()", 1,
		"nor in the locked-buffer exemptions, which lose :checktime")
	e.Literal("*ea.arg != NUL && (!((int)(ea.cmdidx) < 0) || *ea.arg != '=') && !((ea.argt",
		"*ea.arg != NUL && !((ea.argt", 1, "nor in the register argument test")
	e.Literal("(!((int)(ea.cmdidx) < 0) && ea.cmdidx != CMD_put && ea.cmdidx != CMD_iput)",
		"(ea.cmdidx != CMD_put && ea.cmdidx != CMD_iput)", 1, "nor in which registers may be written")
	e.FoldNever(edit.Head("if (((int)(eap->cmdidx) < 0))"), 1, "nor in a % range over windows")

	e.Cut(edit.Line("ni = (!((int)(ea.cmdidx) < 0) && (cmdnames[ea.cmdidx].cmd_func == ex_ni || cmdnames[ea.cmdidx].cmd_func == ex_script_ni));"),
		1, "the stub flag, which no row can raise")
	// do_one_cmd's `int ni;` is named by nothing after the three terms below,
	// and the sweep takes it.
	e.Literal("(!ni && ", "(", 4, "range, bang, extra-argument and required-argument checks apply to every command")
	e.Literal("&& !ni && ", "&& ", 2, "and the range and count checks")
	e.Literal("getargopt(&ea) == FAIL && !ni)", "getargopt(&ea) == FAIL)", 1, "and ++opt parsing")

	e.DropIf(`(?m)^    if \(ea\.cmdidx == CMD_if\)$`, 1, ":if and the level it raised")
	e.FoldNever(`(?m)^    if \(if_level\)$`, 1, "the level is never raised")
	e.Cut(edit.Line("ea.skip = (if_level > 0);"), 1, "so nothing is skipped")
	e.Cut(edit.Line("if_level = 0;"), 1, "the reset")
	// and the level itself, named by nothing now, goes to the sweep

	e.FoldNever(edit.Head("if (ea.cmdidx == CMD_bang)"), 1, ":! keeps no leading space")
	e.Literal("else if (ea.cmdidx == CMD_bang || ea.cmdidx == CMD_terminal || ea.cmdidx == CMD_global",
		"else if (ea.cmdidx == CMD_global", 1, "the commands that take the whole line are :g and :v")
	e.Literal("else if (*p == '\\n' && !(ea.argt & EX_EXPR_ARG))", "else if (*p == '\\n')", 1,
		"and none takes an expression")
	e.Literal(" && (!(ea.argt & EX_BUFNAME) || *(p = skipdigits(ea.arg + 1)) == NUL || ((*p) == ' ' || (*p) == '\\t')))",
		")", 1, "a count is never a buffer name")
	e.FoldNever(edit.Head("if (ea.cmdidx == CMD_try && cmdmod.cmod_did_esilent > 0)"), 1, ":try is not a command")
	if e.Failed() {
		return e.Done()
	}

	// ---- 5: ea.skip, which only :if ever raised --------------------------------
	for _, fn := range []string{"ex_ni", "ex_script_ni"} {
		cur := e.Text()
		a, z, ok := edit.FindDefinition(cur, edit.Blank(cur), fn)
		if !ok {
			return nil, e.Refused("%s is not defined", fn)
		}
		e.Set(append(append([]byte{}, cur[:a]...), cur[z:]...))
	}
	e.Say("ex_ni and ex_script_ni, which no row names")
	e.FoldNever(edit.Head("if (ea.skip)"), 1, "an empty command line is never skipped")
	e.FoldAlways(edit.Head("if (!ea.skip)"), 3, "do_one_cmd: nothing is skipped")
	e.Literal("if (!ea.skip && (ea.argt & EX_RANGE))", "if (ea.argt & EX_RANGE)", 1, "nor a range check")
	e.Literal("eap->addr_type, eap->skip, silent,", "eap->addr_type, FALSE, silent,", 1, "nor an address")
	e.FoldNever(edit.Head("if (eap->skip)"), 2, ":substitute is never skipped")
	e.FoldAlways(edit.Head("if (!eap->skip)"), 6, "nor its pattern, a range, or :match")
	e.Literal(w80lit6, w80lit7, 1, "nor :substitute's previous pattern")
	e.Literal("i <= 0 && !eap->skip && subflags.do_error", "i <= 0 && subflags.do_error", 1, "nor its count")
	if e.Failed() {
		return e.Done()
	}
	if skipRe.Match(e.Text()) {
		return nil, e.Refused("a read of skip survives")
	}

	// ---- 6: the filename and bar parsers ---------------------------------------
	e.Literal(" && eap->cmdidx != CMD_bang && eap->cmdidx != CMD_grep && eap->cmdidx != CMD_grepadd && eap->cmdidx != CMD_hardcopy && eap->cmdidx != CMD_lgrep && eap->cmdidx != CMD_lgrepadd && eap->cmdidx != CMD_lmake && eap->cmdidx != CMD_make && eap->cmdidx != CMD_terminal)",
		")", 1, "expanded filenames are escaped for every command left")
	e.Literal("(eap->usefilter || eap->cmdidx == CMD_bang || eap->cmdidx == CMD_terminal) &&",
		"eap->usefilter &&", 1, "and '!' only for a filter")
	e.Literal(" && (eap->cmdidx != CMD_redir || p != eap->arg + 1 || p[-1] != '@'))", ")", 1,
		"a double quote after :redir @ is a comment like any other")
	if e.Failed() {
		return e.Done()
	}

	// ---- 7: do_exedit ----------------------------------------------------------
	for _, c := range []string{"ERROR_IF_POPUP_WINDOW", "ERROR_IF_TERM_POPUP_WINDOW"} {
		if !regexp.MustCompile(`(?m)^enum \{ ` + c + ` = 0 \};$`).Match(e.Text()) {
			return nil, e.Refused("%s is not the constant 0", c)
		}
	}
	e.FoldNever(edit.Head("if ((eap->cmdidx != CMD_pedit && ERROR_IF_POPUP_WINDOW) || ERROR_IF_TERM_POPUP_WINDOW)"), 1,
		"no popup window refuses an edit")
	e.FoldNever(edit.Head("if ((eap->cmdidx == CMD_new || eap->cmdidx == CMD_vnew) && *eap->arg == NUL)"), 1,
		":new and :vnew are not commands")
	e.FoldAlwaysElse(edit.Head("if ((eap->cmdidx != CMD_split && eap->cmdidx != CMD_vsplit) || *eap->arg != NUL)"), 1,
		"and neither are :split and :vsplit, so every edit edits")
	e.Literal("if (eap->cmdidx == CMD_view || eap->cmdidx == CMD_sview)", "if (eap->cmdidx == CMD_view)", 1,
		":view is read-only and :sview is gone")

	// ---- 8: the address types only stub rows had -------------------------------
	e.FoldNever(edit.Head("if (addr_type == ADDR_TABS_RELATIVE)"), 1, "no relative tab page offset")
	e.FoldNever(edit.Head("if (addr_type == ADDR_LOADED_BUFFERS || addr_type == ADDR_BUFFERS)"), 1,
		"no buffer-number offset")
	if e.Failed() {
		return e.Done()
	}
	// THE TABLE IS FOUND AGAIN, on the text as it now stands.  The Python held
	// tab_start from step 1 and every act since had moved the text under it, so
	// what it scanned was whatever that stale offset happened to land on -- on
	// the canonical text it lands below the table, on rows of another kind that
	// still say ADDR_BUFFERS, and the assertion fires for a reason that has
	// nothing to do with cmdnames[].  What the assertion is FOR is that no
	// SURVIVING ROW carries one of the seven address types this phase removes,
	// and that is what it asks now: the table as it is, found by its own header.
	t = string(e.Text())
	mt2 := tableRe.FindStringSubmatchIndex(t)
	if mt2 == nil {
		return nil, e.Refused("cmdnames[] is gone before its address types were checked")
	}
	var bad []string
	for _, a := range regexp.MustCompile(`ADDR_\w+`).FindAllString(t[mt2[2]:mt2[3]], -1) {
		if w80DeadAddr[a] && !edit.Contains(bad, a) {
			bad = append(bad, a)
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return nil, e.Refused("a live row has one of the address types being removed: %v", bad)
	}
	L := strings.Split(t, "\n")
	var Out []string
	labelsGone, groupsGone := 0, 0
	for i := 0; i < len(L); {
		mm := labelRe.FindStringSubmatch(L[i])
		if mm == nil {
			Out = append(Out, L[i])
			i++
			continue
		}
		ind := mm[1]
		j := i
		var labels []string
		for j < len(L) {
			x := labelRe.FindStringSubmatch(L[j])
			if x == nil || x[1] != ind {
				break
			}
			labels = append(labels, L[j])
			j++
		}
		k := j
		for k < len(L) && (L[k] == "" || (strings.HasPrefix(L[k], ind+" ") && !labelRe.MatchString(L[k]))) {
			k++
		}
		var keep []string
		for _, x := range labels {
			n := "default"
			if strings.TrimSpace(x) != "default:" {
				n = caseRe.FindStringSubmatch(x)[1]
			}
			if !w80DeadAddr[n] {
				keep = append(keep, x)
			}
		}
		switch {
		case len(keep) == len(labels):
			Out = append(Out, L[i:k]...)
		case len(keep) > 0:
			Out = append(Out, keep...)
			Out = append(Out, L[j:k]...)
			labelsGone += len(labels) - len(keep)
		default:
			prev := ""
			for x := len(Out) - 1; x >= 0; x-- {
				if strings.TrimSpace(Out[x]) != "" {
					prev = Out[x]
					break
				}
			}
			if !fallRe.MatchString(prev) {
				return nil, e.Refused("a removed case group can be fallen into from %s",
					edit.PyRepr(strings.TrimSpace(prev)))
			}
			labelsGone += len(labels)
			groupsGone++
		}
		i = k
	}
	e.Set([]byte(strings.Join(Out, "\n")))
	e.Literal(`"Cannot use EX_DFLALL with ADDR_NONE, ADDR_UNSIGNED or ADDR_QUICKFIX"`,
		`"Cannot use EX_DFLALL with ADDR_NONE or ADDR_UNSIGNED"`, 1,
		"the internal error that named the quickfix address type")
	if e.Failed() {
		return e.Done()
	}
	var left []string
	for a := range w80DeadAddr {
		if regexp.MustCompile(`\bcase ` + a + `:`).Match(e.Text()) {
			left = append(left, a)
		}
	}
	if len(left) > 0 {
		sort.Strings(left)
		return nil, e.Refused("case labels survive: %v", left)
	}
	e.Say(fmt.Sprintf("%d case labels for the seven address types, %d whole arms", labelsGone, groupsGone))
	return e.Done()
}

func w80Ints(s string) []int {
	var Out []int
	for _, x := range numRe.FindAllString(s, -1) {
		n, _ := strconv.Atoi(x)
		Out = append(Out, n)
	}
	return Out
}

func isAlpha(c byte) bool { return isLower(c) || (c >= 'A' && c <= 'Z') }
func isLower(c byte) bool { return c >= 'a' && c <= 'z' }
func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func min80(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func sameSet(a, b []string) bool {
	x := append([]string{}, a...)
	y := append([]string{}, b...)
	sort.Strings(x)
	sort.Strings(y)
	return strings.Join(x, "\x00") == strings.Join(y, "\x00")
}

func minus(a, b []string) []string {
	in := map[string]bool{}
	for _, v := range b {
		in[v] = true
	}
	var Out []string
	for _, v := range a {
		if !in[v] && !edit.Contains(Out, v) {
			Out = append(Out, v)
		}
	}
	sort.Strings(Out)
	return Out
}

func pyOrNone(s string) string {
	if s == "" {
		return "None"
	}
	return edit.PyRepr(s)
}

func init() { phase.RegisterArgs("whim80", Edit) }
