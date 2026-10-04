package p096

// Whim phase 96 (formerly 177) -- a line's match on its own.  See GOAL.md.

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim96", Edit) }

// alone is the guard of what the engine does only on the buffer: its
// state's line alone is not set.
const alone = "re->alone.string != nullptr"

// found is what match_lines answers with: each search of a line alone --
// where it started, what it returned and the positions it left (those past
// the last set are all -1, as the engine leaves them) -- and a line's
// searches, in the order ex_substitute's loop will make them.
const found = `typedef struct
{
    colnr_T col;
    long nmatch;
    colnr_T matchcol;
    int nsub;
    int pos;
} regsearch_T;

typedef struct
{
    regsearch_T *searches;
    lpos_T *pos;
    int n;
    int next;
} linefound_T;

static bool match_lines(regmmatch_T *rmp, bool do_all, buf_T *buf, string_T *lines, linenr_T line1, linenr_T n, linefound_T *found);
`

// matchLines is the matcher of lines each on its own, written after the
// engine's entry it runs: match_chunk runs one engine over lines [from,
// to), searching each where ex_substitute's loop will -- from column 0,
// then, with g, from where each match ended, a step on after an empty match
// where the last one ended, until the line's end -- and match_lines one
// over them all: the sequential meaning of what a target may run on many
// engines at once, each over a chunk.
const matchLines = `    static void
match_chunk(regengine_T *re, regmmatch_T *rmp, bool do_all, buf_T *buf, string_T *lines, linenr_T line1, linenr_T from, linenr_T to, linefound_T *found)
{
    regmmatch_T m = *rmp;
    garray_T searches;
    garray_T pos;
    ga_init2(&searches, sizeof(regsearch_T), 64);
    ga_init2(&pos, sizeof(lpos_T), 64);
    for (linenr_T i = from; i < to && !re->failed; ++i)
    {
        char_u *line = lines[i].string;
        colnr_T col = 0;
        colnr_T matchcol = 0;
        colnr_T prev_matchcol = MAXCOL;
        int first = searches.ga_len;
        re->alone = lines[i];
        for (;;)
        {
            long r = bt_regexec_multi(re, &m, curwin, buf, line1 + i, col, nullptr);
            if (re->failed || !ga_grow(&searches, 1) || !ga_grow(&pos, 2 * NSUBEXP))
            {
                re->failed = true;
                break;
            }
            regsearch_T *s = (regsearch_T *)searches.ga_data + searches.ga_len;
            ++searches.ga_len;
            s->col = col;
            s->nmatch = r <= 0 ? 0 : r;
            s->matchcol = m.rmm_matchcol;
            s->nsub = 0;
            s->pos = pos.ga_len;
            if (s->nmatch == 0)
            {
                break;
            }
            for (int k = 0; k < NSUBEXP; ++k)
            {
                if (m.startpos[k].lnum != -1 || m.startpos[k].col != -1 || m.endpos[k].lnum != -1 || m.endpos[k].col != -1)
                {
                    s->nsub = k + 1;
                }
            }
            for (int k = 0; k < s->nsub; ++k)
            {
                ((lpos_T *)pos.ga_data)[pos.ga_len++] = m.startpos[k];
                ((lpos_T *)pos.ga_data)[pos.ga_len++] = m.endpos[k];
            }
            if (!do_all)
            {
                break;
            }
            if (matchcol == prev_matchcol && m.endpos[0].col == matchcol)
            {
                if (line[matchcol] == NUL)
                {
                    break;
                }
                matchcol += utfc_ptr2len(line + matchcol);
            }
            else
            {
                matchcol = m.endpos[0].col;
                prev_matchcol = matchcol;
            }
            if (line[matchcol] == NUL)
            {
                break;
            }
            col = matchcol;
        }
        found[i].n = searches.ga_len - first;
        found[i].next = first;
    }
    for (linenr_T i = from; i < to; ++i)
    {
        found[i].searches = (regsearch_T *)searches.ga_data + found[i].next;
        found[i].pos = (lpos_T *)pos.ga_data;
        found[i].next = 0;
    }
}

    static bool
match_lines(regmmatch_T *rmp, bool do_all, buf_T *buf, string_T *lines, linenr_T line1, linenr_T n, linefound_T *found)
{
    regengine_T *re = (regengine_T *)alloc_clear(sizeof(regengine_T));
    match_chunk(re, rmp, do_all, buf, lines, line1, 0, n, found);
    return !re->failed;
}

`

// searchFound is match_range, which snapshots the lines of a range and
// matches them, and search_found: a search ex_substitute's loop makes, answered by what
// match_lines found when it holds that search -- the line's next one,
// from the same column -- and made otherwise; a line whose search was not
// held has none answered after it.
const searchFound = `    static linefound_T *
match_range(regmmatch_T *rmp, bool do_all, linenr_T line1, linenr_T line2)
{
    linenr_T n = line2 - line1 + 1;
    string_T *lines = (string_T *)alloc(sizeof(string_T) * n);
    for (linenr_T k = 0; k < n; ++k)
    {
        lines[k].string = ml_get(line1 + k);
        lines[k].length = ml_get_len(line1 + k);
    }
    linefound_T *found = (linefound_T *)alloc_clear(sizeof(linefound_T) * n);
    return match_lines(rmp, do_all, curbuf, lines, line1, n, found) ? found : nullptr;
}

    static long
search_found(linefound_T *found, linenr_T line1, linenr_T count, regmmatch_T *rmp, linenr_T lnum, colnr_T col)
{
    if (found != nullptr)
    {
        linefound_T *f = &found[lnum - line1 - (curbuf->b_ml.ml_line_count - count)];
        if (f->next < f->n && f->searches[f->next].col == col)
        {
            regsearch_T *s = &f->searches[f->next];
            ++f->next;
            if (s->nmatch > 0)
            {
                for (int k = 0; k < NSUBEXP; ++k)
                {
                    if (k < s->nsub)
                    {
                        rmp->startpos[k] = f->pos[s->pos + 2 * k];
                        rmp->endpos[k] = f->pos[s->pos + 2 * k + 1];
                    }
                    else
                    {
                        rmp->startpos[k].lnum = -1;
                        rmp->startpos[k].col = -1;
                        rmp->endpos[k].lnum = -1;
                        rmp->endpos[k].col = -1;
                    }
                }
                rmp->rmm_matchcol = s->matchcol;
            }
            return s->nmatch;
        }
        f->next = f->n;
    }
    return vim_regexec_multi(rmp, curwin, curbuf, lnum, col, nullptr);
}

`

// Edit gives the regex engine a mode in which it matches one line handed to
// it, and nothing else -- no other line of the buffer, no host, no message,
// no store into the compiled pattern -- and says so when a match would have
// needed more; adds match_lines(), which makes each line's searches of a
// range that way, in the order ex_substitute()'s loop will make them, and
// records them; and has the loop take a search's answer from the record
// when it holds that search.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): every act is FRAG, in ONE unit
// (Together): the engine's struct written anew with its two members (a
// member is no FRAG spot: its definition's C with the text's literal
// applied), each place found by its form -- a statement, a case label, a
// call, a declaration -- and the functions put beside the forms the text's
// literals named; an error call is moved into its else as a hole, its nodes
// kept.  History keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("alone", e, w)
	is := func(n *graph.Node, pat string) bool {
		p, err := clisp.Pattern(pat)
		return err == nil && n != nil && graph.Matches(p, n)
	}
	sibling := func(n *graph.Node, k int) *graph.Node {
		ks := e.Parent(n).Kids
		for i, x := range ks {
			if x == n && i+k >= 0 && i+k < len(ks) {
				return ks[i+k]
			}
		}
		return nil
	}
	items := func(v *graph.Verbs, pat string, ok func(*graph.Node) bool) []*graph.Node {
		var out []*graph.Node
		for _, m := range v.Find(pat) {
			if e.Item(m) == m && (ok == nil || ok(m)) {
				out = append(out, m)
			}
		}
		return out
	}
	member := v.One("(reg_toolong int)", "the engine's state")
	vrm := e.Defn("vim_regexec_multi")
	var vrc *graph.Node
	for _, d := range e.FileDecls("vim_regcomp") {
		if !d.Is("defn") {
			vrc = d
		}
	}
	es := e.Defn("ex_substitute")
	if vrm == nil || vrc == nil || es == nil {
		v.Die("vim_regexec_multi(), vim_regcomp()'s prototype or ex_substitute() is not where this phase expects it")
	}
	if v.Failed() {
		return v.Done()
	}
	st := e.TopForm(member)
	sc, err := graph.FormsC([]*graph.Node{st})
	if err != nil {
		return err
	}
	const tail = "    unsigned reg_tofreelen;\n    int reg_toolong;\n};\n"
	if strings.Count(string(sc), tail) != 1 {
		v.Die("the engine's state does not end where this phase expects")
		return v.Done()
	}
	v.Together(func(v *graph.Verbs) {
		v.FragAt(e.SpotOf(st), strings.Replace(string(sc), tail, "    unsigned reg_tofreelen;\n    int reg_toolong;\n    string_T alone;\n    bool failed;\n};\n", 1),
			"the engine's state holds the line it matches alone, and whether a match needed more")
		// reading a line: the one it holds, and any other fails the match alone
		v.InFunction("reg_getline_common", func(v *graph.Verbs) {
			v.AfterC("(def get_length int (& flags RGLF_LENGTH))", "if ("+alone+" && lnum != 0)\n{\n    re->failed = true;\n}\n", 1,
				"a line other than the one it holds fails the match alone")
			v.BeforeC("(if get_line (block (= (deref line) (call ml_get_buf _*)) _*) _*)",
				"if ("+alone+")\n{\n    if (get_line)\n    {\n        *line = lnum == 0 ? re->alone.string : (char_u *)\"\";\n    }\n"+
					"    if (get_length)\n    {\n        *length = lnum == 0 ? (colnr_T)re->alone.length : 0;\n    }\n    return;\n}\n", 1,
				"and the buffer is not read")
		})
		// the host is not polled
		poll := "if (!(" + alone + "))\n{\n    fast_breakcheck();\n}\n"
		v.InFunction("reg_nextline", func(v *graph.Verbs) {
			v.ReplaceEachC(items(v, "(call fast_breakcheck)", nil), poll, 1, "reg_nextline polls the host only on the buffer")
		})
		v.InFunction("regmatch", func(v *graph.Verbs) {
			v.ReplaceEachC(items(v, "(call fast_breakcheck)", nil), poll, 2, "and so does regmatch")
			// what a line's match reads besides its text: the cursor, a mark,
			// the Visual area, the line's number, the file's first and last
			// line, a virtual column
			ops := map[string]bool{"RE_BOF": true, "RE_EOF": true, "CURSOR": true, "RE_MARK": true, "RE_VISUAL": true, "RE_LNUM": true, "RE_VCOL": true}
			v.AfterEachC(items(v, "(case ?op)", func(m *graph.Node) bool { return !m.Kids[1].IsList() && ops[m.Kids[1].Atom] }),
				"if ("+alone+")\n{\n    re->failed = true;\n    status = RA_NOMATCH;\n    break;\n}\n", 7,
				"an atom that reads more than the line fails the match alone")
		})
		// no message: the match alone fails, and the buffer's run says it
		for _, f := range []struct {
			name string
			n    int
		}{{"prog_magic_wrong", 1}, {"regrepeat", 1}, {"regstack_push", 1}, {"regmatch", 5}, {"bt_regexec_both", 1}} {
			v.InFunction(f.name, func(v *graph.Verbs) {
				var calls []*graph.Node
				for _, fn := range []string{"emsg", "iemsg", "internal_error"} {
					calls = append(calls, items(v, "(call "+fn+" _*)", nil)...)
				}
				v.WrapEachC(calls, "c", "if ("+alone+")\n{\n    re->failed = true;\n}\nelse\n{\n    $c;\n}\n", f.n,
					"an error fails the match alone and gives no message")
			})
		}
		// the must-have string's length is the program's, and a match alone
		// does not write it: two may run on one program at once
		v.InFunction("bt_regexec_both", func(v *graph.Verbs) {
			must := items(v, "(if (!= (-> prog regmust) nullptr) (block (def c int) _*))", nil)
			v.BeforeEachC(must, "if (prog->regmust != nullptr && "+alone+" && (re->rex.reg_ic || re->rex.reg_icombine))\n{\n    re->failed = true;\n    break;\n}\n", 1,
				"the must-have string is looked for through a local length; alone where case is ignored the match fails, since the length may shrink and a later search read it")
			var cs []*graph.Node
			if len(must) == 1 {
				cs = []*graph.Node{must[0].Kids[2].Kids[1]}
			}
			v.AfterEachC(cs, "int regmlen = prog->regmlen;\n", 1, "with the length in a local")
			v.ReplaceC("(call cstrncmp__regmlen re s (-> prog regmust) prog)", "cstrncmp(re, s, prog->regmust, &regmlen)", 2, "both looks")
			v.BeforeEachC(items(v, "(if (== s nullptr) (block (break)))", func(m *graph.Node) bool {
				p := sibling(m, -1)
				return p != nil && len(graph.Find(p, func(x *graph.Node) bool { return is(x, "(call cstrncmp__regmlen _*)") })) > 0
			}), "if (!("+alone+"))\n{\n    prog->regmlen = regmlen;\n}\n", 1,
				"the length is written back on the buffer, as each look did")
		})
		v.FragAt(e.SpotBefore(vrm), matchLines, "match_chunk() and match_lines(), after the engine's entry they run")
		v.FragAt(e.SpotAfter(vrc), "\n"+found, "what match_lines() finds, and its prototype, for ex_substitute")
		v.FragAt(e.SpotBefore(es), searchFound, "search_found(), before ex_substitute")
		v.In(es, func(v *graph.Verbs) {
			v.AfterC("(= line2 (-> eap line2))", "linefound_T *found = !subflags.do_ask && line2 > eap->line1 ? match_range(&regmatch, subflags.do_all, eap->line1, line2) : nullptr;\nlinenr_T found_count = curbuf->b_ml.ml_line_count;\n", 1,
				"ex_substitute matches the range's lines each alone first")
			v.ReplaceC("(= nmatch (call vim_regexec_multi (addr regmatch) curwin curbuf lnum (cast colnr_T 0) nullptr))",
				"nmatch = search_found(found, eap->line1, found_count, &regmatch, lnum, (colnr_T)0);\n", 1, "and its searches take what that found")
			var next []*graph.Node
			for _, m := range v.Find("(= nmatch (call vim_regexec_multi (addr regmatch) curwin curbuf sub_firstlnum matchcol nullptr))") {
				if is(e.Parent(m), "(== _ 0)") {
					next = append(next, m)
				}
			}
			v.ReplaceEachC(next, "(nmatch = search_found(found, eap->line1, found_count, &regmatch, sub_firstlnum, matchcol))", 1,
				"the search for a line's next match, before the line is replaced, too")
		})
	})
	return v.Done()
}
