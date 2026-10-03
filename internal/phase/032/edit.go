package p032

// Whim phase 32 (formerly 93) -- the buffer has no name.  See GOAL.md.
//
// Phases 28, 29 and 30 took every way to ASK for a file and phase 31 took the machinery
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
// CMD_file tests -- the `curbuf_locked()` conjunct phase 30 deliberately kept,
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
// 30's EX_ARGOPT.
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
// WHAT THIS PHASE NEEDS OF PHASES 29, 30 AND 31, and none of it can be a `uses` line,
// packages.sh refusing one inside a package: `:read` was one of the six EX_XFILE
// rows and phase 29 took it; four more went with the :edit family in phase 30, which
// is why :file is the LAST and part D exists at all; and `set_rw_fname` was
// `setfname`'s second caller and went with `readfile` in phase 31, which is what
// leaves `rename_buffer` as its only one.
//
// THE INPUT BINARY IS BUILT BY THE PLAN, before the edit, from the boundary's own makefile
// flags, as every Part II edit since phase 4e does, and the source goes with it as
// $state/old.c.  The check needs both: `:file NEWNAME` is the one thing that proves
// the old binary could name a buffer at all, and no recording can see it.
// The flags are read out of the boundary's makefile rather than written here a
// second time: the core's compile line is the boundary's (GOALS.md core rule 8).

//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3b).  The same acts, in the same
// order, on the program's graph, and the text version's report (history
// keeps it, with editlit.go's literals):
//
//   - the anchors, the writes of the three fields and the functions they
//     stand in, the rows and the last counts are the text's own questions
//     on the C view (TEXTQ), its regular expressions and its definitions'
//     spans;
//   - a literal of whole statements is the run of items it was (Run,
//     CutRun), found once in its function; a literal that took an operand
//     is DropOperand, or the operand the parentheses held moved into place;
//   - a `return K;` body behind an if of a constant is that if folded and
//     the return after it cut;
//   - buflist_new's and shorten_fnames' parameters are PARAM's, each one
//     edit for its prototype, definition and call, reported as the text's
//     three literals were, where the text made the last of them.  The text
//     left `char_u *ffname = ffname_arg;` naming a parameter that was gone,
//     for the sweep; the graph leaves that use dangling (ParamOptions'
//     Dangle) for the collection, which takes the local with it.

import (
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim32", Edit) }

// w32Fields are the three the whole phase is about, and they are NULL for ever
// once part B has run.
var w32Fields = []string{"b_ffname", "b_sfname", "b_fname"}

var w32Before = map[string]int{
	// :file's row, ex_file and rename_buffer, and :write's and :edit's uses
	// of the names, went at phase 1 (filefront, the reform's D4)
	"b_ffname": 22, "b_sfname": 15, "b_fname": 27, // open_buffer's went at phase 3 (D9)
	"CMD_file": 3, "EX_XFILE": 3, "buflist_new": 3, "buflist_name_nr": 3,
	"buf_spname": 5, "buf_get_fname": 3, "fileinfo": 3, "check_fname": 3,
	"readonlymode": 0, "mch_dirname": 5, // readonlymode falls out at phase 3 since D9 "shorten_buf_fname": 2,
	// check_changed went with :q's refusal at phase 1 (quitfront)
	"check_changed": 0, "no_write_message": 0,
	"p_ur": 0, "p_ro": 0, "read_cmd_fd": 12, "vim_fsync": 3, // their rows went at phase 1 (D3)
	"scriptin": 8, "redir_fd": 0, // folded at phase 1: only :redir wrote it (D2)
}

var w32After = map[string]int{
	"CMD_file": 0, "EX_XFILE": 1, "buflist_name_nr": 1, "readonlymode": 0,
	"shorten_buf_fname": 1, "check_fname": 3, "buf_get_fname": 3,
	"check_changed": 0, "no_write_message": 0, "p_ur": 0, "p_ro": 0,
	"read_cmd_fd": 12, "vim_fsync": 3, "scriptin": 8, "redir_fd": 0,
}

// w32Writers are the four functions every write to the three fields lives in,
// and w32Readers the five every surviving mention lives in.  Both are computed
// against, not asserted about: a write anywhere else means every fold is a guess.
// (setfname and rename_buffer went with :file at phase 1, filefront, D4.)
var w32Writers = []string{"buflist_new", "shorten_buf_fname"}
var w32Readers = []string{"otherfile_buf", "buf_setino", "eval_vars", "buflist_name_nr"}

var w32AnyRow = regexp.MustCompile(`(?m)^    \[CMD_\w+\] = \{.*$`)

func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("noname", e, w)

	assignments := func(t []byte, name string) []int {
		var out []int
		for _, m := range regexp.MustCompile(`\b`+name+`\b\s*\)?\s*=[^=]`).FindAllIndex(t, -1) {
			out = append(out, m[0])
		}
		return out
	}
	uses := func(t []byte, name string) []int {
		var out []int
		for _, m := range regexp.MustCompile(`->\s*`+name+`\b`).FindAllIndex(t, -1) {
			out = append(out, m[0])
		}
		return out
	}
	// functionsHolding is the text's: the functions whose definitions'
	// spans in t hold the offsets, refused where one is in none of names
	functionsHolding := func(t []byte, offsets []int, names []string) []string {
		type sp struct {
			a, z int
			Name string
		}
		var spans []sp
		b := edit.Blank(t)
		for _, n := range names {
			if a, z, ok := edit.FindDefinition(t, b, n); ok {
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
				v.Die("a mention at line %d is in none of %s -- this phase was counted "+
					"against a different file",
					strings.Count(string(t[:off]), "\n")+1, strings.Join(sorted, " "))
				return nil
			}
		}
		out := make([]string, 0, len(seen))
		for n := range seen {
			out = append(out, n)
		}
		sort.Strings(out)
		return out
	}
	mentionsAre := func(t []byte, name string, n int, format string) {
		if k := edit.MentionCount(t, name); k != n {
			v.Die(format, k)
		}
	}
	// the then arm of an if of a constant, and the return after it cut:
	// `if (K) { A } return R;` is A
	keepThenReturn := func(fn, cond, ret, what string) {
		v.InFunction(fn, func(v *graph.Verbs) {
			v.Muted(func(v *graph.Verbs) {
				v.FoldAlwaysAt(cond, 1, false, what)
				v.Cut(ret, 1, what)
			})
		})
		v.Say(what)
	}

	t := v.Text()
	for _, name := range edit.SortedKeys(w32Before) {
		if k := edit.MentionCount(t, name); k != w32Before[name] {
			v.Die("%s has %d mentions, expected %d -- the anchors below were counted "+
				"against a different file", name, k, w32Before[name])
			return v.Done()
		}
	}
	v.Say("b_ffname 22, b_sfname 15, b_fname 27, CMD_file 3, EX_XFILE 3 -- the file the " +
		"seven parts were counted against")

	var offs []int
	for _, fld := range w32Fields {
		offs = append(offs, assignments(t, fld)...)
	}
	got := functionsHolding(t, offs, w32Writers)
	if v.Failed() {
		return v.Done()
	}
	want := append([]string{}, w32Writers...)
	sort.Strings(want)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		v.Die("the writes to %s live in %s, expected exactly %s",
			strings.Join(w32Fields, "/"), strings.Join(got, " "), strings.Join(want, " "))
		return v.Done()
	}
	v.Sayf("%d writes to b_ffname, b_sfname and b_fname, and every one is in buflist_new "+
		"or shorten_buf_fname -- the two this phase accounts for.  That, and nothing "+
		"weaker, is why every fold below may take a constant", len(offs))

	v.DeleteEnumerators([]string{"CMD_file"}, graph.Renumber, "the CMD_file enumerator of enum CMD_index")
	v.InFunction("do_one_cmd", func(v *graph.Verbs) {
		v.DropOperand("(!= (. ea cmdidx) CMD_file)", 1,
			"do_one_cmd's curbuf_locked() exemption: `ea.cmdidx != CMD_file` is "+
				"TRUE for ever, and phase 30 kept it saying this phase would take it")
		v.Cut("(if (&& (== (. ea cmdidx) CMD_file) (!= (deref (. ea arg)) NUL) (call curbuf_locked)) (block (goto doend)))", 1,
			"do_one_cmd's second CMD_file test, deleted as text rather than "+
				"folded: its condition names the enumerator that is going")
	})
	if v.Failed() {
		return v.Done()
	}
	mentionsAre(v.Text(), "CMD_file", 0, "CMD_file still has %d mentions")

	// buflist_new's two name parameters: one edit, made where the text made
	// its call site's literal
	v.Say("buflist_new's prototype loses both name parameters")
	v.Say("and so does its definition")
	v.InFunction("buflist_new", func(v *graph.Verbs) {
		v.CutRun("the prologue and the lookup: fname_expand() on two NULLs, a stat() the "+
			"`sfname == nullptr` disjunct already short-circuited, and the search for "+
			"an existing buffer of the same name, whose guard `ffname != nullptr` is "+
			"FALSE -- no buffer can be found by a name that is not given",
			"(call fname_expand curbuf (addr ffname) (addr sfname))",
			"(if (|| (== sfname nullptr) (< (call stat (paren (cast (ptr char) sfname)) (paren (addr st))) 0)) (block (= (. st st_dev) (cast dev_t (- 1)))))",
			"(if (&& (!= ffname nullptr) (! (& flags (| BLN_DUMMY BLN_NEW))) (!= (= buf (call buflist_findname_stat ffname (addr st))) nullptr)) _*)")
		if x := v.One("(if (== buf nullptr) (block (call vim_free ffname) (return nullptr)))",
			"the alloc failure arm's vim_free(ffname): there is no ffname to free"); x != nil {
			v.In(x, func(v *graph.Verbs) {
				v.Cut("(call vim_free ffname)", 1, "the alloc failure arm's vim_free(ffname): there is no ffname to free")
			})
		}
		v.Cut("(if (!= ffname nullptr) (block (= (-> buf b_ffname) ffname) (= (-> buf b_sfname) (call vim_strsave sfname))))", 1,
			"the assignment that named the buffer -- `ffname != nullptr` is FALSE, and "+
				"this is the statement the whole phase is about")
		const fail = "the failure arm: its first disjunct is FALSE, so only the wininfo " +
			"allocation can fail, and the two names it freed are not there to free"
		if x := v.One("(if (|| (paren (&& (!= ffname nullptr) (|| (== (-> buf b_ffname) nullptr) (== (-> buf b_sfname) nullptr)))) (== (-> buf b_wininfo) nullptr)) _*)", fail); x != nil {
			v.Muted(func(v *graph.Verbs) {
				v.In(x, func(v *graph.Verbs) {
					v.DropOperand("(paren (&& (!= ffname nullptr) _*))", 1, fail)
					v.CutRun(fail,
						"(if (!= (-> buf b_sfname) (-> buf b_ffname)) _*)",
						"(call vim_free (-> buf b_ffname))",
						"(= (paren (-> buf b_ffname)) nullptr)")
				})
			})
		}
		v.Say(fail)
		v.Cut("(= (-> buf b_fname) (-> buf b_sfname))", 1, "b_fname = b_sfname, which is nullptr = nullptr")
		v.FoldAlwaysElse("(== (. st st_dev) (cast dev_t (- 1)))", 1,
			"the device block: `st.st_dev` was set to -1 by the prologue that has "+
				"gone, so the TRUE arm is the one that ran and b_dev_valid is false")
	})

	// the one call site, and the two parameters with it
	// (create_windows, the text's report says; it is in win_alloc_firstwin)
	v.InFunction("win_alloc_firstwin", func(v *graph.Verbs) {
		v.One("(= curbuf (call buflist_new nullptr nullptr 1L BLN_LISTED))",
			"the one call site, create_windows', which already passed nullptr, nullptr")
	})
	if !v.Failed() {
		var drops []graph.ParamDrop
		for _, p := range []string{"ffname_arg", "sfname_arg"} {
			drops = append(drops, graph.ParamDrop{Decl: e.Defn("buflist_new"), I: e.ParamIndex("buflist_new", p)})
		}
		if _, err := e.DropParams(drops, graph.ParamOptions{Dangle: true}); err != nil {
			v.Die("the one call site, create_windows' -- %v", err)
		}
	}
	v.Say("the one call site, create_windows', which already passed nullptr, nullptr")
	v.InFunction("buflist_new", func(v *graph.Verbs) {
		body := v.Text()
		for _, gone := range []string{"ffname", "sfname", "st"} {
			if edit.MentionCount(body, gone) != 1 {
				v.Die("%s is named inside buflist_new other than by its declaration", gone)
				return
			}
		}
	})
	v.Say("buflist_new names nothing: ffname, sfname and st are left as unused locals")

	const unload = "can_unload_buffer: `fname` is nullptr either way, so E937 names the " +
		"buffer \"[No Name]\" -- which is what it printed before"
	v.InFunction("can_unload_buffer", func(v *graph.Verbs) {
		v.Muted(func(v *graph.Verbs) {
			v.Rewrite("(? (!= fname nullptr) fname ?n)", "?n", 1, unload)
			v.Cut("(def fname (ptr char_u) (? (!= (-> buf b_fname) nullptr) (-> buf b_fname) (-> buf b_ffname)))", 1, unload)
		})
	})
	v.Say(unload)
	v.InFunction("close_buffer", func(v *graph.Verbs) {
		v.FoldAlwaysAt("(== (-> buf b_ffname) nullptr)", 1, false,
			"close_buffer: `b_ffname == nullptr` is TRUE, so an unloaded buffer is always "+
				"deleted rather than kept for its name")
	})
	v.InFunction("curbuf_reusable", func(v *graph.Verbs) {
		v.DropOperand("(== (-> curbuf b_ffname) nullptr)", 1,
			"curbuf_reusable: `b_ffname == nullptr` is TRUE, so the conjunct goes "+
				"rather than being kept with a fixed answer")
	})
	const alt = "getaltfname: buflist_name_nr() is FAIL ALWAYS, so the alternate file " +
		"is E23 and nullptr -- which is what the `#` register already answered"
	v.InFunction("getaltfname", func(v *graph.Verbs) {
		v.Muted(func(v *graph.Verbs) {
			v.FoldAlwaysAt("(== (call buflist_name_nr 0 (addr fname) (addr dummy)) FAIL)", 1, false, alt)
			v.Cut("(return fname)", 1, alt)
			v.Cut("(def fname (ptr char_u))", 1, alt)
			v.Cut("(def dummy linenr_T)", 1, alt)
		})
	})
	v.Say(alt)
	v.InFunction("fileinfo", func(v *graph.Verbs) {
		v.FoldAlwaysElse("(!= name nullptr)", 1,
			"fileinfo: buf_spname() never returns nullptr now, so CTRL-G "+
				"prints the special name and never a path -- the else arm, "+
				"which read b_fname and b_ffname, cannot be entered")
	})
	keepThenReturn("buf_spname", "(== (-> buf b_fname) nullptr)", "(return nullptr)",
		"buf_spname: `b_fname == nullptr` is TRUE, so it answers for every "+
			"buffer and can no longer return nullptr")
	keepThenReturn("buf_get_fname", "(== (-> buf b_fname) nullptr)", "(return (-> buf b_fname))",
		"buf_get_fname: the same, and \"[No Name]\" is now the only name the "+
			"editor has for a buffer")
	keepThenReturn("check_fname", "(== (-> curbuf b_ffname) nullptr)", "(return OK)",
		"check_fname: E32 for every buffer, and it stays because the `%` "+
			"register still asks it")
	v.InFunction("shorten_buf_fname", func(v *graph.Verbs) {
		v.FoldNeverAt(graph.IfNotArm,
			"(&& (!= (-> buf b_fname) nullptr) (! (call path_with_url (-> buf b_fname))) (|| force (== (-> buf b_sfname) nullptr) (call mch_isFullName (-> buf b_sfname))))", 1,
			"shorten_buf_fname: `b_fname != nullptr` is FALSE, so there is no path to "+
				"shorten and the function has nothing left to do")
	})
	// in the one statement pat finds in fn, the node sub matches is nullptr
	nullOf := func(fn, pat, sub, what string) {
		v.InFunction(fn, func(v *graph.Verbs) {
			if x := v.One(pat, what); x != nil {
				v.Muted(func(v *graph.Verbs) {
					v.In(x, func(v *graph.Verbs) {
						v.RewriteFunc(sub, 1, func(*graph.Node, graph.Bindings) ([]*graph.Node, error) {
							return []*graph.Node{graph.NewAtom("nullptr")}, nil
						}, what)
					})
				})
			}
		})
		v.Say(what)
	}
	nullOf("file_name_at_cursor",
		"(return (call file_name_in_line (call ml_get_curline) (. (-> curwin w_cursor) col) options count (-> curbuf b_ffname) file_lnum))",
		"(-> curbuf b_ffname)",
		"file_name_at_cursor: `curbuf->b_ffname` is the nullptr it passes now")
	v.InFunction("set_b0_fname", func(v *graph.Verbs) {
		v.FoldAlwaysElse("(== (-> buf b_ffname) nullptr)", 1,
			"set_b0_fname: `b_ffname == nullptr` is TRUE, so block zero's "+
				"file name is empty -- and the stat() in the arm that goes is "+
				"one of the two this phase takes")
	})
	nullOf("get_spec_reg", "(= (deref argp) (-> curbuf b_fname))", "(-> curbuf b_fname)",
		"get_spec_reg: the `%` register is `b_fname`, which is nullptr -- the "+
			"register already yielded nothing, and check_fname() above it still "+
			"says E32")
	v.InFunction("ex_display", func(v *graph.Verbs) {
		v.FoldNeverAt(graph.IfNotArm,
			"(&& (!= (-> curbuf b_fname) nullptr) (|| (== arg nullptr) (!= (call vim_strchr arg '%') nullptr)) (! got_int) (! (call message_filtered (-> curbuf b_fname))))", 1,
			"ex_display: the `\"%` line of :registers needs a buffer name and there is "+
				"none -- it was already never printed")
		v.DropIf("(&& (|| (== arg nullptr) (!= (call vim_strchr arg '#') nullptr)) (! got_int))", 1,
			"ex_display: and the `\"#` block goes whole, because buflist_name_nr() "+
				"inside it is FAIL ALWAYS and the block holds nothing else -- which is "+
				"what makes that function uncalled and the collection's")
	})
	v.InFunction("get_trans_bufname", func(v *graph.Verbs) {
		v.FoldAlwaysElse("(!= (call buf_spname buf) nullptr)", 1,
			"get_trans_bufname: buf_spname() is non-nullptr, so every window "+
				"and every :ls row reads \"[No Name]\" -- as they already did")
	})
	if v.Failed() {
		return v.Done()
	}
	t = v.Text()
	mentionsAre(t, "buflist_name_nr", 1, "buflist_name_nr has %d mentions after both callers were folded, expected "+
		"1 -- its definition, for the collection")

	var left []string
	for _, r := range w32AnyRow.FindAllString(string(t), -1) {
		if strings.Contains(r, "EX_XFILE") {
			left = append(left, r)
		}
	}
	if len(left) > 0 {
		v.Die("%d cmdnames[] rows still carry EX_XFILE, so the fold below would be a "+
			"guess: %s", len(left), edit.CoreHead(left[0], 60))
		return v.Done()
	}
	v.Say("no cmdnames[] row carries EX_XFILE any more -- :file was the last, as :read " +
		"was EX_ARGOPT's in phase 29 and the :edit family in phase 30")
	v.InFunction("do_one_cmd", func(v *graph.Verbs) {
		v.FoldNeverAt(graph.IfNotArm,
			"(&& (paren (& (. ea argt) EX_XFILE)) (== (call expand_filename (addr ea) cmdlinep (addr errormsg)) FAIL))", 1,
			"do_one_cmd's expand_filename() call: `ea.argt & EX_XFILE` is 0 for every "+
				"command, so this is the anchor the collection reads the whole "+
				"filename-expansion layer from")
	})
	v.InFunction("separate_nextcmd", func(v *graph.Verbs) {
		v.RewriteFunc("(| EX_CTRLV EX_XFILE)", 1,
			func(m *graph.Node, _ graph.Bindings) ([]*graph.Node, error) {
				return []*graph.Node{m.Kids[1]}, nil
			},
			"separate_nextcmd's CTRL-V test: the EX_XFILE disjunct is 0 for every "+
				"row, and dropping it is what takes the enumerator to zero mentions")
	})
	if v.Failed() {
		return v.Done()
	}
	mentionsAre(v.Text(), "EX_XFILE", 1, "EX_XFILE has %d mentions, expected 1 -- its own definition, for the collection")

	v.InFunction("buflist_new", func(v *graph.Verbs) {
		v.Cut("(= (-> buf b_dev_valid) false)", 1,
			"b_dev_valid's one surviving assignment, which part B left: every reader "+
				"is inside a function the collection takes, and deadfields.py cannot remove a "+
				"field that is still written")
	})
	v.InFunction("shorten_fnames", func(v *graph.Verbs) {
		v.CutRun("shorten_fnames: the cwd, and the call to a function with an empty body",
			"(def dirname (array PATH_MAX char_u))",
			"(def buf (ptr buf_T))",
			"(call mch_dirname dirname PATH_MAX)",
			"(= buf curbuf)",
			"(call shorten_buf_fname buf dirname force)")
	})
	v.Say("and its prototype takes void, because an unused PARAMETER is what " +
		"tools/sweep.sh's -Wno-unused-parameter cannot see -- phase 31's " +
		"anchor 4 measured that")
	v.Say("the definition with it")
	if !v.Failed() {
		if _, err := e.DropParam("shorten_fnames", "force", graph.ParamOptions{}); err != nil {
			v.Die("and its one call site -- %v", err)
		}
	}
	v.Say("and its one call site")

	v.InFunction("find_file_name_in_path", func(v *graph.Verbs) {
		v.FoldNeverAt(graph.IfNotArm, "(& options FNAME_EXP)", 1,
			"find_file_name_in_path: the `path` search arm goes, so CTRL-F and CTRL-P "+
				"both extract the word under the cursor and neither consults a disk -- "+
				"this is the fold that frees stat()")
	})
	if v.Failed() {
		return v.Done()
	}

	t = v.Text()
	offs = nil
	writes := 0
	for _, fld := range w32Fields {
		offs = append(offs, uses(t, fld)...)
		writes += len(assignments(t, fld))
	}
	got = functionsHolding(t, offs, w32Readers)
	if v.Failed() {
		return v.Done()
	}
	want = append([]string{}, w32Readers...)
	sort.Strings(want)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		v.Die("the surviving mentions of the three fields are in %s, expected exactly %s",
			strings.Join(got, " "), strings.Join(want, " "))
		return v.Done()
	}
	v.Sayf("%d mentions of b_ffname, b_sfname and b_fname are left, %d of them writes, and "+
		"every one is inside setfname, rename_buffer, otherfile_buf, buf_setino, "+
		"eval_vars or buflist_name_nr -- none of which has a caller the collection can "+
		"reach", len(offs), writes)

	for _, name := range edit.SortedKeys(w32After) {
		if k := edit.MentionCount(t, name); k != w32After[name] {
			v.Die("%s has %d mentions after the cut, expected %d", name, k, w32After[name])
			return v.Done()
		}
	}
	v.Say("the cut is done: CMD_file 0, EX_XFILE 1 (its own definition), buflist_name_nr " +
		"1, readonlymode 0 -- and read_cmd_fd 12, vim_fsync 3 and scriptin 8 " +
		"untouched, each of them a later phase's")
	return v.Done()
}
