package cut

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/crefactor/graph"
)

// identBody is nv_ident rewritten to the half that is search.  Generated from
// the Python module's own constant by importing it, because it is a non-raw
// triple-quoted string full of backslashes.  It is in UTF-8's spelling, with
// no has_mbyte, since utf8only runs before it at phase 2 (the reform's D7).
const identBody = "    char_u      *ptr = nullptr;\n" +
	"    char_u      *buf;\n" +
	"    usize bufsize;\n" +
	"    usize      buflen;\n" +
	"    char_u      *p;\n" +
	"    int         n = 0;\n" +
	"    int cmdchar;\n" +
	"    int g_cmd;\n" +
	"    char_u *aux_ptr;\n" +
	"\n" +
	"    if (cap->cmdchar == 'g')\n" +
	"    {\n" +
	"        cmdchar = cap->nchar;\n" +
	"        g_cmd = TRUE;\n" +
	"    }\n" +
	"    else\n" +
	"    {\n" +
	"        cmdchar = cap->cmdchar;\n" +
	"        g_cmd = FALSE;\n" +
	"    }\n" +
	"\n" +
	"    if (cmdchar == POUND)\n" +
	"    {\n" +
	"        cmdchar = '#';\n" +
	"    }\n" +
	"\n" +
	"    if (ptr == nullptr && (n = find_ident_under_cursor(&ptr, (cmdchar == '*' || cmdchar == '#') ? FIND_IDENT | FIND_STRING : FIND_IDENT)) == 0)\n" +
	"    {\n" +
	"        clearop(cap->oap);\n" +
	"        return;\n" +
	"    }\n" +
	"\n" +
	"    bufsize = (usize)(n * 2 + 30);\n" +
	"    buf = alloc(bufsize);\n" +
	"    if (buf == nullptr)\n" +
	"    {\n" +
	"        return;\n" +
	"    }\n" +
	"    buf[0] = NUL;\n" +
	"    buflen = 0;\n" +
	"\n" +
	"    setpcmark();\n" +
	"        curwin->w_cursor.col = (colnr_T)(ptr - ml_get_curline());\n" +
	"\n" +
	"    if (!g_cmd && vim_iswordp(ptr))\n" +
	"    {\n" +
	"            strcpy((char *)(buf), (char *)(\"\\\\<\"));\n" +
	"        buflen =  (sizeof(\"\\\\<\" \"\") - 1) ;\n" +
	"    }\n" +
	"    no_smartcase = TRUE;\n" +
	"\n" +
	"    if (cmdchar == '*')\n" +
	"    {\n" +
	"        aux_ptr = (char_u *)(magic_isset() ? \"/.*~[^$\\\\\" : \"/^$\\\\\");\n" +
	"    }\n" +
	"    else\n" +
	"    {\n" +
	"        aux_ptr = (char_u *)(magic_isset() ? \"/?.*~[^$\\\\\" : \"/?^$\\\\\");\n" +
	"    }\n" +
	"\n" +
	"    p = buf + buflen;\n" +
	"    while (n-- > 0)\n" +
	"    {\n" +
	"        if (vim_strchr(aux_ptr, *ptr) != nullptr)\n" +
	"        {\n" +
	"            *p++ = '\\\\';\n" +
	"        }\n" +
	"\n" +
	"        int i;\n" +
	"        int len = utfc_ptr2len(ptr) - 1;\n" +
	"\n" +
	"        for (i = 0; i < len && n >= 1; ++i, --n)\n" +
	"        {\n" +
	"            *p++ = *ptr++;\n" +
	"        }\n" +
	"        *p++ = *ptr++;\n" +
	"    }\n" +
	"    *p = NUL;\n" +
	"    buflen = p - buf;\n" +
	"\n" +
	"    if (!g_cmd && (vim_iswordp(mb_prevptr(ml_get_curline(), ptr))))\n" +
	"    {\n" +
	"            strcpy((char *)(buf + buflen), (char *)(\"\\\\>\"));\n" +
	"        buflen +=  (sizeof(\"\\\\>\" \"\") - 1) ;\n" +
	"    }\n" +
	"\n" +
	"    init_history();\n" +
	"    add_to_history(HIST_SEARCH, buf, buflen, TRUE, NUL);\n" +
	"\n" +
	"    (void)normal_search(cap, cmdchar == '*' ? '/' : '?', buf, buflen, 0, nullptr);\n" +
	"\n" +
	"    vim_free(buf);"

var identLeft = regexp.MustCompile(`\b(?:nv_K_getcmd|do_nv_ident|g_tag_at_cursor)\b`)

// NoIdent leaves `*` and `#` and takes K, CTRL-], g] and the two CTRL-W forms.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): nv_ident's body by FRAG
// (BodyC), the two nv_cmds rows pointed at nv_error (RewriteAt: a row keeps
// its place), g]'s labels by DropCase (history keeps the text version).
func NoIdent(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("noident", e, w)
	if e.Defn("nv_ident") == nil {
		return fmt.Errorf("noident: nv_ident is not defined at file scope")
	}
	was := 0
	v.InFunction("nv_ident", func(v *graph.Verbs) {
		fn := v.Text()
		if !bytes.Contains(fn, []byte("nv_K_getcmd")) {
			v.Die("nv_ident does not look like the one this expects")
		}
		was = bodyLines(fn)
	})
	q := graph.NewVerbs("noident", e, io.Discard)
	q.BodyC("nv_ident", identBody, "nv_ident")
	if err := v.Done(); err != nil {
		return err
	}
	if err := q.Done(); err != nil {
		return err
	}
	v.Sayf("nv_ident was %d lines and is now the search half", was)

	// A ROW IS NEVER DELETED FROM nv_cmds[], IT IS POINTED AT nv_error.
	// nv_cmd_idx[] is a static const array of INDICES INTO nv_cmds[],
	// precomputed and sorted by command character, so deleting two rows shifts
	// every later index while the precomputed table still points at the old
	// positions -- every normal command after them dispatches to the wrong
	// function.
	//
	// The first version of this phase deleted the rows.  It built, it swept
	// clean, it passed the linkage and symbol checks -- and 39 of the 67
	// behaviour cases moved: CTRL-A, joins, macros, marks, multibyte motions,
	// nothing to do with K or tags.
	q.InTable("nv_cmds", func(q *graph.Verbs) {
		q.RewriteAt("(init Ctrl_RSB ?f NV_NCW 0)", "f", "nv_error", 1, "the CTRL-] row")
		q.RewriteAt("(init 'K' ?f 0 0)", "f", "nv_error", 1, "the K row")
	})
	if q.Failed() {
		return fmt.Errorf("noident: the CTRL-] and K rows are not where this expects -- %v", q.Err)
	}
	q.InFunction("nv_g_cmd", func(q *graph.Verbs) {
		q.DropCase("(case Ctrl_RSB)", 1, "g CTRL-]")
		q.DropCase("(case ']')", 1, "g]")
	})
	if q.Failed() {
		return fmt.Errorf("noident: nv_g_cmd's tag cases are not where this expects -- %v", q.Err)
	}
	v.Say("K and CTRL-] answer nv_error, and g] leaves nv_g_cmd")

	// do_window's CTRL-W ] and CTRL-W g] went with do_window at phase 1
	// (nowindows, the reform's D9)
	v.Sayf("%d nv_K_getcmd/do_nv_ident mentions left for the sweep", len(identLeft.FindAll(v.Text(), -1)))
	return v.Done()
}
