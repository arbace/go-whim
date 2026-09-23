package p138

// Whim phase 138 -- no parameter carries an eval value.  See GOAL.md.
//
// vim_regsub_both's expr, match_add's pos_list, cursor_pos_info's dict, the
// formatter's tvs and find_ex_command's Vim9 lookup and context are passed
// nullptr by every call.  They go, each test of them folds, and the sweep takes
// typval_T, lists, dicts, type_T, class_T and the rest of the eval values.
//
// THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
// makefile flags, as $state/old beside $state/old.c, for the check.
// An edit that starts a background job waits for it before it exits (tools/phaserun.sh).

import (
	"io"

	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim138", Edit) }

// Whim138 takes Out the parameters that carry an eval value.
//
// Four functions still take one, and every call passes nullptr: the
// substitute string's expression (vim_regsub_both's typval_T *expr, for
// substitute() with a funcref), matchaddpos()'s list of positions
// (match_add's list_T *pos_list), wordcount()'s dictionary
// (cursor_pos_info's dict_T *dict), and printf()'s argument list (the
// formatter's typval_T *tvs, in the host).  Each parameter goes with its
// nullptr, and each test of it becomes what it always was.  So do
// find_ex_command()'s Vim9 lookup and compile context, which it never read,
// and the builtin function types cfunc_T and cfunc_free_T, which nothing names
// and the sweep does not take -- a function-pointer typedef is not a type it
// follows.  They are the last
// things naming typval_T, list_T and dict_T outside their own definitions, so
// the sweep takes the eval layer's value types with them.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "evalparm", W: w}
	var err error
	steps := []struct {
		Old, New, What string
		n              int
	}{
		// vim_regsub_both
		{"static int vim_regsub_both(char_u *source, typval_T *expr, char_u *dest,", "static int vim_regsub_both(char_u *source, char_u *dest,", "vim_regsub_both() takes no expression: its prototype", 1},
		{"vim_regsub_both(char_u      *source, typval_T    *expr, char_u      *dest,", "vim_regsub_both(char_u      *source, char_u      *dest,", "its definition", 1},
		{"vim_regsub_both(source, nullptr, dest, destlen, flags)", "vim_regsub_both(source, dest, destlen, flags)", "its one call", 1},
		{"if ((source == nullptr && expr == nullptr) || dest == nullptr)", "if (source == nullptr || dest == nullptr)", "a NULL source is refused whatever the expression was", 1},
		{"if (expr != nullptr || (source[0] == '\\\\' && source[1] == '='))", "if (source[0] == '\\\\' && source[1] == '=')", "and only a \\= source is an expression", 1},
		// match_add
		{"int         id, list_T      *pos_list, char_u      *conceal_char)", "int         id, char_u      *conceal_char)", "match_add() takes no list of positions, which it never read", 1},
		{"match_add(curwin, g, p + 1, 10, id, nullptr, nullptr);", "match_add(curwin, g, p + 1, 10, id, nullptr);", "its one call", 1},
		// cursor_pos_info
		{"static void cursor_pos_info(dict_T *dict);", "static void cursor_pos_info(void);", "cursor_pos_info() fills no dictionary: its prototype", 1},
		{"\ncursor_pos_info(dict_T *dict)\n", "\ncursor_pos_info(void)\n", "its definition", 1},
		{"cursor_pos_info(nullptr);", "cursor_pos_info();", "its one call", 1},
		// the host's formatter
		{"static int vim_vsnprintf_typval(char *str, usize str_m, const char *fmt, va_list ap, typval_T *tvs)", "static int vim_vsnprintf_typval(char *str, usize str_m, const char *fmt, va_list ap)", "the formatter takes no argument list: its prototype", 1},
		{"va_list ap_start, typval_T *tvs)", "va_list     ap_start)", "its definition", 1},
		{"vim_vsnprintf_typval(str, str_m, fmt, ap, nullptr)", "vim_vsnprintf_typval(str, str_m, fmt, ap)", "its one call", 1},
		{"const char  *fmt, typval_T    *tvs)", "const char *fmt)", "parse_fmt_types() takes none either", 1},
		{"parse_fmt_types(&ap_types, &num_posarg, fmt, tvs)", "parse_fmt_types(&ap_types, &num_posarg, fmt)", "its one call", 1},
		{", tvs != nullptr) == FAIL)", ", FALSE) == FAIL)", "and no number in a format is read from a list", 10},
		// find_ex_command, whose lookup and compile context were Vim9 script's
		{"static char_u *find_ex_command(exarg_T *eap, int *full, int (*lookup)(char_u *, usize, int cmd, cctx_T *), cctx_T *cctx);", "static char_u *find_ex_command(exarg_T *eap, int *full);", "find_ex_command() takes no Vim9 lookup or context, which it never read: its prototype", 1},
		{"find_ex_command(exarg_T *eap, int     *full, int     (*lookup)(char_u *, usize, int cmd, cctx_T *), cctx_T  *cctx)", "find_ex_command(exarg_T *eap, int     *full)", "its definition", 1},
		{"find_ex_command(&ea, nullptr, nullptr, nullptr)", "find_ex_command(&ea, nullptr)", "its one call", 1},
		// the builtin-function types, which name typval_T and nothing names
		{"typedef int (*cfunc_T)(int argcount, typval_T *argvars, typval_T *rettv, void *state);\n", "", "the builtin function type goes", 1},
		{"typedef void (*cfunc_free_T)(void *state);\n", "", "and its state's destructor", 1},
	}
	for _, s := range steps {
		if text, err = p.Literal(text, s.Old, s.New, s.What, s.n); err != nil {
			return nil, err
		}
	}
	if text, err = p.FoldAlways(text, "cursor_pos_info", `if \(dict == nullptr\)`, "cursor_pos_info() always gives its message", 3); err != nil {
		return nil, err
	}
	if text, err = p.FoldNever(text, "vim_vsnprintf_typval", `if \(tvs != nullptr\)`, "and the formatter always clamps an overlong width or precision", 2); err != nil {
		return nil, err
	}
	if text, err = p.FoldNever(text, "vim_vsnprintf_typval", `if \(tvs != nullptr && tvs\[num_posarg != 0 \? num_posarg : arg_idx - 1\]\.v_type != VAR_UNKNOWN\)`, "and never counts arguments left over in a list", 1); err != nil {
		return nil, err
	}
	if n := p.Mentions(text, "tvs"); n != 0 {
		return nil, p.Die("tvs has %d mentions left", n)
	}
	return text, nil
}
