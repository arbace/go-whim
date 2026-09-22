package p069

// Whim phase 69 -- one file argument, and no argument list.  See GOAL.md.
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
// is what whimdelta.sh exists to refuse.
//
// The probes are LOAD-FIRST -- the first one proves the buffer holds the file's
// lines, because that is the check the earlier attempt did not have and needed.

import (
	"fmt"
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

// Whim69 makes the argument list one file: the second file argument goes, :next
// and :previous become ex_ni, every ADDR_ARGUMENTS arm becomes a constant, and
// the list type itself follows.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("onearg", text, w)

	// 1. one file argument, and the name still reaching curbuf
	e.InFunction("command_line_scan", func(e *edit.E) {
		e.Literal(w69OldArg, w69NewArg, "a second file argument, and the list that held them")
	})

	// 2. :next and :previous
	for _, c := range []struct{ Name, handler string }{{"next", "ex_next"}, {"previous", "ex_previous"}} {
		e.Sub(fmt.Sprintf(`(?m)^([ \t]*\[CMD_%s\] = \{\(char_u \*\)"%s", sizeof\("%s"\) - 1, )%s,`,
			c.Name, c.Name, c.Name, c.handler), "${1}ex_ni,", 1, fmt.Sprintf(":%s points at ex_ni", c.Name))
	}

	// 3. the readers that can only ever see one
	e.Body("check_more", w69lit1, "check_more refusing to quit with files left to edit")
	e.Body("append_arg_number", w69lit2, "the (N of M) suffix on the file message")

	// EVERY command that uses ADDR_ARGUMENTS -- argadd, argdelete, argdo,
	// argedit, argument, sargument -- is already ex_ni, so no live command
	// reaches these arms.  But THE LABELS MUST STAY: these switches enumerate
	// ADDR_* exhaustively, and deleting one only earns "enumeration value
	// 'ADDR_ARGUMENTS' not handled in switch" -- seven of them, which is what
	// the sweep kept reporting as "left alone" and could never converge on.  So
	// each arm gets a constant Body instead.
	arm := func(fn, old, new, what string, n int) {
		e.InFunction(fn, func(e *edit.E) { e.LiteralN(old, new, n, what) })
	}
	arm("parse_cmd_address", w69lit3, w69lit4, "an argument range in a command line", 1)
	arm("address_default_all", w69lit5, w69lit6, "an argument range with no range given", 1)
	arm("default_address", w69lit7, w69lit8, "the default line for an argument range", 1)
	arm("get_address", w69lit9, w69lit10, "an argument range parsed from an address", 2)
	arm("get_address", w69lit11, w69lit10, "the last line of an argument range", 1)
	arm("invalid_range", w69lit12, w69lit13, "an argument range checked for validity", 1)

	// 3b. the readers with live callers.  check_arg_idx() is called from six
	// live functions, so its BODY folds and the calls go; editing_arg_idx() is
	// reached only from it.  arg_all() builds the ## expansion from every entry,
	// and now has none to build from.
	e.Lines(`check_arg_idx\((?:win|curwin)\);`, 6, "the six calls that revalidated the argument index")
	e.InFunction("eval_vars", func(e *edit.E) {
		e.Literal(w69lit16, w69lit17, "## expanding to every file in the argument list")
	})
	e.InFunction("win_init_some", func(e *edit.E) {
		e.Cut(`(?m)^[ \t]*newp->w_alist = oldp->w_alist;\n[ \t]*\+\+newp->w_alist->al_refcount;\n[ \t]*newp->w_arg_idx = oldp->w_arg_idx;\n\n?`, 1,
			"a new window inheriting the argument list")
	})
	e.Lines(`curwin->w_arg_idx = -1;`, 1, "the index a swap-file quit invalidated")
	e.Cut(`(?m)^[ \t]*alist_T[ \t]+\*w_alist;\n[ \t]*int[ \t]+w_arg_idx;\n[ \t]*bool[ \t]+w_arg_idx_invalid;\n`, 1,
		"the window's argument-list fields")

	// main() takes the first entry's name into params.fname and never reads it:
	// the WHOLE guarded assignment goes, not just the line, or alist_name
	// outlives the list.
	e.Cut(`(?m)^[ \t]*if \( \(global_alist\.al_ga\.ga_len\)  > 0\)\n[ \t]*\{\n[ \t]*params\.fname = alist_name\(& \(\(aentry_T \*\)global_alist\.al_ga\.ga_data\) \[0\]\);\n[ \t]*\}\n`, 1,
		"main taking the first argument as the file name")
	e.Cut(`(?m)^[ \t]*char_u[ \t]+\*fname;\n\n`, 1, "mparm_T's unread fname")

	// The list itself: initialised at startup and pointed at by the one window.
	// These are the last two mentions, and without them alist_init,
	// global_alist, alist_T and aentry_T all lose their readers.
	e.InFunction("common_init_2", func(e *edit.E) {
		e.Cut(`(?m)^[ \t]*alist_init\(&global_alist\);\n[ \t]*global_alist\.id = 0;\n\n?`, 1, "the argument list set up at startup")
	})
	e.InFunction("win_alloc_firstwin", func(e *edit.E) {
		e.Lines(`curwin->w_alist = &global_alist;`, 1, "the one window pointing at it")
	})

	// The two types, matched in their INPUT shape.  An earlier version patterned
	// against `typedef struct arglist { int id; } alist_T;` -- which is what the
	// file looks like AFTER a sweep has stripped the fields, not before one --
	// and matched nothing.  Both are removed here rather than left to
	// typereach.py, which takes roots from mentions outside every type
	// definition and still leaves the typedef standing.
	e.Cut(`(?m)^typedef struct arglist\n\{\n[ \t]*garray_T[ \t]+al_ga;\n[ \t]*int[ \t]+al_refcount;\n[ \t]*int[ \t]+id;\n\} alist_T;\n\n?`, 1,
		"the argument list type itself")
	e.Cut(`(?m)^typedef struct argentry\n\{\n[ \t]*char_u[ \t]+\*ae_fname;\n[ \t]*int[ \t]+ae_fnum;\n\} aentry_T;\n\n?`, 1,
		"the argument entry type")

	// and the count message, which one file argument can never satisfy
	e.Cut(`(?m)^[ \t]*if \( \(global_alist\.al_ga\.ga_len\)  > 1 && !silent_mode\)\n[ \t]*\{\n[ \t]*printf\(_\("%d files to edit\\n"\),  \(global_alist\.al_ga\.ga_len\) \);\n[ \t]*\}\n\n?`, 1,
		"the \"N files to edit\" message at startup")
	return e.Done()
}

func init() { edit.Register("whim69", Edit) }
