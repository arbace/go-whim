package cut

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/crefactor/graph"
)

// Not written into the C any more -- the canonical form has no comments --
// and kept as the account of this cut, for whoever reads the program.
//
//lint:ignore U1000 the account of this cut, kept for its reader; the canonical C has no comments to carry it
const relativeTimeNote = `// How long ago, not when.  Phase 9 took away every way this editor could
// be told what zone the clock is in, and undo history does not outlive the
// process -- :wundo and :rundo are ex_ni -- so every time this formats is
// within one session, which is exactly what "ago" measures.
`

const relativeTime = `    long seconds = (long)(vim_time() - tt);

    vim_snprintf((char *)buf, buflen, NGETTEXT("%ld second ago", "%ld seconds ago", seconds), seconds);`

// ifLines is the text's count of an if on the C view of its function: the
// lines from the one head begins through its block's closing brace.
func ifLines(fn []byte, head string) (int, error) {
	m := regexp.MustCompile(edit.Head(head)).FindIndex(fn)
	if m == nil {
		return 0, fmt.Errorf("no match for %s", edit.PyRepr(head))
	}
	b := edit.Blank(fn)
	k := bytes.LastIndexByte(fn[:m[0]], '\n') + 1
	o := m[1] + bytes.IndexByte(b[m[1]:], '{')
	c := edit.Match(b, o)
	if c < 0 {
		return 0, fmt.Errorf("unbalanced block for %s", edit.PyRepr(head))
	}
	return bytes.Count(fn[k:c], []byte{'\n'}) + 1, nil
}

// NoRecover takes recovery away: nothing can set recoverymode any more.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): a DeleteDefinition, the folds
// by their conditions (FoldNever for the arm whose else is kept, FoldAlways
// for the two that always ran, refused where one has an else, DropIf), an
// operand dropped, add_time's body FRAG; the line counts the text's on the
// C view (history keeps the text version).
func NoRecover(e *graph.Editor, w io.Writer) error {
	// -r and -L, the only two things that set recoverymode, went with the
	// command line (argvfront, the reform's D1)
	v := graph.NewVerbs("norecover", e, w)

	// recover_names is the one reader of p_dir left, and dropoptions --strict
	// refuses 'directory' later in this phase with no sweep between; the
	// other two are the sweep's.
	if e.Defn("recover_names") == nil {
		v.Die("recover_names is not defined at file scope")
		return v.Done()
	}
	// its lines on its own C view: the file's count would take the blank line
	// the print puts between forms too
	lines := 0
	v.InFunction("recover_names", func(v *graph.Verbs) { lines = bytes.Count(v.Text(), []byte{'\n'}) })
	q := graph.NewVerbs("norecover", e, io.Discard)
	q.DeleteDefinition("recover_names", "recover_names")
	if err := q.Done(); err != nil {
		return err
	}
	v.Sayf("recover_names, %d lines", lines)

	q = graph.NewVerbs("norecover", e, io.Discard)
	q.DropIf("(&& recoverymode (== (. params fname) nullptr))", 2, "the -r arms")
	if err := q.Done(); err != nil {
		return err
	}
	v.Say("the two `-r with no file` arms of main and vim_main2")

	// vim_main2's stdin arm read edit_type, which nothing writes once the
	// command line is cut: the fall-out closure took it (argvfront, D1)
	foldCounted := func(fn, head, cond string, never bool, say string) {
		if v.Failed() {
			return
		}
		n := 0
		v.InFunction(fn, func(v *graph.Verbs) {
			var err error
			if n, err = ifLines(v.Text(), head); err != nil {
				v.Die("%v", err)
				return
			}
			ifs := v.Find("(if " + cond + " _*)")
			if len(ifs) == 1 && !never && len(ifs[0].Kids) > 3 {
				v.Die("%s -- the block has an else", say)
				return
			}
			q := graph.NewVerbs("norecover", e, io.Discard)
			q.In(v.Scope(), func(q *graph.Verbs) {
				if never {
					q.FoldNever(cond, 1, say)
				} else {
					q.FoldAlways(cond, 1, say)
				}
			})
			if q.Err != nil {
				v.Die("%v", q.Err)
			}
		})
		if v.Failed() {
			return
		}
		if never {
			v.Sayf("%s, and the %d lines it guarded", say, n)
		} else {
			v.Sayf("%s, %d lines that always ran", say, n)
		}
	}
	foldCounted("create_windows", "if (recoverymode)", "recoverymode", true,
		"the recovery arm of create_windows")

	// readfile() had to know whether it was filling a buffer from a swap file
	// rather than from the file itself.  It never is.
	v.InFunction("readfile", func(v *graph.Verbs) {
		c := v.Find("(&& (! recoverymode) (! filtering) (! (& flags READ_DUMMY)))")
		if len(c) != 1 {
			v.Die("readfile's `reading from stdin` message is not where this expects")
			return
		}
		q := graph.NewVerbs("norecover", e, io.Discard)
		q.In(c[0], func(q *graph.Verbs) { q.DropOperand("(! recoverymode)", 1, "readfile's message") })
		if q.Err != nil {
			v.Die("%v", q.Err)
		}
	})
	foldCounted("readfile", "if (!recoverymode)", "(! recoverymode)", false,
		"readfile's redraw and line count")
	foldCounted("readfile", "if (!(recoverymode && error))", "(! (&& recoverymode error))", false,
		"readfile's return value")

	if !v.Failed() && e.Defn("add_time") == nil {
		v.Die("add_time is not defined at file scope")
	}
	if v.Failed() {
		return v.Done()
	}
	q = graph.NewVerbs("norecover", e, io.Discard)
	q.BodyC("add_time", relativeTime, "add_time")
	if err := q.Done(); err != nil {
		return err
	}
	v.Say("add_time says how long ago, not when; localtime_r and " +
		"strftime go with it")

	v.Sayf("%d recoverymode mentions left for the sweep", v.TextCount(`\brecoverymode\b`))
	return v.Done()
}
