package p152

// Whim phase 152 -- the option variables are typed.  See GOAL.md.
//
// vimoption_T.var, optset_T.os_varp, get_varp() and every varp held the address
// of an int, a long or a char_u * as a char_u * (tx/FINDINGS.md, 2).  They are
// an optvar_T, a pointer of each kind and a window-local flag, and a
// window-local option's global value is get_varp_allbuf(), not a byte offset.
//
// THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
// makefile flags, as $state/old beside $state/old.c, for the check.
// An edit that starts a background job waits for it before it exits (tools/phaserun.sh).

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim152", Edit) }

// W152Type is optvar_T and its helpers, exported so the check requires the
// identical text.
const W152Type = `typedef struct
{
    int         *ov_int;
    long        *ov_long;
    char_u      **ov_str;
    int         ov_win;
} optvar_T;

    static optvar_T
optvar_int(int *p)
{
    optvar_T    v = {p, nullptr, nullptr, 0};

    return v;
}

    static optvar_T
optvar_long(long *p)
{
    optvar_T    v = {nullptr, p, nullptr, 0};

    return v;
}

    static optvar_T
optvar_str(char_u **p)
{
    optvar_T    v = {nullptr, nullptr, p, 0};

    return v;
}

    static optvar_T
optvar_none(void)
{
    optvar_T    v = {nullptr, nullptr, nullptr, 0};

    return v;
}

    static int
optvar_is_null(optvar_T v)
{
    return v.ov_int == nullptr && v.ov_long == nullptr && v.ov_str == nullptr && !v.ov_win;
}

`

// W152Allbuf is get_varp_allbuf(): the global value of a window-local option
// with no global variable, the field of w_allbuf_opt that phase 151's
// `(char *)get_varp(p) + sizeof(winopt_T)` reached.
const W152Allbuf = `    static optvar_T
get_varp_allbuf(struct vimoption *p)
{
    switch ((int)p->indir)
    {
%s    }
    return optvar_none();
}

`

var (
	w152Decl  = regexp.MustCompile(`(?m)^\s+(long|int|char_u)\s+(\*?)(wo_\w+|b_p_\w+|b_changed);`)
	w152Glob  = regexp.MustCompile(`(?m)^static (long|int|char_u)\s+(\*?)(\w+)\s*(?:=[^;]*)?;`)
	w152Addr  = regexp.MustCompile(`\(char_u \*\)&\((\s*(?:curwin->\s*w_onebuf_opt\.|curbuf->)(\w+)\s*)\)`)
	w152Deref = regexp.MustCompile(`\*\((int \*|long \*|char_u \*\*|char \*\*)\)\s*(?:\(\s*((?:\bvarp|p->var|opp->var|options\[[^\]]+\]\.var|get_varp(?:_scope)?\(&\(?options\[[^\]]+\]\)?(?:, [\w |?:]+)?\)))\s*\)|((?:\bvarp|p->var|opp->var|options\[[^\]]+\]\.var|get_varp(?:_scope)?\(&\(?options\[[^\]]+\]\)?(?:, [\w |?:]+)?\))))`)
	w152Value = regexp.MustCompile(`\((long \*|char_u \*\*|int \*)\)\s*((?:\bvarp|opp->var|args->os_varp|get_varp(?:_scope)?\(&\(?options\[[^\]]+\]\)?(?:, [\w |?:]+)?\)))`)
	w152Win   = regexp.MustCompile(`(options\[[^\]]+\]\.var|p->var)\s*(==|!=)\s*\(\(char_u \*\)-1\)`)
	w152Cmp   = regexp.MustCompile(`\bvarp == \(char_u \*\)&(\w+)`)
	w152Null  = regexp.MustCompile(`((?:\bvarp|p->var|opp->var|options\[[^\]]+\]\.var))\s*(==|!=)\s*nullptr`)
)

func w152Kind(t, star string) string {
	switch {
	case t == "int" && star == "":
		return "optvar_int"
	case t == "long" && star == "":
		return "optvar_long"
	case t == "char_u" && star == "*":
		return "optvar_str"
	}
	return ""
}

// Whim152 types the option variables.
//
// vimoption_T.var, optset_T.os_varp, get_varp() and every `varp` held the
// address of an option's variable -- an int, a long or a char_u * -- as a
// char_u *, cast back at every read: `*(int *)varp`.  A window-local option
// with no global variable held (char_u *)-1, and its global value was reached
// as `(char *)get_varp(p) + sizeof(winopt_T)`, the same field one winopt_T
// further on.  The Go transpilation held them as `any` (tx/FINDINGS.md, 2).
// Now they are an optvar_T: a pointer of each kind, one set, and a flag for
// the window-local sentinel.  Each read names its kind (`*varp.ov_int`), the
// table's rows are typed by the row's P_BOOL, P_NUM or P_STRING, get_varp()'s
// returns by the declared type of the field they name, and the window-local
// global value is get_varp_allbuf(), the w_allbuf_opt field -- in
// get_varp_scope() and in set_string_option_global(), whose two callers hand
// it curwin's w_onebuf_opt field, the one the byte offset stepped from.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "optvar", W: w}
	s := string(text)
	var err error
	lit := func(old, new, what string, n int) {
		if err != nil {
			return
		}
		if k := strings.Count(s, old); k != n {
			err = p.Die("%s (%.60q) -- occurs %d times, expected %d", what, old, k, n)
			return
		}
		s = strings.ReplaceAll(s, old, new)
		if what != "" {
			p.Say(what)
		}
	}
	// the types of the fields and globals an option can name
	kind := map[string]string{}
	for _, m := range w152Decl.FindAllStringSubmatch(s, -1) {
		kind[m[3]] = w152Kind(m[1], m[2])
	}
	for _, m := range w152Glob.FindAllStringSubmatch(s, -1) {
		if _, ok := kind[m[3]]; !ok {
			kind[m[3]] = w152Kind(m[1], m[2])
		}
	}

	// 1. the table's rows
	head := "static struct vimoption options[] = {\n"
	ti := strings.Index(s, head)
	if ti < 0 {
		return nil, p.Die("options[] is not where this phase expects it")
	}
	b := cutil.Blank([]byte(s))
	tend := cutil.Match(b, ti+len(head)-2)
	type cut struct {
		a, z int
		t    string
	}
	var cuts []cut
	rows := 0
	for k := ti + len(head) - 1; k < tend; k++ {
		if b[k] != '{' {
			continue
		}
		re := cutil.Match(b, k)
		// the fourth top-level field of the row
		depth, field, start := 0, 0, k+1
		var fs [][2]int
		for j := k + 1; j < re; j++ {
			switch b[j] {
			case '(', '{':
				depth++
			case ')', '}':
				depth--
			case ',':
				if depth == 0 {
					fs = append(fs, [2]int{start, j})
					start = j + 1
					field++
				}
			}
		}
		if len(fs) < 4 {
			return nil, p.Die("a row of options[] has %d fields", len(fs))
		}
		flags := s[fs[2][0]:fs[2][1]]
		v := strings.TrimSpace(s[fs[3][0]:fs[3][1]])
		var nv string
		vn := strings.Join(strings.Fields(v), "")
		switch {
		case vn == "nullptr" || vn == "(char_u*)nullptr":
			nv = "{nullptr, nullptr, nullptr, 0}"
		case vn == "((char_u*)-1)" || vn == "(char_u*)((char_u*)-1)":
			nv = "{nullptr, nullptr, nullptr, 1}"
		case strings.HasPrefix(v, "(char_u *)&"):
			addr := "&" + strings.TrimSpace(strings.TrimPrefix(v, "(char_u *)&"))
			switch {
			case strings.Contains(flags, "P_BOOL"):
				nv = "{" + addr + ", nullptr, nullptr, 0}"
			case strings.Contains(flags, "P_NUM"):
				nv = "{nullptr, " + addr + ", nullptr, 0}"
			case strings.Contains(flags, "P_STRING"):
				nv = "{nullptr, nullptr, " + addr + ", 0}"
			}
		}
		if nv == "" {
			return nil, p.Die("a row's variable %q with flags %q has no kind", v, strings.TrimSpace(flags))
		}
		lead := s[fs[3][0] : fs[3][0]+len(s[fs[3][0]:fs[3][1]])-len(strings.TrimLeft(s[fs[3][0]:fs[3][1]], " \n"))]
		cuts = append(cuts, cut{fs[3][0], fs[3][1], lead + nv})
		rows++
		k = re
	}
	for k := len(cuts) - 1; k >= 0; k-- {
		s = s[:cuts[k].a] + cuts[k].t + s[cuts[k].z:]
	}
	p.Say(fmt.Sprintf("the %d rows of options[] name their variable by kind", rows))

	// 2. the type, the fields, the prototypes
	lit("    char_u *var;\n    idopt_T indir;\n", "    optvar_T    var;\n    idopt_T     indir;\n", "vimoption_T.var is an optvar_T", 1)
	lit("typedef struct\n{\n    char_u *os_varp;\n", W152Type+"typedef struct\n{\n    optvar_T    os_varp;\n", "and so is optset_T.os_varp, after the type and its helpers", 1)
	lit("static char_u *get_varp_scope(struct vimoption *p, int scope);\n\nstatic char_u *get_varp(struct vimoption *);\n",
		"static optvar_T get_varp_scope(struct vimoption *p, int scope);\nstatic optvar_T get_varp(struct vimoption *);\n", "get_varp() and get_varp_scope() return one", 1)
	for _, f := range []string{"get_varp_scope", "get_option_varp_scope", "get_varp"} {
		lit("    static char_u *\n"+f+"(", "    static optvar_T\n"+f+"(", "", 1)
	}
	lit("    static char_u *\nget_option_var(int opt_idx)\n{\n    return options[opt_idx].var;\n}", "    static char_u **\nget_option_var(int opt_idx)\n{\n    return options[opt_idx].var.ov_str;\n}", "get_option_var() returns the string variable its one caller wants", 1)
	lit("(char_u **)get_option_var(opt_idx)", "get_option_var(opt_idx)", "", 1)
	if err != nil {
		return nil, err
	}

	// 3. get_varp()'s and get_varp_scope()'s returns, typed by the field
	n := 0
	var bad []string
	s = w152Addr.ReplaceAllStringFunc(s, func(m string) string {
		sm := w152Addr.FindStringSubmatch(m)
		k := kind[sm[2]]
		if k == "" {
			bad = append(bad, sm[2])
			return m
		}
		n++
		return k + "(&(" + sm[1] + "))"
	})
	if len(bad) > 0 {
		return nil, p.Die("fields of no known type: %v", bad)
	}
	p.Say(fmt.Sprintf("%d addresses of an option's field are typed by the field", n))

	// 4. the window-local global value
	var cases strings.Builder
	for _, m := range regexp.MustCompile(`(?m)^        case   \(idopt_T\)\(PV_WIN \+ \(int\)\((WV_\w+)\)\)  :\n            return (optvar_\w+)\(&\(curwin-> w_onebuf_opt\.(\w+) \)\);\n`).FindAllStringSubmatch(s, -1) {
		fmt.Fprintf(&cases, "        case   (idopt_T)(PV_WIN + (int)(%s))  :\n            return %s(&(curwin-> w_allbuf_opt.%s ));\n", m[1], m[2], m[3])
	}
	lit("        if (p->var == ((char_u *)-1))\n        {\n            return (char_u *)((char *)(get_varp(p)) + sizeof(winopt_T));\n        }\n",
		"        if (p->var.ov_win)\n        {\n            return get_varp_allbuf(p);\n        }\n", "a window-local option's global value is get_varp_allbuf()", 1)
	lit("    static optvar_T\nget_varp_scope(", fmt.Sprintf(W152Allbuf, cases.String())+"    static optvar_T\nget_varp_scope(", "", 1)
	lit("    return options[opt_idx].var == ((char_u *)-1);", "    return options[opt_idx].var.ov_win;", "and the sentinel is the flag", 1)

	// 5. the two functions that reused varp for the string it held
	lit("        varp = get_varp(&(options[opt_idx]));\n        if (varp != nullptr)\n        {\n            varp = *(char_u **)(varp);\n        }\n        return varp;\n",
		"        varp = get_varp(&(options[opt_idx]));\n        if (!optvar_is_null(varp))\n        {\n            return *varp.ov_str;\n        }\n        return nullptr;\n", "get_term_code() returns the string without reusing varp for it", 1)
	lit("        varp = *(char_u **)(varp);\n        if (varp == nullptr)\n        {\n            NameBuff[0] = NUL;\n        }\n        else if (opp->flags & P_EXPAND)\n        {\n            home_replace(nullptr, varp, NameBuff,  PATH_MAX , FALSE);\n        }\n        else if ((char_u **)opp->var == &p_pt)\n        {\n            str2specialbuf(p_pt, NameBuff,  PATH_MAX );\n        }\n        else\n        {\n            vim_strncpy(NameBuff, varp,  PATH_MAX  - 1);\n        }\n",
		"        char_u      *s = *varp.ov_str;\n\n        if (s == nullptr)\n        {\n            NameBuff[0] = NUL;\n        }\n        else if (opp->flags & P_EXPAND)\n        {\n            home_replace(nullptr, s, NameBuff,  PATH_MAX , FALSE);\n        }\n        else if (opp->var.ov_str == &p_pt)\n        {\n            str2specialbuf(p_pt, NameBuff,  PATH_MAX );\n        }\n        else\n        {\n            vim_strncpy(NameBuff, s,  PATH_MAX  - 1);\n        }\n",
		"nor does option_value2string()", 1)

	// 6. the rest, by rule
	n = len(w152Deref.FindAllString(s, -1))
	s = w152Deref.ReplaceAllStringFunc(s, func(m string) string {
		sm := w152Deref.FindStringSubmatch(m)
		x := sm[2] + sm[3]
		switch sm[1] {
		case "int *":
			return "*" + x + ".ov_int"
		case "long *":
			return "*" + x + ".ov_long"
		case "char_u **":
			return "*" + x + ".ov_str"
		}
		return "(char *)*" + x + ".ov_str"
	})
	p.Say(fmt.Sprintf("%d reads of an option's variable name its kind", n))
	n = len(w152Value.FindAllString(s, -1))
	s = w152Value.ReplaceAllStringFunc(s, func(m string) string {
		sm := w152Value.FindStringSubmatch(m)
		switch sm[1] {
		case "long *":
			return sm[2] + ".ov_long"
		case "int *":
			return sm[2] + ".ov_int"
		}
		return sm[2] + ".ov_str"
	})
	p.Say(fmt.Sprintf("%d casts of it to a typed pointer name the pointer", n))
	n = len(w152Null.FindAllString(s, -1))
	s = w152Null.ReplaceAllStringFunc(s, func(m string) string {
		sm := w152Null.FindStringSubmatch(m)
		if sm[2] == "==" {
			return "optvar_is_null(" + sm[1] + ")"
		}
		return "!optvar_is_null(" + sm[1] + ")"
	})
	p.Say(fmt.Sprintf("%d tests for no variable ask optvar_is_null()", n))
	gs := regexp.MustCompile(`\(char_u \*\*\)get_option_varp_scope\(([^;]*)\);`)
	n = len(gs.FindAllString(s, -1))
	if n != 2 {
		return nil, p.Die("%d string reads through get_option_varp_scope(), and this phase was written against 2", n)
	}
	s = gs.ReplaceAllString(s, "get_option_varp_scope($1).ov_str;")
	lit("            char_u *p = get_option_varp_scope(opt_idx, OPT_LOCAL);\n            free_string_option(*(char_u **)p);\n            *(char_u **)p = empty_option;\n",
		"            char_u **p = get_option_varp_scope(opt_idx, OPT_LOCAL).ov_str;\n            free_string_option(*p);\n            *p = empty_option;\n", "the callers of get_option_varp_scope() take the string variable", 1)
	lit("        p = (char_u **)((char *)(varp) + sizeof(winopt_T));\n", "        p = get_varp_allbuf(&(options[opt_idx])).ov_str;\n",
		"set_string_option_global() finds a window-local option's global value by name, not by byte offset", 1)
	n = len(w152Win.FindAllString(s, -1))
	s = w152Win.ReplaceAllStringFunc(s, func(m string) string {
		sm := w152Win.FindStringSubmatch(m)
		if sm[2] == "==" {
			return sm[1] + ".ov_win"
		}
		return "!" + sm[1] + ".ov_win"
	})
	p.Say(fmt.Sprintf("%d tests for the window-local sentinel read the flag", n))
	s = w152Cmp.ReplaceAllStringFunc(s, func(m string) string {
		sm := w152Cmp.FindStringSubmatch(m)
		f := map[string]string{"optvar_int": "ov_int", "optvar_long": "ov_long", "optvar_str": "ov_str"}[kind[sm[1]]]
		return "varp." + f + " == &" + sm[1]
	})
	lit("        return nullptr;\n    }\n    return get_varp(p);\n}", "        return optvar_none();\n    }\n    return get_varp(p);\n}", "", 1)
	lit("get_varp(struct vimoption *p)\n{\n    if (optvar_is_null(p->var))\n    {\n        return nullptr;\n    }\n", "get_varp(struct vimoption *p)\n{\n    if (optvar_is_null(p->var))\n    {\n        return optvar_none();\n    }\n", "", 1)
	lit("        varp = nullptr;\n", "        varp = optvar_none();\n", "", 2)
	lit("args.os_varp = (char_u *)varp;", "args.os_varp = optvar_str(varp);", "", 1)
	lit("options[opt_idx].var == (char_u *)p)", "options[opt_idx].var.ov_str == p)", "", 1)
	lit("        if (p->var == var)\n", "        if ((char_u *)p->var.ov_str == var)\n", "free_one_termoption() compares as it did", 1)
	if err != nil {
		return nil, err
	}
	// 7. every generic varp declaration
	decl := regexp.MustCompile(`char_u(\s+)\*(varp|varp_arg)\b`)
	n = len(decl.FindAllString(s, -1))
	s = decl.ReplaceAllString(s, "optvar_T$1$2")
	p.Say(fmt.Sprintf("%d varp declarations are optvar_T", n))
	return []byte(s), nil
}
