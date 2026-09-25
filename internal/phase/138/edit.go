package p138

// Whim phase 138 -- no parameter carries an eval value.  See GOAL.md.
//
// vim_regsub_both's expr, match_add's pos_list, cursor_pos_info's dict, the
// formatter's tvs and find_ex_command's Vim9 lookup and context are passed
// nullptr by every call.  They go, each test of them folds, and the sweep takes
// typval_T, lists, dicts, type_T, class_T and the rest of the eval values.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/edit"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.Register("whim138", Edit) }

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
// and with them the builtin function types cfunc_T and cfunc_free_T, which
// the sweep takes.  They are the last
// things naming typval_T, list_T and dict_T outside their own definitions, so
// the sweep takes the eval layer's value types with them.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	e := edit.New("evalparm", text, w)
	// vim_regsub_both
	e.Literal("static int vim_regsub_both(char_u *source, typval_T *expr, char_u *dest,", "static int vim_regsub_both(char_u *source, char_u *dest,", 1,
		"vim_regsub_both() takes no expression: its prototype")
	e.Literal("vim_regsub_both(char_u *source, typval_T *expr, char_u *dest,", "vim_regsub_both(char_u *source, char_u *dest,", 1,
		"its definition")
	e.Literal("vim_regsub_both(source, nullptr, dest, destlen, flags)", "vim_regsub_both(source, dest, destlen, flags)", 1,
		"its one call")
	e.Literal("if ((source == nullptr && expr == nullptr) || dest == nullptr)", "if (source == nullptr || dest == nullptr)", 1,
		"a NULL source is refused whatever the expression was")
	e.Literal("if (expr != nullptr || (source[0] == '\\\\' && source[1] == '='))", "if (source[0] == '\\\\' && source[1] == '=')", 1,
		"and only a \\= source is an expression")
	// match_add
	e.Literal("int id, list_T *pos_list, char_u *conceal_char)", "int id, char_u *conceal_char)", 1,
		"match_add() takes no list of positions, which it never read")
	e.Literal("match_add(curwin, g, p + 1, 10, id, nullptr, nullptr);", "match_add(curwin, g, p + 1, 10, id, nullptr);", 1,
		"its one call")
	// cursor_pos_info
	e.Literal("static void cursor_pos_info(dict_T *dict);", "static void cursor_pos_info(void);", 1,
		"cursor_pos_info() fills no dictionary: its prototype")
	e.Literal("\ncursor_pos_info(dict_T *dict)\n", "\ncursor_pos_info(void)\n", 1,
		"its definition")
	e.Literal("cursor_pos_info(nullptr);", "cursor_pos_info();", 1,
		"its one call")
	// the host's formatter
	e.Literal("static int vim_vsnprintf_typval(char *str, usize str_m, const char *fmt, va_list ap, typval_T *tvs)", "static int vim_vsnprintf_typval(char *str, usize str_m, const char *fmt, va_list ap)", 1,
		"the formatter takes no argument list: its prototype")
	e.Literal("va_list ap_start, typval_T *tvs)", "va_list ap_start)", 1,
		"its definition")
	e.Literal("vim_vsnprintf_typval(str, str_m, fmt, ap, nullptr)", "vim_vsnprintf_typval(str, str_m, fmt, ap)", 1,
		"its one call")
	e.Literal("const char *fmt, typval_T *tvs)", "const char *fmt)", 1,
		"parse_fmt_types() takes none either")
	e.Literal("parse_fmt_types(&ap_types, &num_posarg, fmt, tvs)", "parse_fmt_types(&ap_types, &num_posarg, fmt)", 1,
		"its one call")
	e.Literal(", tvs != nullptr) == FAIL)", ", FALSE) == FAIL)", 10,
		"and no number in a format is read from a list")
	// find_ex_command, whose lookup and compile context were Vim9 script's
	e.Literal("static char_u *find_ex_command(exarg_T *eap, int *full, int (*lookup)(char_u *, usize, int cmd, cctx_T *), cctx_T *cctx);", "static char_u *find_ex_command(exarg_T *eap, int *full);", 1,
		"find_ex_command() takes no Vim9 lookup or context, which it never read: its prototype")
	e.Literal("find_ex_command(exarg_T *eap, int *full, int (*lookup)(char_u *, usize, int cmd, cctx_T *), cctx_T *cctx)", "find_ex_command(exarg_T *eap, int *full)", 1,
		"its definition")
	e.Literal("find_ex_command(&ea, nullptr, nullptr, nullptr)", "find_ex_command(&ea, nullptr)", 1,
		"its one call")
	e.InFunction("cursor_pos_info", func(e *edit.E) {
		e.FoldAlways(`if \(dict == nullptr\)`, 3, "cursor_pos_info() always gives its message")
	})
	e.InFunction("vim_vsnprintf_typval", func(e *edit.E) {
		e.FoldNever(`if \(tvs != nullptr\)`, 2, "and the formatter always clamps an overlong width or precision")
		e.FoldNever(`if \(tvs != nullptr && tvs\[num_posarg != 0 \? num_posarg : arg_idx - 1\]\.v_type != VAR_UNKNOWN\)`, 1, "and never counts arguments left over in a list")
	})
	n := e.Mentions("tvs")
	e.Expect(n == 0, "tvs has %d mentions left", n)
	return e.Done()
}
