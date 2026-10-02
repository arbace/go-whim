package cut

// argvfront.go is the pipeline reform's first drop package, D1
// (doc/PIPELINE-REFORM.md §5): the editor's command line cut, on the
// seed's text, to what the product accepts -- `+{command}` and nothing
// else, bare `+` meaning `$` -- in one cut, where thirteen phases cut it an
// option or two at a time.  What the options set is left to the phases
// that fold it, and what nothing calls any more to the sweep.

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/edit"
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

const (
	argvEnumsOld = "enum { ME_UNKNOWN_OPTION = 0 };\n\nenum { ME_TOO_MANY_ARGS = 1 };\n\nenum { ME_ARG_MISSING = 2 };\n\nenum { ME_GARBAGE = 3 };\n\nenum { ME_EXTRA_CMD = 4 };\n"
	argvEnumsNew = "enum { ME_UNKNOWN_OPTION = 0 };\n\nenum { ME_EXTRA_CMD = 1 };\n"
	argvRowsOld  = "    \"Unknown option argument\",\n    \"Too many edit arguments\",\n    \"Argument missing after\",\n    \"Garbage after option argument\",\n"
	argvRowsNew  = "    \"Unknown option argument\",\n"
)

// ArgvFront cuts the command line to `+{command}`: command_line_scan()'s
// body, the calls of parse_command_name() and early_arg_scan() (argv[0]'s
// mode and the options read before the rest), main()'s prescan for
// --clean, and the errors it can no longer give, mainerr_arg_missing()
// with them.  Each is asserted to be
// where the seed has it, once.
func ArgvFront(text []byte, w io.Writer) ([]byte, error) {
	// command_line_scan's body, found by brace matching from its head
	head := regexp.MustCompile(`(?m)^command_line_scan\(mparm_T \*parmp\)\n`)
	m := head.FindAllIndex(text, -1)
	if len(m) != 1 {
		return nil, fmt.Errorf("argvfront: command_line_scan is defined %d times, not once", len(m))
	}
	blanked := edit.Blank(text)
	open := m[0][1] + bytes.IndexByte(blanked[m[0][1]:], '{')
	close := edit.Match(blanked, open)
	if close < 0 {
		return nil, fmt.Errorf("argvfront: command_line_scan is unbalanced")
	}
	was := bytes.Count(text[open:close+1], []byte("\n"))
	text = append(append(append([]byte{}, text[:open]...), argvScanBody...), text[close+1:]...)
	// the calls, and main()'s prescan
	for _, c := range []struct{ pat, what string }{
		{edit.Line("parse_command_name(&params);"), "parse_command_name's call"},
		{edit.Line("early_arg_scan(paramp);"), "early_arg_scan's call"},
		{`(?m)^    for \(i = 1; i < argc; \+\+i\)\n    \{\n        if \(strcasecmp\(\(char \*\)\(argv\[i\]\), \(char \*\)\("--clean"\)\) == 0\)\n        \{\n            params\.clean = TRUE;\n            break;\n        \}\n    \}\n`, "main's --clean prescan"},
	} {
		re := regexp.MustCompile(c.pat)
		out, n := edit.ReplaceAllCounted(re, text, nil)
		if n != 1 {
			return nil, fmt.Errorf("argvfront: %s is there %d times, not once", c.what, n)
		}
		text = out
	}
	// the errors it can still give: the enumerators are main_errors[]'s
	// indices, so the two are rewritten together, to the two the parser
	// names and the row nothing names that the product keeps
	for _, c := range []struct{ old, new, what string }{
		{argvEnumsOld, argvEnumsNew, "the ME_* enumerators"},
		{argvRowsOld, argvRowsNew, "main_errors[]"},
	} {
		if n := bytes.Count(text, []byte(c.old)); n != 1 {
			return nil, fmt.Errorf("argvfront: %s are there %d times, not once", c.what, n)
		}
		text = bytes.Replace(text, []byte(c.old), []byte(c.new), 1)
	}
	// the one helper that named an error gone: nothing calls it now
	t2, ok := edit.DeleteDefinition(text, "mainerr_arg_missing")
	if !ok {
		return nil, fmt.Errorf("argvfront: mainerr_arg_missing is not defined")
	}
	proto := []byte("static void mainerr_arg_missing(char_u *str);\n")
	if bytes.Count(t2, proto) != 1 {
		return nil, fmt.Errorf("argvfront: mainerr_arg_missing's prototype is there %d times, not once", bytes.Count(t2, proto))
	}
	text = bytes.Replace(t2, proto, nil, 1)
	fmt.Fprintf(w, "  argvfront    the command line is +{command} alone: command_line_scan %d lines -> %d, argv[0], the early scan and the --clean prescan gone\n",
		was, bytes.Count([]byte(argvScanBody), []byte("\n")))
	return text, nil
}
