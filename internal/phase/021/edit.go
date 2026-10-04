package p021

// Whim phase 21 (formerly 69) -- one file argument, and no argument list.  See GOAL.md.
//
// THE ORDER MATTERS, and it is the opposite of the obvious one.  An earlier
// attempt at "one buffer" imposed reuse inside buflist_new() and deleted the
// argument-list call that reaches it -- and that call is THE ONLY THING THAT NAMES
// THE FIRST BUFFER.  open_buffer() reads through `readfile(curbuf->b_ffname, ...)`,
// so with no name it read nothing: the buffer came up empty, every edit was a
// silent no-op, and :wq wrote the original bytes back.  It built and it passed two
// of three probes.
//
// So this phase limits the command line FIRST and keeps the naming path exactly as
// it is:
//
// ONE FILE ARGUMENT.  A second non-option argument is mainerr(ME_TOO_MANY_ARGS),
// which is what vim already answers for a second `-`.  One file means one
// entry, which is what makes the list pointless rather than merely unused.
// THE NAME STILL GOES THROUGH buflist_add().  curbuf exists and is unnamed and
// empty when command_line_scan() runs -- main() calls common_init_2(), which
// calls win_alloc_first(), before the scan -- so buflist_new() reuses it and
// sets b_ffname, exactly as today.  Only the LIST around that call goes.
// THE ARGUMENT LIST.  :next and :previous point at ex_ni; the other 21 argument
// commands already do.  ex_next(), ex_previous(), do_argfile(), do_arglist(),
// arglist_del_files(), alist_set(), alist_clear(), alist_add(), alist_name(),
// editing_arg_idx(), check_arglist_locked(), arg_had_last, global_alist,
// alist_T, aentry_T, w_alist, w_arg_idx, w_arg_idx_invalid and mparm_T.fname
// go with them, by fold or by sweep.
//
// WHAT FOLDS BECAUSE THE COUNT IS ALWAYS ONE: check_more(), whose "N more files to
// edit" refusal can never fire; append_arg_number(), the "(N of M)" suffix in
// :file; and the four ADDR_ARGUMENTS arms of Ex range parsing.
//
// THE DELTA: NONE, which was measured rather than assumed.  :next and :previous
// were declared as moving and did not: an exsweep row is `exit= left= err=`, and
// with one file argument do_argfile() already answered "there is only one file to
// edit" -- so pointing the rows at ex_ni changes the message text, which the sweep
// does not record, while the exit status, the files touched and stderr all stay the
// same.  The declaration is narrowed to match the measurement; widening one to fit
// is what the delta check exists to refuse.
//
// The probes are LOAD-FIRST -- the first one proves the buffer holds the file's
// lines, because that is the check the earlier attempt did not have and needed.

// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3a): the bodies and the ADDR_*
// arms are the text's literals made nodes in one unit (FRAG: BodyC,
// LiteralC, Together; the arms at two depths one literal, spacing aside),
// the ## expansion two values rebuilt (BUILD), runs of items and the
// member cut; each counted, its report the text's (history keeps the text
// version).

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

// alistLen is `ALIST_LEN(&global_alist)` as the expansion left it.
const alistLen = "(paren (. global_alist al_ga ga_len))"

// Whim21 makes the argument list one file: the second file argument goes, :next
// and :previous become ex_ni, every ADDR_ARGUMENTS arm becomes a constant, and
// the list type itself follows.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("onearg", e, w)

	// 1. a second file argument: the command line's own, gone with it
	// (argvfront, the reform's D1)

	// 2. :next and :previous point at ex_ni from phase 1 (exfront, the
	// reform's D2)

	// 3. the readers that can only ever see one
	//
	// EVERY command that uses ADDR_ARGUMENTS -- argadd, argdelete, argdo,
	// argedit, argument, sargument -- is already ex_ni, so no live command
	// reaches these arms.  But THE LABELS MUST STAY: these switches enumerate
	// ADDR_* exhaustively, and deleting one only earns "enumeration value
	// 'ADDR_ARGUMENTS' not handled in switch" -- seven of them, which is what
	// the sweep kept reporting as "left alone" and could never converge on.  So
	// each arm gets a constant Body instead.
	//
	// THE TWO ARMS IN get_address ARE AT TWO DEPTHS, which a literal of items
	// does not see: spacing aside, one literal is both.
	v.Together(func(v *graph.Verbs) {
		v.BodyC("check_more", w21lit1, "check_more refusing to quit with files left to edit")
		v.BodyC("append_arg_number", w21lit2, "the (N of M) suffix on the file message")
		v.InFunction("parse_cmd_address", func(v *graph.Verbs) {
			v.LiteralC(w21lit3, w21lit4, 1, "an argument range in a command line")
		})
		v.InFunction("address_default_all", func(v *graph.Verbs) {
			v.LiteralC(w21lit5, w21lit6, 1, "an argument range with no range given")
		})
		v.InFunction("default_address", func(v *graph.Verbs) {
			v.LiteralC(w21lit7, w21lit8, 1, "the default line for an argument range")
		})
		v.InFunction("get_address", func(v *graph.Verbs) {
			v.LiteralC("case ADDR_ARGUMENTS: lnum = curwin->w_arg_idx + 1; break;",
				"case ADDR_ARGUMENTS:\n    lnum = 0;\n    break;\n", 2, "an argument range parsed from an address")
			v.LiteralC("case ADDR_ARGUMENTS: lnum = ((curwin)->w_alist->al_ga.ga_len); break;",
				"case ADDR_ARGUMENTS:\n    lnum = 0;\n    break;\n", 1, "the last line of an argument range")
		})
		v.InFunction("invalid_range", func(v *graph.Verbs) {
			v.LiteralC(w21lit12, w21lit13, 1, "an argument range checked for validity")
		})
	})

	// 3b. the readers with live callers.  check_arg_idx() is called from six
	// live functions, so its BODY folds and the calls go; editing_arg_idx() is
	// reached only from it.  arg_all() builds the ## expansion from every entry,
	// and now has none to build from.
	// three: the others died with commands retired or deleted at phase 1
	// (D2, D4), and buf_name_changed()'s with setfname()'s last caller once
	// phase 5b's body for create_windows() runs at phase 5, before this
	// phase now
	if !v.Failed() {
		n := v.Count("(call check_arg_idx curwin)") + v.Count("(call check_arg_idx win)")
		v.Expect(n == 3, "the three calls that revalidated the argument index -- matched %d times, expected 3", n)
	}
	v.Cut("(call check_arg_idx _)", 3, "the three calls that revalidated the argument index")
	v.InFunction("eval_vars", func(v *graph.Verbs) {
		const what = "## expanding to every file in the argument list"
		r := v.Run(what, "(= result (call arg_all))", "(= resultbuf result)")
		if r == nil {
			return
		}
		for i, val := range []string{`(cast (ptr char_u) "")`, "nullptr"} {
			with, err := e.Build(r[i], val, nil)
			if err == nil {
				err = e.Replace(r[i].Kids[2], with...)
			}
			if err != nil {
				v.Die("%s -- %v", what, err)
				return
			}
		}
		v.Say(what)
	})
	// win_init_some(), where a new window inherited the argument list, went
	// with the window split at phase 5 (whim5b and whim5c, which run before
	// this phase now)
	// create_windows() invalidated the index when a swap-file prompt quit;
	// its body is phase 5b's from phase 5 (whim5b, which runs before this
	// phase now)
	// The window's argument-list fields are named by nothing after this, and
	// the collection takes them.

	// main() takes the first entry's name into params.fname and never reads it:
	// the WHOLE guarded assignment goes, not just the line, or alist_name
	// outlives the list.
	v.Cut("(if (> "+alistLen+" 0) (block (= (. params fname) (call alist_name _))))", 1,
		"main taking the first argument as the file name")
	// mparm_T's field, and no other `char_u *fname;`: the member after argv.
	if td := v.One("(typedef mparm_T (struct (argc int) (argv (ptr (ptr char))) (fname (ptr char_u)) _*))",
		"mparm_T's unread fname"); td != nil {
		v.In(td, func(v *graph.Verbs) { v.Cut("(fname (ptr char_u))", 1, "mparm_T's unread fname") })
	}

	// The list itself: initialised at startup and pointed at by the one window.
	// These are the last two mentions, and without them alist_init,
	// global_alist, alist_T and aentry_T all lose their readers.
	v.InFunction("common_init_2", func(v *graph.Verbs) {
		v.CutRun("the argument list set up at startup", "(call alist_init (addr global_alist))", "(= (. global_alist id) 0)")
	})
	// the one window pointed at it in win_alloc_firstwin(), whose body is
	// phase 5b's from phase 5 (whim5b, which runs before this phase now)

	// and the count message, which one file argument can never satisfy
	v.Cut(fmt.Sprintf(`(if (&& (> %s 1) (! silent_mode)) (block (call printf (call _ "%%d files to edit\n") %s)))`, alistLen, alistLen), 1,
		"the \"N files to edit\" message at startup")
	return v.Done()
}

func init() { phase.RegisterGraph("whim21", Edit) }
