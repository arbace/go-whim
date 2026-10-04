package p004f

// Whim phase 4f (formerly 59) -- no command-line completion.  See GOAL.md.
//
// The command line no longer completes anything.  In getcmdline_int() the
// 'wildchar' and 'wildcharm' keys, S-Tab, CTRL-D (list), CTRL-A (insert all),
// CTRL-L (longest match) and CTRL-N/CTRL-P over matches go; each of those keys is
// now an ordinary character, and CTRL-N/CTRL-P browse history as they did with no
// matches.  CTRL-L still adds a character to an incremental search.
//
// What completion shared with filename expansion stays: expand_filename() ->
// ExpandOne() with EXPAND_FILES, and the argument list through expand_wildcards().  So ExpandFromContext() keeps its file branch and loses the
// rest -- options, mappings, buffers, highlight groups, ++opt, every command's
// argument completion -- and ExpandOne() keeps the one mode its last caller asks
// for.  The sweep takes set_one_cmd_context() and everything under it.
//
// The six wild* options go: 'wildchar', 'wildcharm', 'wildmode', 'wildoptions',
// 'wildignore' and 'wildignorecase'.  The last two were read by globbing too, and
// fold as empty and off.
//
// THE DELTA: none the harnesses record.  The probes check the options are unknown
// and that `:e` still edits a named file.

// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3a): the text version's heads,
// lines and literals are acts on the nodes -- ifs dropped and folded, case
// runs dropped, runs of items cut, conditions rewritten from their own
// operands, the options[] rows' callbacks pointed at nullptr -- each
// counted, its report the text's (history keeps it).

import (
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// key is vim's key code for a two-byte termcap name, as a form: the text's
// vimtext.Key.
func key(a, b string) string {
	return "(paren (- (+ (paren '" + a + "') (<< (cast int (paren '" + b + "')) 8))))"
}

// kex is K_SPECIAL's code for an extra key, as a form.
func kex(k string) string {
	return "(paren (- (+ (paren KS_EXTRA) (<< (cast int (paren " + k + ")) 8))))"
}

// valueCompletion is an options[] row's value-completion callback.
var valueCompletion = regexp.MustCompile(`^expand_set_\w+$`)

// Whim4f takes command-line completion: the wildcard machinery in
// getcmdline_int, every options[] row's value-completion callback, and every
// context ExpandFromContext knew but files.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("nocompletion", e, w)

	v.InFunction("getcmdline_int", func(v *graph.Verbs) {
		v.DropIf("(&& (. ccline cmdbuff_replaced) (> (. xpc xp_numfiles) 0))", 1,
			"a replaced command line freeing its matches")
		v.DropIf("(&& (== c "+kex("KE_WILD")+") did_hist_navigate)", 1,
			"a wildcard trigger after history navigation")
		v.Cut("(= did_hist_navigate TRUE)", 1,
			"history navigation remembered for the wildcard trigger")
		v.DropIf("(&& (!= c p_wc) (== c "+key("k", "B")+") (> (. xpc xp_numfiles) 0))", 1,
			"S-Tab stepping back through matches")
		v.DropIf("(&& (paren did_wild_list) (! key_is_wc) (> (. xpc xp_numfiles) 0))", 1,
			"CTRL-E and CTRL-Y over a match list")
		v.DropIf("(&& (|| (== c ESC) (== c Ctrl_C)) (paren (& (index wim_flags 0) WIM_LIST)))", 1,
			"leaving a 'wildmode' list clearing 'hlsearch'")
		v.Cut("(= end_wildmenu (paren _))", 1, "deciding the match list ends")
		v.DropIf("end_wildmenu", 1, "ending the match list")
		v.DropIf("(|| (paren (&& (== c p_wc) (! gotesc) KeyTyped)) (== c p_wcm) (== c "+kex("KE_WILD")+"))", 1,
			"completing on 'wildchar', 'wildcharm' or the wildcard trigger")
		v.DropIf("(&& (== c "+key("k", "B")+") KeyTyped)", 1, "S-Tab completing backwards")
		v.One("(if (== (call showmatches (addr xpc) TRUE) EXPAND_NOTHING) (block (break)))", "CTRL-D listing matches")
		v.DropCaseRun("(case Ctrl_D)", 1, "CTRL-D listing matches")
		v.One("(if (== (call nextwild (addr xpc) WILD_ALL 0 (!= firstc '@')) FAIL) (block (break)))", "CTRL-A inserting every match")
		v.DropCaseRun("(case Ctrl_A)", 1, "CTRL-A inserting every match")
		if r := v.Run("CTRL-L completing the longest match",
			"(if (== (call nextwild (addr xpc) WILD_LONGEST 0 (!= firstc '@')) FAIL) (block (break)))",
			"(goto cmdline_changed)"); r != nil {
			if err := v.Editor().ReplaceRun(r[0], r[1], graph.Break()); err != nil {
				v.Die("CTRL-L completing the longest match -- %v", err)
			} else {
				v.Say("CTRL-L completing the longest match")
			}
		}
		v.DropIf("(> (. xpc xp_numfiles) 0)", 1, "CTRL-N and CTRL-P stepping through matches")
		if r := v.Run("leaving the command line resetting the match list",
			"(label returncmd)", "_", "(= did_wild_list FALSE)", "(= wim_index 0)"); r != nil {
			if err := v.Editor().ReplaceRun(r[2], r[3]); err != nil {
				v.Die("leaving the command line resetting the match list -- %v", err)
			} else {
				v.Say("leaving the command line resetting the match list")
			}
		}
		v.RewriteAt("(call may_trigger_safestate ?x)", "x", "TRUE", 1, "SafeState not waiting on a match list")
		v.Rewrite("(&& (== (. xpc xp_context) EXPAND_NOTHING) ?r)", "?r", 1,
			"incremental search not waiting on a completion context")
	})

	// Each options[] row names a callback that completes its value, called only
	// by :set completion, which is gone -- but the table keeps them reachable,
	// and with them ExpandGeneric() and the fuzzy matcher.  The row's callback
	// becomes NULL.  The count is a FLOOR rather than a number: how many rows
	// carry one is a fact about a table other phases are also cutting, and a
	// floor refuses the case this guards against -- a pattern that has stopped
	// matching.  A callback is an element of a row followed by another, as
	// the text's `expand_set_\w+,` was.
	v.InTable("options", func(v *graph.Verbs) {
		var cbs []*graph.Node
		for _, r := range v.Rows() {
			args := r.Args()
			for i, a := range args {
				if i < len(args)-1 && !a.IsList() && valueCompletion.MatchString(a.Atom) {
					cbs = append(cbs, a)
				}
			}
		}
		k := len(cbs)
		v.Expect(k >= 20, "options[] names %d value-completion callbacks, expected many", k)
		for _, a := range cbs {
			if v.Failed() {
				break
			}
			if err := v.Editor().Replace(a, graph.NewAtom("nullptr")); err != nil {
				v.Die("options[] -- %v", err)
			}
		}
		if !v.Failed() {
			v.Say(fmt.Sprintf("options[] no longer names a value-completion callback (%d rows)", k))
		}
	})

	v.InFunction("ExpandFromContext", func(v *graph.Verbs) {
		// The non-file contexts are one run, from the empty-match assignment to
		// the function's last `return ret;`.  It is cut by its ends rather than
		// by a pattern over the whole span: the run is hundreds of lines of
		// unrelated cases, and a pattern that matched all of them would match
		// anything.
		v.SpliceFirst(`(= (deref matches) (cast (ptr (ptr char_u)) ""))`, "(return ret)", "",
			"every completion context but files")
		ctx := func(c string) string { return "(== (-> xp xp_context) " + c + ")" }
		v.FoldAlways("(|| "+ctx("EXPAND_FILES")+" "+ctx("EXPAND_DIRECTORIES")+" "+ctx("EXPAND_FILES_IN_PATH")+" "+
			ctx("EXPAND_FINDFUNC")+" "+ctx("EXPAND_DIRS_IN_CDPATH")+")", 1,
			"file expansion is the only context")
	})

	v.InFunction("ExpandOne", func(v *graph.Verbs) {
		v.FoldNever("(|| (== mode WILD_NEXT) (== mode WILD_PREV) (== mode WILD_PAGEUP) (== mode WILD_PAGEDOWN))", 1,
			"ExpandOne stepping through matches")
		v.FoldNever("(== mode WILD_CANCEL)", 1, "ExpandOne cancelling or applying a match")
		v.FoldNever("(== mode WILD_FREE)", 1, "ExpandOne only freeing")
		v.FoldNever("(&& (== mode WILD_LONGEST) (> (-> xp xp_numfiles) 0))", 1, "ExpandOne finding the longest match")
		v.FoldNever("(&& (== mode WILD_ALL) (> (-> xp xp_numfiles) 0) (! got_int))", 1, "ExpandOne joining every match")
		v.FoldAlways("(|| (== mode WILD_EXPAND_FREE) (== mode WILD_ALL))", 1, "ExpandOne always cleaning up after expanding")
	})

	v.InFunction("didset_options2", func(v *graph.Verbs) {
		v.Cut("(call check_opt_wim)", 1, "startup parsing 'wildmode' into flags nothing reads")
	})
	// 'wildignorecase''s test in expand_filename: p_wic is never written once
	// its row is dropped, and the fall-out closure took it at phase 1 (D3)
	v.InFunction("expand_wildcards", func(v *graph.Verbs) {
		v.DropIf("(deref p_wig)", 1, "'wildignore' in filename globbing")
	})
	wc := "(== (cast (ptr long) varp) (addr p_wc))"
	wcm := "(== (cast (ptr long) varp) (addr p_wcm))"
	v.InFunction("do_set_option_numeric", func(v *graph.Verbs) {
		v.FoldNever("(&& (|| "+wc+" "+wcm+") _)", 1, ":set wc= accepting a key name")
	})
	v.InFunction("wc_use_keyname", func(v *graph.Verbs) {
		v.FoldNever("(|| (paren "+wc+") (paren "+wcm+"))", 1, ":set wc? showing a key name")
	})
	return v.Done()
}

func init() { phase.RegisterGraph("whim4f", Edit) }
