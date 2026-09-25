package p093

// Whim phase 93 -- the buffer has no name.  See GOAL.md.
//
// Phases 89, 90 and 91 took every way to ASK for a file and phase 92 took the machinery
// that read one.  What is left of the filesystem in this editor is a NAME: three
// char_u* fields on every buffer -- `b_ffname`, `b_sfname`, `b_fname` -- and the one
// command that could still set them, `:file`.  This phase takes the command, stops
// `buflist_new()` naming the buffer it makes, and folds the sixteen places that ask
// what the name is.  After it the three fields are written nowhere, the sweep takes
// them, and `[No Name]` is no longer one of the answers the editor can give but the
// only one.
//
// IT IS ALSO WHERE THE CORE STOPS ASKING THE FILESYSTEM QUESTIONS OF ITS OWN ACCORD.
// Three libc symbols go, and each for the reason of one part below:
//
// stat      `mch_getperm()`, reached from `find_file_in_path()`, which part G
// makes unreachable: CTRL-F and CTRL-P extract a word from the buffer
// and nothing looks for it on a disk.
// getcwd    `mch_dirname()`, whose three callers were `shorten_fname()`,
// `shorten_fnames()` and `mch_FullName()`.  Part F takes the second and
// the sweep the other two.
// strerror  `mch_dirname()`'s error arm, and nothing else's.
//
// SEVEN PARTS, A to G, and every removal that is not one of them is the sweep's
// (GOALS.md core rule 1).  Sixty functions go and this file names not one of them.
//
// A  `:file` goes: the enumerator, the cmdnames[] row, and BOTH of do_one_cmd's
// CMD_file tests -- the `curbuf_locked()` conjunct phase 91 deliberately kept,
// and the second test below it.  All in one edit with the enumerator, or the
// text does not compile.  `ex_file` -> `rename_buffer` -> `setfname` then die
// by the sweep; `fileinfo()` SURVIVES, having three other callers.
// B  `buflist_new()` never names: its one call site already passes NULL, NULL, so
// both parameters go with the `fname_expand`/`stat`/`buflist_findname_stat`
// prologue, the `if (ffname != NULL)` assignment, the failure arm's frees and
// the `st.st_dev` block.
// C  the sixteen folds, one per site, each with the constant it takes written out
// here.  `== NULL` is TRUE and folds always; `!= NULL` is FALSE and folds
// never.
// D  `EX_XFILE` reaches zero rows -- `:file` was its last one -- so do_one_cmd's
// `expand_filename()` call can never be entered.  Folding it never is what
// hands the sweep 32 functions and some 1,300 lines, the same shape as phase
// 91's EX_ARGOPT.
// E  `readonlymode` and `b_dev_valid`'s assignment, each write-only after C and
// B, and neither of them anything a warning or a sweep tool can see.
// F  `shorten_fnames()` loses the cwd it fetched for a now-empty
// `shorten_buf_fname()`.
// G  `find_file_name_in_path()`'s `FNAME_EXP` arm.
//
// FOLD `buflist_name_nr` AT ITS CALLERS, NEVER IN PLACE, and the agent that surveyed
// this phase made the mistake first.  Its body is `buf = buflist_findnr(fnum); if
// (buf == NULL || buf->b_fname == NULL) return FAIL; *fname = buf->b_fname; ...
// return OK;`.  Folding the whole `if` away gives a function that returns OK with
// `*fname` never written -- a silent behaviour change in the direction that crashes.
// What is true is that it returns FAIL ALWAYS, so the fold belongs at
// `getaltfname()` and at `ex_display()`, and only then is it uncalled and the
// sweep's.
//
// THREE SITES HAVE AN `else` AND cutil.fold_always REFUSES THEM, by design: keeping
// a body and dropping an else is not what it does.  fileinfo(), set_b0_fname() and
// get_trans_bufname() use the local fold_always_else() below, which keeps the if
// body where it was; the canonical print re-indents it.  And FOUR MORE are a function whose whole body is
// the `if`: buf_spname(), buf_get_fname(), check_fname() and getaltfname().
// fold_always there leaves an unreachable `return buf->b_fname;` behind -- measured
// -- which no sweep tool removes and which would keep `b_fname` alive for ever.
// Those four are exact-text rewrites of the body.
//
// THE TWO FOLDS IN eval_vars() ARE NOT MADE, and that is a correction to the brief
// this phase was written from.  Both of its `if (b_fname == NULL)` arms are inside a
// function part D makes unreachable: `expand_filename()` and
// `expand_wildcards_eval()` are its only callers and both go.  Folding inside text
// the sweep deletes changes no output and states nothing, so rule 1 applies -- the
// check requires `eval_vars` at 0 mentions afterwards, which is the assertion that
// replaces the fold.
//
// WHAT THIS PHASE NEEDS OF PHASES 90, 91 AND 92, and none of it can be a `uses` line,
// packages.sh refusing one inside a package: `:read` was one of the six EX_XFILE
// rows and phase 90 took it; four more went with the :edit family in phase 91, which
// is why :file is the LAST and part D exists at all; and `set_rw_fname` was
// `setfname`'s second caller and went with `readfile` in phase 92, which is what
// leaves `rename_buffer` as its only one.
//
// THE INPUT BINARY IS BUILT BY THE PLAN, before the edit, from the boundary's own makefile
// flags, as every Part II edit since phase 85 does, and the source goes with it as
// $state/old.c.  The check needs both: `:file NEWNAME` is the one thing that proves
// the old binary could name a buffer at all, and no recording can see it.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).

import (
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim93", Edit) }

// w93Fields are the three the whole phase is about, and they are NULL for ever
// once part B has run.
var w93Fields = []string{"b_ffname", "b_sfname", "b_fname"}

var w93Before = map[string]int{
	"b_ffname": 32, "b_sfname": 26, "b_fname": 29,
	"CMD_file": 4, "EX_XFILE": 4, "buflist_new": 3, "buflist_name_nr": 3,
	"buf_spname": 7, "buf_get_fname": 3, "fileinfo": 4, "check_fname": 3,
	"readonlymode": 3, "mch_dirname": 5, "shorten_buf_fname": 2,
	"check_changed": 4, "no_write_message": 3,
	"p_ur": 2, "p_ro": 2, "read_cmd_fd": 12, "vim_fsync": 3,
	"scriptin": 8, "redir_fd": 6,
}

var w93After = map[string]int{
	"CMD_file": 0, "EX_XFILE": 1, "buflist_name_nr": 1, "readonlymode": 1,
	"shorten_buf_fname": 1, "check_fname": 3, "buf_get_fname": 3,
	"check_changed": 4, "no_write_message": 3, "p_ur": 2, "p_ro": 2,
	"read_cmd_fd": 12, "vim_fsync": 3, "scriptin": 8, "redir_fd": 6,
}

// w93Writers are the four functions every write to the three fields lives in,
// and w93Readers the five every surviving mention lives in.  Both are computed
// against, not asserted about: a write anywhere else means every fold is a guess.
var w93Writers = []string{"buflist_new", "setfname", "rename_buffer", "shorten_buf_fname"}
var w93Readers = []string{"setfname", "rename_buffer", "otherfile_buf", "buf_setino",
	"eval_vars", "buflist_name_nr"}

var (
	w93CmdRow  = regexp.MustCompile(`(?m)^    \[CMD_file\] = \{.*\n`)
	w93AnyRow  = regexp.MustCompile(`(?m)^    \[CMD_\w+\] = \{.*$`)
	w93ElseTop = regexp.MustCompile(`^[ \t]*else[ \t]*\n[ \t]*\{`)
)

// Whim93 takes the buffer's NAME: `:file`, buflist_new()'s two name parameters,
// sixteen folds of b_ffname/b_sfname/b_fname and three further folds that free
// the last three questions the core asked the filesystem.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "noname", W: w}
	var err error

	mentions := edit.MentionCount
	textEdit := func(t []byte, old, new, what string, n int) ([]byte, error) {
		k := strings.Count(string(t), old)
		if k != n {
			return nil, p.Die("%s -- the text occurs %d times, expected %d: %s",
				what, k, n, cutil.PyRepr(edit.CoreHead(old, 70)))
		}
		p.Say(what)
		return []byte(strings.ReplaceAll(string(t), old, new)), nil
	}
	inFunction := func(t []byte, name string, edit func([]byte) ([]byte, error)) ([]byte, error) {
		a, z, ok := cutil.FindDefinition(t, cutil.Blank(t), name)
		if !ok {
			return nil, p.Die("%s is not defined", name)
		}
		Body, err := edit(t[a:z])
		if err != nil {
			return nil, err
		}
		return []byte(string(t[:a]) + string(Body) + string(t[z:])), nil
	}
	within := func(t []byte, fn, old, new, what string, n int) ([]byte, error) {
		Out, err := inFunction(t, fn, func(s []byte) ([]byte, error) {
			k := strings.Count(string(s), old)
			if k != n {
				return nil, p.Die("%s -- %s occurs %d times in %s, expected %d",
					what, cutil.PyRepr(edit.CoreHead(old, 60)), k, fn, n)
			}
			return []byte(strings.ReplaceAll(string(s), old, new)), nil
		})
		if err != nil {
			return nil, err
		}
		p.Say(what)
		return Out, nil
	}
	// fold is SCOPED TO ONE DEFINITION, and file-wide would be wrong here rather
	// than merely loose: `if (buf->b_ffname == NULL)` is the whole of
	// close_buffer's fold and the head of set_b0_fname's, written identically at
	// the same indent, so a file-wide count of 1 fails and a count of 2 would
	// fold two different shapes with one rule.
	fold := func(t []byte, fn, how, pattern, what string, n int) ([]byte, error) {
		Out, err := inFunction(t, fn, func(s []byte) ([]byte, error) {
			var f func([]byte, string, int) ([]byte, error)
			switch how {
			case "always":
				f = cutil.FoldAlways
			case "never":
				f = cutil.FoldNever
			default:
				f = cutil.DropIf
			}
			o, err := f(s, pattern, n)
			if err != nil {
				return nil, p.Die("%s -- %v", what, err)
			}
			return o, nil
		})
		if err != nil {
			return nil, err
		}
		p.Say(what)
		return Out, nil
	}
	// foldAlwaysElse keeps A of `if (TRUE) { A } else { B }` inside fn.
	// cutil.FoldAlways refuses a block with an else, deliberately, and this is
	// the shape three sites here have.  The Body is kept as it is written; the
	// canonical print re-indents it.
	foldAlwaysElse := func(t []byte, fn, ifline, what string) ([]byte, error) {
		Out, err := inFunction(t, fn, func(sb []byte) ([]byte, error) {
			s := string(sb)
			if k := strings.Count(s, ifline); k != 1 {
				return nil, p.Die("%s -- the if line occurs %d times in %s, expected 1", what, k, fn)
			}
			b := cutil.Blank(sb)
			i := strings.Index(s, ifline)
			o := strings.Index(string(b[i+len(ifline)-1:]), "{") + i + len(ifline) - 1
			c := cutil.Match(b, o)
			if c < 0 {
				return nil, p.Die("%s -- unbalanced block", what)
			}
			endIf := strings.Index(s[c:], "\n") + c + 1
			m := w93ElseTop.FindStringIndex(s[endIf:])
			if m == nil {
				return nil, p.Die("%s -- the block has no else, so cutil.fold_always is the tool", what)
			}
			o2 := endIf + m[1] - 1
			c2 := cutil.Match(b, o2)
			if c2 < 0 {
				return nil, p.Die("%s -- unbalanced else block", what)
			}
			Body := s[strings.Index(s[o:], "\n")+o+1 : strings.LastIndex(s[:c], "\n")+1]
			return []byte(s[:i] + Body + s[strings.Index(s[c2:], "\n")+c2+1:]), nil
		})
		if err != nil {
			return nil, err
		}
		p.Say(what)
		return Out, nil
	}
	// assignments is every write to a field: `x->name =`, and `(x->name) =` as
	// slim spells it.  uses is every mention through `->`, so not its declaration.
	assignments := func(t []byte, name string) []int {
		var Out []int
		for _, m := range regexp.MustCompile(`\b`+name+`\b\s*\)?\s*=[^=]`).FindAllIndex(t, -1) {
			Out = append(Out, m[0])
		}
		return Out
	}
	uses := func(t []byte, name string) []int {
		var Out []int
		for _, m := range regexp.MustCompile(`->\s*`+name+`\b`).FindAllIndex(t, -1) {
			Out = append(Out, m[0])
		}
		return Out
	}
	functionsHolding := func(t []byte, offsets []int, names []string) ([]string, error) {
		type sp struct {
			a, z int
			Name string
		}
		var spans []sp
		b := cutil.Blank(t)
		for _, n := range names {
			if a, z, ok := cutil.FindDefinition(t, b, n); ok {
				spans = append(spans, sp{a, z, n})
			}
		}
		seen := map[string]bool{}
		for _, off := range offsets {
			hit := false
			for _, s := range spans {
				if s.a <= off && off < s.z {
					seen[s.Name] = true
					hit = true
					break
				}
			}
			if !hit {
				sorted := append([]string{}, names...)
				sort.Strings(sorted)
				return nil, p.Die("a mention at line %d is in none of %s -- this phase was counted "+
					"against a different file",
					strings.Count(string(t[:off]), "\n")+1, strings.Join(sorted, " "))
			}
		}
		Out := make([]string, 0, len(seen))
		for n := range seen {
			Out = append(Out, n)
		}
		sort.Strings(Out)
		return Out, nil
	}

	// ---- 0. the shape every anchor below was counted against ------------------
	for _, name := range edit.SortedKeys(w93Before) {
		if k := mentions(text, name); k != w93Before[name] {
			return nil, p.Die("%s has %d mentions, expected %d -- the anchors below were counted "+
				"against a different file", name, k, w93Before[name])
		}
	}
	p.Say("b_ffname 32, b_sfname 26, b_fname 29, CMD_file 4, EX_XFILE 4 -- the file the " +
		"seven parts were counted against")

	var offs []int
	for _, fld := range w93Fields {
		offs = append(offs, assignments(text, fld)...)
	}
	got, err := functionsHolding(text, offs, w93Writers)
	if err != nil {
		return nil, err
	}
	want := append([]string{}, w93Writers...)
	sort.Strings(want)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		return nil, p.Die("the writes to %s live in %s, expected exactly %s",
			strings.Join(w93Fields, "/"), strings.Join(got, " "), strings.Join(want, " "))
	}
	p.Sayf("%d writes to b_ffname, b_sfname and b_fname, and every one is in buflist_new, "+
		"setfname, rename_buffer or shorten_buf_fname -- the four this phase accounts "+
		"for.  That, and nothing weaker, is why every fold below may take a constant", len(offs))

	// ---- A. :file goes --------------------------------------------------------
	if text, err = textEdit(text, w93lit3, "", "the CMD_file enumerator of enum CMD_index", 1); err != nil {
		return nil, err
	}
	rows := w93CmdRow.FindAllString(string(text), -1)
	if len(rows) != 1 {
		return nil, p.Die("the cmdnames[] row for :file matches %d lines, expected 1", len(rows))
	}
	text = []byte(strings.ReplaceAll(string(text), rows[0], ""))
	p.Say("the cmdnames[] row [CMD_file] = {...}, one physical line: ex_file has no other " +
		"reference, and rename_buffer and setfname no other caller")
	if text, err = textEdit(text, w93lit4, w93lit5,
		"do_one_cmd's curbuf_locked() exemption: `ea.cmdidx != CMD_file` is "+
			"TRUE for ever, and phase 91 kept it saying this phase would take it", 1); err != nil {
		return nil, err
	}
	if text, err = textEdit(text, w93lit6, "",
		"do_one_cmd's second CMD_file test, deleted as text rather than "+
			"folded: its condition names the enumerator that is going", 1); err != nil {
		return nil, err
	}
	if k := mentions(text, "CMD_file"); k != 0 {
		return nil, p.Die("CMD_file still has %d mentions", k)
	}

	// ---- B. buflist_new never names -------------------------------------------
	for _, e := range []struct{ Old, New, What string }{
		{w93lit7, w93lit8, "buflist_new's prototype loses both name parameters"},
		{w93lit9, w93lit10, "and so does its definition"},
	} {
		if text, err = textEdit(text, e.Old, e.New, e.What, 1); err != nil {
			return nil, err
		}
	}
	for _, e := range []struct{ Old, New, What string }{
		{w93lit13, "", "the prologue and the lookup: fname_expand() on two NULLs, a stat() the " +
			"`sfname == NULL` disjunct already short-circuited, and the search for " +
			"an existing buffer of the same name, whose guard `ffname != NULL` is " +
			"FALSE -- no buffer can be found by a name that is not given"},
		{w93lit14, w93lit15, "the alloc failure arm's vim_free(ffname): there is no ffname to free"},
		{w93lit16, "", "the assignment that named the buffer -- `ffname != NULL` is FALSE, and " +
			"this is the statement the whole phase is about"},
		{w93lit17, w93lit18, "the failure arm: its first disjunct is FALSE, so only the wininfo " +
			"allocation can fail, and the two names it freed are not there to free"},
		{w93lit19, "", "b_fname = b_sfname, which is NULL = NULL"},
		{w93lit20, w93lit21, "the device block: `st.st_dev` was set to -1 by the prologue that has " +
			"gone, so the TRUE arm is the one that ran and b_dev_valid is false"},
	} {
		if text, err = within(text, "buflist_new", e.Old, e.New, e.What, 1); err != nil {
			return nil, err
		}
	}
	if text, err = textEdit(text, w93lit22, w93lit23,
		"the one call site, create_windows', which already passed NULL, NULL", 1); err != nil {
		return nil, err
	}
	a, z, ok := cutil.FindDefinition(text, cutil.Blank(text), "buflist_new")
	if !ok {
		return nil, p.Die("buflist_new is not defined")
	}
	// The ffname, sfname and st locals are still declared, and nothing else
	// in buflist_new names them; the sweep takes the three.
	for _, gone := range []string{"ffname", "sfname", "st"} {
		if mentions(text[a:z], gone) != 1 {
			return nil, p.Die("%s is named inside buflist_new other than by its declaration", gone)
		}
	}
	p.Say("buflist_new names nothing: ffname, sfname and st are left as unused locals")

	// ---- C. the sixteen folds, with the constant each takes -------------------
	if text, err = fold(text, "open_buffer", "never",
		`(?m)^    if \(readonlymode && curbuf->b_ffname != NULL && \(curbuf->b_flags & BF_NEVERLOADED\)\)$`,
		"open_buffer: `b_ffname != NULL` is FALSE, so a buffer can never be made "+
			"read-only for being a never-loaded file -- and this was readonlymode's "+
			"only reader", 1); err != nil {
		return nil, err
	}
	if text, err = textEdit(text, w93lit24, w93lit25,
		"can_unload_buffer: `fname` is NULL either way, so E937 names the "+
			"buffer \"[No Name]\" -- which is what it printed before", 1); err != nil {
		return nil, err
	}
	if text, err = fold(text, "close_buffer", "always", `(?m)^    if \(buf->b_ffname == NULL\)$`,
		"close_buffer: `b_ffname == NULL` is TRUE, so an unloaded buffer is always "+
			"deleted rather than kept for its name", 1); err != nil {
		return nil, err
	}
	if text, err = textEdit(text, "curbuf != NULL && curbuf->b_ffname == NULL && curbuf->b_nwindows <= 1",
		"curbuf != NULL && curbuf->b_nwindows <= 1",
		"curbuf_reusable: `b_ffname == NULL` is TRUE, so the conjunct goes "+
			"rather than being kept with a fixed answer", 1); err != nil {
		return nil, err
	}
	if text, err = textEdit(text, w93lit26, w93lit27,
		"getaltfname: buflist_name_nr() is FAIL ALWAYS, so the alternate file "+
			"is E23 and NULL -- which is what the `#` register already answered", 1); err != nil {
		return nil, err
	}
	if text, err = foldAlwaysElse(text, "fileinfo", w93lit28,
		"fileinfo: buf_spname() never returns NULL now, so CTRL-G "+
			"prints the special name and never a path -- the else arm, "+
			"which read b_fname and b_ffname, cannot be entered"); err != nil {
		return nil, err
	}
	for _, e := range []struct{ Old, New, What string }{
		{w93lit29, w93lit30, "buf_spname: `b_fname == NULL` is TRUE, so it answers for every " +
			"buffer and can no longer return NULL"},
		{w93lit31, w93lit32, "buf_get_fname: the same, and \"[No Name]\" is now the only name the " +
			"editor has for a buffer"},
		{w93lit33, w93lit34, "check_changed_any: buf_spname() is non-NULL, so E162 names the " +
			"buffer through it and never through b_fname"},
		{w93lit35, w93lit36, "check_fname: E32 for every buffer, and it stays because the `%` " +
			"register still asks it"},
	} {
		if text, err = textEdit(text, e.Old, e.New, e.What, 1); err != nil {
			return nil, err
		}
	}
	if text, err = fold(text, "shorten_buf_fname", "never",
		`(?m)^    if \(buf->b_fname != NULL && !path_with_url\(buf->b_fname\) && \(force \|\| buf->b_sfname == NULL \|\| mch_isFullName\(buf->b_sfname\)\)\)$`,
		"shorten_buf_fname: `b_fname != NULL` is FALSE, so there is no path to "+
			"shorten and the function has nothing left to do", 1); err != nil {
		return nil, err
	}
	if text, err = textEdit(text, w93lit37, w93lit38,
		"file_name_at_cursor: `curbuf->b_ffname` is the NULL it passes now", 1); err != nil {
		return nil, err
	}
	if text, err = foldAlwaysElse(text, "set_b0_fname", w93lit39,
		"set_b0_fname: `b_ffname == NULL` is TRUE, so block zero's "+
			"file name is empty -- and the stat() in the arm that goes is "+
			"one of the two this phase takes"); err != nil {
		return nil, err
	}
	if text, err = textEdit(text, w93lit40, w93lit41,
		"get_spec_reg: the `%` register is `b_fname`, which is NULL -- the "+
			"register already yielded nothing, and check_fname() above it still "+
			"says E32", 1); err != nil {
		return nil, err
	}
	if text, err = fold(text, "ex_display", "never",
		`(?m)^    if \(curbuf->b_fname != NULL && \(arg == NULL \|\| vim_strchr\(arg, '%'\) != NULL\) && !got_int && !message_filtered\(curbuf->b_fname\)\)$`,
		"ex_display: the `\"%` line of :registers needs a buffer name and there is "+
			"none -- it was already never printed", 1); err != nil {
		return nil, err
	}
	if text, err = fold(text, "ex_display", "drop",
		`(?m)^    if \(\(arg == NULL \|\| vim_strchr\(arg, '#'\) != NULL\) && !got_int\)$`,
		"ex_display: and the `\"#` block goes whole, because buflist_name_nr() "+
			"inside it is FAIL ALWAYS and the block holds nothing else -- which is "+
			"what makes that function uncalled and the sweep's", 1); err != nil {
		return nil, err
	}
	if text, err = foldAlwaysElse(text, "get_trans_bufname", w93lit42,
		"get_trans_bufname: buf_spname() is non-NULL, so every window "+
			"and every :ls row reads \"[No Name]\" -- as they already did"); err != nil {
		return nil, err
	}
	if k := mentions(text, "buflist_name_nr"); k != 1 {
		return nil, p.Die("buflist_name_nr has %d mentions after both callers were folded, expected "+
			"1 -- its definition, for the sweep", k)
	}

	// ---- D. EX_XFILE reaches zero rows ----------------------------------------
	var left []string
	for _, r := range w93AnyRow.FindAllString(string(text), -1) {
		if strings.Contains(r, "EX_XFILE") {
			left = append(left, r)
		}
	}
	if len(left) > 0 {
		return nil, p.Die("%d cmdnames[] rows still carry EX_XFILE, so the fold below would be a "+
			"guess: %s", len(left), edit.CoreHead(left[0], 60))
	}
	p.Say("no cmdnames[] row carries EX_XFILE any more -- :file was the last, as :read " +
		"was EX_ARGOPT's in phase 90 and the :edit family in phase 91")
	if text, err = fold(text, "do_one_cmd", "never",
		`(?m)^    if \(\(ea\.argt & EX_XFILE\) && expand_filename\(&ea, cmdlinep, &errormsg\) == FAIL\)$`,
		"do_one_cmd's expand_filename() call: `ea.argt & EX_XFILE` is 0 for every "+
			"command, so this is the anchor the sweep reads the whole "+
			"filename-expansion layer from", 1); err != nil {
		return nil, err
	}
	if text, err = textEdit(text, w93lit43, w93lit44,
		"separate_nextcmd's CTRL-V test: the EX_XFILE disjunct is 0 for every "+
			"row, and dropping it is what takes the enumerator to zero mentions", 1); err != nil {
		return nil, err
	}
	if k := mentions(text, "EX_XFILE"); k != 1 {
		return nil, p.Die("EX_XFILE has %d mentions, expected 1 -- its own definition, for the sweep", k)
	}

	// ---- E. the two write-only leftovers --------------------------------------
	if text, err = fold(text, "did_set_readonly", "drop",
		`(?m)^    if \(!curbuf->b_p_ro && \(args->os_flags & OPT_LOCAL\) == 0\)$`,
		"did_set_readonly's readonlymode write, with the `if` around it: C1 took "+
			"the only reader, and an `if` with an empty body is not something any tool "+
			"here removes", 1); err != nil {
		return nil, err
	}
	if text, err = within(text, "buflist_new", w93lit21, "",
		"b_dev_valid's one surviving assignment, which part B left: every reader "+
			"is inside a function the sweep takes, and deadfields.py cannot remove a "+
			"field that is still written", 1); err != nil {
		return nil, err
	}

	// ---- F. shorten_fnames stops asking where it is ---------------------------
	if text, err = within(text, "shorten_fnames", w93lit46, "",
		"shorten_fnames: the cwd, and the call to a function with an empty body", 1); err != nil {
		return nil, err
	}
	for _, e := range []struct{ Old, New, What string }{
		{w93lit47, w93lit48, "and its prototype takes void, because an unused PARAMETER is what " +
			"tools/sweep.sh's -Wno-unused-parameter cannot see -- phase 92's " +
			"anchor 4 measured that"},
		{w93lit49, w93lit50, "the definition with it"},
		{w93lit51, w93lit52, "and its one call site"},
	} {
		if text, err = textEdit(text, e.Old, e.New, e.What, 1); err != nil {
			return nil, err
		}
	}

	// ---- G. nothing looks a name up on a disk ---------------------------------
	if text, err = fold(text, "find_file_name_in_path", "never", `(?m)^    if \(options & FNAME_EXP\)$`,
		"find_file_name_in_path: the `path` search arm goes, so CTRL-F and CTRL-P "+
			"both extract the word under the cursor and neither consults a disk -- "+
			"this is the fold that frees stat()", 1); err != nil {
		return nil, err
	}

	// ---- what the sweep is handed, as a count rather than as trust ------------
	offs = nil
	writes := 0
	for _, fld := range w93Fields {
		offs = append(offs, uses(text, fld)...)
		writes += len(assignments(text, fld))
	}
	got, err = functionsHolding(text, offs, w93Readers)
	if err != nil {
		return nil, err
	}
	want = append([]string{}, w93Readers...)
	sort.Strings(want)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		return nil, p.Die("the surviving mentions of the three fields are in %s, expected exactly %s",
			strings.Join(got, " "), strings.Join(want, " "))
	}
	p.Sayf("%d mentions of b_ffname, b_sfname and b_fname are left, %d of them writes, and "+
		"every one is inside setfname, rename_buffer, otherfile_buf, buf_setino, "+
		"eval_vars or buflist_name_nr -- none of which has a caller the sweep can "+
		"reach", len(offs), writes)

	for _, name := range edit.SortedKeys(w93After) {
		if k := mentions(text, name); k != w93After[name] {
			return nil, p.Die("%s has %d mentions after the cut, expected %d", name, k, w93After[name])
		}
	}
	p.Say("the cut is done: CMD_file 0, EX_XFILE 1 (its own definition), buflist_name_nr " +
		"1, readonlymode 0 -- and read_cmd_fd 12, vim_fsync 3, scriptin 8 and redir_fd " +
		"6 untouched, each of them a later phase's")
	return text, nil
}
