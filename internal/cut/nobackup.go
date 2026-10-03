package cut

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/graph"
)

// backupWord counts what is left for the sweep, on the C view.
var backupWord = regexp.MustCompile(`\bbackup\b`)

// NoBackup takes the backup file away.
//
// buf_write's five backup blocks go whole, the owner carried to the backup
// collapses to the branch that now always runs, the tests that asked whether
// a backup happened lose that question, and the locals and the ACL calls
// (stubs in this build) that drove them go after them.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B1a): each act is the text
// version's (history keeps it), on buf_write's graph, counted; a block's
// lines are the lines its deletion takes from the function's C view, which
// are the lines the text cut.  ONE ACT REPRODUCES AN ACCIDENT OF THE TEXT,
// on purpose, since the snapshots hold it: the text cut the lines `{`,
// `acl = mch_get_acl(fname);`, `}` and left `if (!newfile)` without its
// block, so that once the two statements after it went too, the if took
// `prev_got_int = got_int;` as its body.  The graph deletes the call and,
// where the text's if took its new body, moves that statement into the
// emptied block: the same program, said as a move.  The statement's local is
// set at its declaration too, so the if decides nothing a reader sees but
// the value of got_int at the declaration against its value there.
func NoBackup(e *graph.Editor, w io.Writer) error {
	// the acts are quiet: the text reported a line for a group of them
	v := graph.NewVerbs("nobackup", e, io.Discard)
	say := func(format string, a ...any) {
		if !v.Failed() {
			fmt.Fprintf(w, "  %-13s%s\n", "nobackup", fmt.Sprintf(format, a...))
		}
	}
	total := 0
	// lines is the lines act takes from buf_write's C view.
	lines := func(act func(v *graph.Verbs)) int {
		n := 0
		v.InFunction("buf_write", func(v *graph.Verbs) {
			before := bytes.Count(v.Text(), []byte{'\n'})
			act(v)
			if !v.Failed() {
				n = before - bytes.Count(v.Text(), []byte{'\n'})
			}
		})
		return n
	}
	block := func(cond, what string) int {
		n := lines(func(v *graph.Verbs) {
			s := v.One(cond, what)
			if s != nil && len(s.Kids) > 3 {
				v.Die("%s -- has an else", what)
				return
			}
			if s != nil {
				if err := v.Editor().Delete(s); err != nil {
					v.Die("%s -- %v", what, err)
				}
			}
		})
		say("%-46s %4d lines", what, n)
		return n
	}
	for _, b := range []struct{ cond, what string }{
		{"(if (&& (! (&& append (== (deref p_pm) NUL))) (! filtering) (>= perm 0) dobackup) _*)", "the backup itself"},
		{"(if (&& (deref p_pm) dobackup) _*)", "'patchmode'"},
		{"(if (!= backup nullptr) _*)", "the backup kept or removed after the write"},
		{"(if (&& (! p_bk) (!= backup nullptr) (! (. write_info bw_conv_error)) _) _*)", "the backup deleted when 'backup' is off"},
	} {
		total += block(b.cond, b.what)
	}

	// The owner carried onto the written file when the backup was a rename.
	// Its `else if` is the branch that now always runs, so the pair collapses
	// to that rather than going -- buf_setino() still has to happen.  The text
	// cut through the arm's `if (...)`, so what is left is its block, bare.
	v.InFunction("buf_write", func(v *graph.Verbs) {
		v.Rewrite("(if (&& (!= backup nullptr) (! backup_copy)) _ (if (! (-> buf b_dev_valid)) ?b))", "?b", 1,
			"the owner carried to the backup")
	})
	say("%-46s %4d lines", "the owner carried to the backup", 13)

	total += block("(if (&& (!= backup nullptr) (== wfname fname)) _*)", "the roll-back to a backup")

	v.InFunction("buf_write", func(v *graph.Verbs) {
		v.DropOperand("(! (&& exiting (!= backup nullptr)))", 1, "the `written a backup while exiting` test")
		v.Rewrite("(|| ?a dobackup)", "?a", 1, "the conversion test")
		v.DropOperand("(|| (! dobackup) backup_copy)", 1, "the permissions test")
		v.FoldAlways("(if (! backup_copy) _*)", 1, "the ACL put back")
	})
	say("thirteen tests that asked whether a backup happened")

	// get_bkc_flags reads b_p_bkc, and droplocal b_p_bkc runs later in this
	// phase with no sweep between; vim_rename, vim_copyfile and set_file_time
	// are the sweep's.
	v.DeleteDefinition("get_bkc_flags", "get_bkc_flags")
	// The three ACL functions, their declarations, the acl local and the
	// vim_acl_T type are the sweep's once these calls are gone.
	v.InFunction("buf_write", func(v *graph.Verbs) {
		v.Cut("(= acl (call mch_get_acl fname))", 1, "buf_write's mch_get_acl")
		v.Cut("(call mch_set_acl wfname acl)", 1, "the ACL put back")
		v.Cut("(call mch_free_acl acl)", 1, "buf_write's mch_free_acl")
	})
	say("the ACL calls, which were stubs in this build")

	// set_init_default_backupskip() builds 'backupskip' from /tmp at startup
	// and looks the row up BY NAME -- the lookup that returns -1 for a row
	// that is not there, is not checked, and indexes options[-1].
	// (its call first: it has no prototype, so the call refers to the
	// definition, which DeleteDefinition refuses while it is there)
	v.Cut("(call set_init_default_backupskip)", 1, "its call")
	v.DeleteDefinition("set_init_default_backupskip", "set_init_default_backupskip")
	say("the 'backupskip' default, built at startup by name")

	// The three handlers 'backupcopy', 'backupext' and 'patchmode' kept
	// reachable went with their rows, dropped at phase 1 (optfront, D3).
	v.Cut("(cast void (call opt_strings_flags p_bkc p_bkc_values (addr bkc_flags) TRUE))", 1,
		"didset_string_options' p_bkc line")
	say("didset_string_options stops reading 'backupcopy'")

	v.DropIf("(&& (== (deref arg) '>') (== varp (cast (ptr char_u) (addr p_bdir))))", 1,
		"the `is this option a directory?` test")
	say("the `is this option a directory?` test")
	// the directory-list test folds at phase 2 (whim18, the reform's D6)

	v.InFunction("buf_write", func(v *graph.Verbs) {
		v.Cut("(def bkc (unsigned int) (call get_bkc_flags buf))", 1, "bkc")
		v.Cut("(= dobackup (paren (|| p_wb p_bk (!= (deref p_pm) NUL))))", 1, "its one assignment")
		v.Cut("(call vim_free backup)", 1, "the free of backup")
		v.DropIf("(&& dobackup (!= (deref p_bsk) NUL) (call match_file_list p_bsk sfname ffname))", 1,
			"the 'backupskip' test")
		// the text's accident, above: the if its cut left bodiless takes the
		// statement that now follows it
		v.Splice("(if (! newfile) (block))", "(= prev_got_int ?g)", "(if (! newfile) (block (= prev_got_int ?g)))",
			"the if the text left bodiless")
	})
	say("six locals and the assignment that drove them")

	if !v.Failed() {
		say("%d lines of backup machinery; %d mentions left for the sweep",
			total, len(backupWord.FindAll(v.Text(), -1)))
	}
	return v.Done()
}
