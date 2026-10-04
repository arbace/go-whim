package cut

// argvfront.go is the pipeline reform's first drop package, D1
// (doc/PIPELINE-REFORM.md §5): the editor's command line cut, on the
// seed's text, to what the product accepts -- `+{command}` and nothing
// else, bare `+` meaning `$` -- in one cut, where thirteen phases cut it an
// option or two at a time.  What the options set is left to the phases
// that fold it, and what nothing calls any more to the sweep.

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
)

// argvScanBody is command_line_scan()'s body as the product has it, in the
// seed's spelling.
const argvScanBody = `{
    int argc = parmp->argc;
    char **argv = parmp->argv;
    int argv_idx;
    --argc;
    ++argv;
    argv_idx = 1;
    while (argc > 0)
    {
        if (argv[0][0] == '+')
        {
            if (parmp->n_commands >= MAX_ARG_CMDS)
            {
                mainerr(ME_EXTRA_CMD, nullptr);
            }
            argv_idx = -1;
            if (argv[0][1] == NUL)
            {
                parmp->commands[parmp->n_commands++] = (char_u *)"$";
            }
            else
            {
                parmp->commands[parmp->n_commands++] = (char_u *)&(argv[0][1]);
            }
        }
        else
        {
            mainerr(ME_UNKNOWN_OPTION, (char_u *)argv[0]);
        }
        if (argv_idx <= 0 || argv[0][argv_idx] == NUL)
        {
            --argc;
            ++argv;
            argv_idx = 1;
        }
    }
}`

// ArgvFront cuts the command line to `+{command}`: command_line_scan()'s
// body, the calls of parse_command_name() and early_arg_scan() (argv[0]'s
// mode and the options read before the rest), main()'s prescan for
// --clean, and the errors it can no longer give, mainerr_arg_missing()
// with them.  Each is asserted to be where the seed has it, once.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the body by FRAG (BodyC), the
// calls and the prescan cut by form, main_errors[]'s three rows deleted
// with the ME_* enumerators said as their index (INITROW), so that the
// three that indexed them go and ME_EXTRA_CMD is 1 where it was 4 -- what
// the text program spelled out as a replacement of both (history keeps
// it); B2b's TestRowsArgvFront first proved it.
func ArgvFront(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("argvfront", e, w)
	q := graph.NewVerbs("argvfront", e, io.Discard)
	if e.Defn("command_line_scan") == nil {
		v.Die("command_line_scan is not defined")
		return v.Done()
	}
	was := 0
	v.InFunction("command_line_scan", func(v *graph.Verbs) { was = bodyLines(v.Text()) })
	body := strings.TrimSuffix(strings.TrimPrefix(argvScanBody, "{\n"), "}")
	q.BodyC("command_line_scan", body, "command_line_scan's body")
	q.InFunction("main", func(q *graph.Verbs) {
		q.Cut("(call parse_command_name (addr params))", 1, "parse_command_name's call")
		q.Cut(`(for (= i 1) (< i argc) (pre++ i) (block (if (== (call strcasecmp _ (cast (ptr char) (paren "--clean"))) 0) _)))`,
			1, "main's --clean prescan")
	})
	q.Cut("(call early_arg_scan paramp)", 1, "early_arg_scan's call")
	// the errors it can still give: the enumerators are main_errors[]'s
	// indices, so the rows go with them said, to the two the parser names
	// and the row nothing names that the product keeps
	var ix graph.RowIndex
	for _, n := range []string{"ME_UNKNOWN_OPTION", "ME_TOO_MANY_ARGS", "ME_ARG_MISSING", "ME_GARBAGE", "ME_EXTRA_CMD"} {
		var en *graph.Node
		for _, d := range e.Decls(n) {
			if e.IsEnumerator(d) {
				en = d
			}
		}
		if en == nil {
			v.Die("%s is not an enumerator", n)
			return v.Done()
		}
		ix.Enumerators = append(ix.Enumerators, en)
	}
	q.InTable("main_errors", func(q *graph.Verbs) {
		done := q.DeleteRowsEach([]string{`"Too many edit arguments"`, `"Argument missing after"`, `"Garbage after option argument"`},
			ix, "main_errors[]")
		q.Expect(done == nil || len(done.GoneEnumerators) == 3 && len(done.Renumbered) == 1 && done.Renumbered[0].New == 1,
			"the ME_* enumerators did not become ME_UNKNOWN_OPTION 0 and ME_EXTRA_CMD 1")
	})
	// the one helper that named an error gone: nothing calls it now
	q.DeleteDefinition("mainerr_arg_missing", "mainerr_arg_missing")
	q.Cut("(def static mainerr_arg_missing _*)", 1, "mainerr_arg_missing's prototype")
	if err := q.Done(); err != nil {
		return err
	}
	now := 0
	v.InFunction("command_line_scan", func(v *graph.Verbs) { now = bodyLines(v.Text()) })
	v.Sayf("the command line is +{command} alone: command_line_scan %d lines -> %d, argv[0], the early scan and the --clean prescan gone",
		was, now)
	return v.Done()
}
