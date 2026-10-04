package graphcheck

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/arbace/go-whim/crefactor/cemit"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/steps"
)

// FRAG on the real snapshots (doc/GRAPH-MIGRATION.md, B2a): literal C the
// phases splice -- read from their own editlit.go and edit.go, not retyped
// -- made graph nodes in context on q(N-1)'s graph read back from its Lisp,
// and held to the text program's splice printed canonically, byte for
// byte; and the graph after the splice held to the import of its C view
// (SameGraph: every refers edge, every typed edge, ids aside), so that the
// nodes FRAG made are the nodes the importer makes.
//
//   - a whole body: phase 68's syn_name2id_len (q067), phase 72's
//     ml_new_data (q071), each as the phase's literal writes it, spaced
//     by hand;
//   - a run of statements: phase 22's `:edit` block in do_ecmd (q021): a
//     header's function (stat) and struct (st_dev, st_ino), a typedef
//     of it (stat_T), a cast to a header's type (dev_t), a goto to the
//     function's label;
//   - external declarations: phase 39's host block (q038, before
//     host_jump): 16 definitions and objects, the headers' macros
//     (FD_ZERO, FD_SET, FD_ISSET, errno, SIG_IGN), types (struct termios,
//     sig_atomic_t, fd_set) and functions;
//   - builtins: phase 38's launcher (__builtin_setjmp, __builtin_longjmp)
//     appended to q038 without it;
//   - macros expanded: phase 42's 7 MIN and 16 MAX (q041), each the
//     header's own expansion with its arguments read from its text.
func TestFragOnSnapshots(t *testing.T) {
	dir, _, _, _ := setup(t)
	lit := func(file, name string) string { return phaseLit(t, file, name) }
	rows := fragTextRows(t)
	for _, c := range []struct {
		name   string
		before int
		in     func(in []byte) []byte // the text the act runs on: q(N-1), or made from a snapshot
		text   func(in []byte) []byte // the text program's act
		golden string                 // or its result's row in testdata/frag_text_verbs.md
		graph  func(v *graph.Verbs)
		refuse string // what FRAG says, for a splice it refuses
	}{
		{
			name: "body/68", before: 67,
			// e.Body("syn_name2id_len", lit("068/edit.go", "W68LookupBody"), "body")
			golden: "body/68",
			graph:  func(v *graph.Verbs) { v.BodyC("syn_name2id_len", lit("068/edit.go", "W68LookupBody"), "body") },
		},
		{
			name: "body/61", before: 60,
			// e.Body("buflist_findnr", lit("061/edit.go", "W61FindnrBody"), "body")
			golden: "body/61",
			graph:  func(v *graph.Verbs) { v.BodyC("buflist_findnr", lit("061/edit.go", "W61FindnrBody"), "body") },
		},
		{
			// phase 72's body names bh_data, a member the phase adds in an
			// earlier step: on q071 it is not there, and cc's check says so
			// at the fragment's own line
			name: "refused/72", before: 71,
			graph:  func(v *graph.Verbs) { v.BodyC("ml_new_data", lit("072/edit.go", "W72NewData"), "body") },
			refuse: "frag 1 line 16:7: type struct block_hdr {bh_id short_u} has no member named bh_data",
		},
		{
			name: "run/22", before: 21,
			//	e.InFunction("do_ecmd", func(e *edit.E) {
			//		e.Literal(lit("022/editlit.go", "w22OldOpen"), lit("022/editlit.go", "w22NewOpen"), 1, "run")
			//	})
			golden: "run/22",
			graph: func(v *graph.Verbs) {
				v.InFunction("do_ecmd", func(v *graph.Verbs) {
					v.LiteralC(lit("022/editlit.go", "w22OldOpen"), lit("022/editlit.go", "w22NewOpen"), 1, "run")
				})
			},
		},
		{
			// phase 21's literal acts, every one that puts C in, made in ONE
			// import (Together): two bodies, six runs in six functions, two
			// of them the same run twice
			name: "together/21", before: 20,
			//	l := func(n string) string { return lit("021/editlit.go", n) }
			//	e.Body("check_more", l("w21lit1"), "")
			//	e.Body("append_arg_number", l("w21lit2"), "")
			//	e.InFunction("parse_cmd_address", func(e *edit.E) { e.Literal(l("w21lit3"), l("w21lit4"), 1, "") })
			//	e.InFunction("address_default_all", func(e *edit.E) { e.Literal(l("w21lit5"), l("w21lit6"), 1, "") })
			//	e.InFunction("default_address", func(e *edit.E) { e.Literal(l("w21lit7"), l("w21lit8"), 1, "") })
			//	e.InFunction("get_address", func(e *edit.E) {
			//		e.Sub(`(?m)^([ \t]*)case ADDR_ARGUMENTS:\n[ \t]*lnum = curwin->w_arg_idx \+ 1;\n[ \t]*break;\n`,
			//			"${1}case ADDR_ARGUMENTS:\n${1}    lnum = 0;\n${1}    break;\n", 2, "")
			//		e.Sub(`(?m)^([ \t]*)case ADDR_ARGUMENTS:\n[ \t]*lnum = \(\(curwin\)->w_alist->al_ga\.ga_len\);\n[ \t]*break;\n`,
			//			"${1}case ADDR_ARGUMENTS:\n${1}    lnum = 0;\n${1}    break;\n", 1, "")
			//	})
			//	e.InFunction("invalid_range", func(e *edit.E) { e.Literal(l("w21lit12"), l("w21lit13"), 1, "") })
			//	e.InFunction("eval_vars", func(e *edit.E) { e.Literal(l("w21lit16"), l("w21lit17"), 1, "") })
			golden: "together/21",
			graph: func(v *graph.Verbs) {
				l := func(n string) string { return lit("021/editlit.go", n) }
				v.Together(func(v *graph.Verbs) {
					v.BodyC("check_more", l("w21lit1"), "check_more")
					v.BodyC("append_arg_number", l("w21lit2"), "append_arg_number")
					v.InFunction("parse_cmd_address", func(v *graph.Verbs) { v.LiteralC(l("w21lit3"), l("w21lit4"), 1, "3") })
					v.InFunction("address_default_all", func(v *graph.Verbs) { v.LiteralC(l("w21lit5"), l("w21lit6"), 1, "5") })
					v.InFunction("default_address", func(v *graph.Verbs) { v.LiteralC(l("w21lit7"), l("w21lit8"), 1, "7") })
					v.InFunction("get_address", func(v *graph.Verbs) {
						v.LiteralC("case ADDR_ARGUMENTS: lnum = curwin->w_arg_idx + 1; break;", "case ADDR_ARGUMENTS: lnum = 0; break;", 2, "twice")
						v.LiteralC("case ADDR_ARGUMENTS: lnum = ((curwin)->w_alist->al_ga.ga_len); break;", "case ADDR_ARGUMENTS: lnum = 0; break;", 1, "once")
					})
					v.InFunction("invalid_range", func(v *graph.Verbs) { v.LiteralC(l("w21lit12"), l("w21lit13"), 1, "12") })
					v.InFunction("eval_vars", func(v *graph.Verbs) { v.LiteralC(l("w21lit16"), l("w21lit17"), 1, "16") })
				})
			},
		},
		{
			name: "top/39", before: 38,
			text: func(in []byte) []byte {
				old := "\nstatic void *host_jump[5];\n"
				return bytes.Replace(in, []byte(old), []byte("\n"+lit("039/editlit.go", "w39Host")+old[1:]), 1)
			},
			graph: func(v *graph.Verbs) { v.TopBeforeC("host_jump", lit("039/editlit.go", "w39Host"), "the host block") },
		},
		{
			name: "builtin/38", before: 38,
			in: func(in []byte) []byte {
				i := bytes.Index(in, []byte("\nstatic void *host_jump[5];\n"))
				return append([]byte(nil), in[:i]...)
			},
			text: func(in []byte) []byte { return append(append([]byte(nil), in...), lit("038/edit.go", "whim38New")...) },
			graph: func(v *graph.Verbs) {
				e := v.Editor()
				if _, err := e.SpliceC(graph.Frag{At: e.SpotEnd(nil), Src: lit("038/edit.go", "whim38New")}); err != nil {
					v.Die("the launcher -- %v", err)
				}
			},
		},
		{
			name: "macros/42", before: 41,
			text: func(in []byte) []byte { return expandMinMax(t, in) },
			graph: func(v *graph.Verbs) {
				tmpl := minMaxTemplates(t)
				v.ExpandMacros([]string{"MIN", "MAX"}, func(name string, args []string) (string, error) {
					return strings.NewReplacer("ZZA", args[0], "ZZB", args[1]).Replace(tmpl[name]), nil
				}, 23, "7 MIN and 16 MAX")
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			in := snapOf(t, dir, c.before)
			row, golden := rows[c.golden]
			if c.golden != "" && (!golden || row.in != fmt.Sprintf("%x", sha256.Sum256(in))) {
				t.Skipf("q%03d: not the snapshot testdata/frag_text_verbs.md records for %s; skipped", c.before, c.golden)
			}
			if c.in != nil {
				in = c.in(in)
			}
			path := filepath.Join(t.TempDir(), "whim-vim.c")
			want := in
			if c.text != nil {
				var err error
				if want, err = cemit.Canonical(path, c.text(in)); err != nil {
					t.Fatal(err)
				}
			}
			t0 := time.Now()
			g, _, err := graph.Import(path, in)
			if err != nil {
				t.Fatal(err)
			}
			imp := time.Since(t0)
			lisp := g.Lisp()
			// GRAPH_MEASURE=N: the act N times more, each on the graph read
			// back afresh, and the median
			var runs []time.Duration
			if n, _ := strconv.Atoi(os.Getenv("GRAPH_MEASURE")); n > 0 && c.refuse == "" {
				for range n {
					h, err := graph.Read(lisp)
					if err != nil {
						t.Fatal(err)
					}
					v := graph.NewVerbs(c.name, graph.NewEditor(h), io.Discard)
					t0 := time.Now()
					c.graph(v)
					runs = append(runs, time.Since(t0))
				}
				slices.Sort(runs)
			}
			h, err := graph.Read(lisp)
			if err != nil {
				t.Fatal(err)
			}
			e := graph.NewEditor(h)
			v := graph.NewVerbs(c.name, e, io.Discard)
			t0 = time.Now()
			c.graph(v)
			frag := time.Since(t0)
			if len(runs) > 0 {
				frag = runs[len(runs)/2]
			}
			if c.refuse != "" {
				if err := v.Done(); err == nil || !strings.Contains(err.Error(), c.refuse) {
					t.Fatalf("%v, want a refusal saying %q", err, c.refuse)
				}
				t.Logf("refused: %v", v.Done())
			} else if err := v.Done(); err != nil {
				t.Fatal(err)
			}
			if err := e.Check(); err != nil {
				t.Fatal(err)
			}
			got, err := h.C()
			if err != nil {
				t.Fatal(err)
			}
			if golden {
				if d := fmt.Sprintf("%x", sha256.Sum256(got)); d != row.want || len(got) != row.n {
					t.Fatalf("the graph's C view is not the text verbs' (testdata/frag_text_verbs.md, %s): %d bytes, sha256 %s, against %d, %s",
						c.golden, len(got), d, row.n, row.want)
				}
			} else if !bytes.Equal(got, want) {
				t.Fatalf("the graph's C view is not the text's: %d bytes against %d\n%s", len(got), len(want), diffAt(got, want))
			}
			back, _, err := graph.Import(path, got)
			if err != nil {
				t.Fatal(err)
			}
			if err := graph.SameGraph(h, back); err != nil {
				t.Fatalf("the graph is not the import of its C view: %v", err)
			}
			given, gone := 0, 0
			for _, a := range e.Log {
				given += len(a.New)
				gone += len(a.Gone)
			}
			t.Logf("FRAG %v (the whole file's import: %v); %d acts, %d ids given, %d superseded, %d untyped",
				frag.Round(time.Millisecond), imp.Round(time.Millisecond), len(e.Log), given, gone, len(e.Untyped))
		})
	}
}

// fragTextRow is a row of testdata/frag_text_verbs.md: the digest of the
// snapshot a case runs on, and the digest and length of what the text
// verbs gave there, printed canonically.
type fragTextRow struct {
	in, want string
	n        int
}

// fragTextRows reads testdata/frag_text_verbs.md's fenced block, by case.
func fragTextRows(t *testing.T) map[string]fragTextRow {
	b, err := os.ReadFile("internal/graphcheck/testdata/frag_text_verbs.md")
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]fragTextRow{}
	fence := false
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(ln, "```") {
			fence = !fence
			continue
		}
		if fence {
			var name string
			var before int
			var r fragTextRow
			if _, err := fmt.Sscan(ln, &name, &before, &r.in, &r.want, &r.n); err != nil {
				t.Fatalf("frag_text_verbs.md: %q: %v", ln, err)
			}
			rows[name] = r
		}
	}
	return rows
}

// phaseLit is the string constant name in internal/phase/FILE, read from
// the phase's own source.
func phaseLit(t *testing.T, file, name string) string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), filepath.Join("internal/phase", file), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out string
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, id := range vs.Names {
			if id.Name == name && i < len(vs.Values) {
				out, found = constString(t, vs.Values[i]), true
			}
		}
		return true
	})
	if !found {
		t.Fatalf("no constant %s in %s", name, file)
	}
	return out
}

// constString is a string constant's value: literals joined by +.
func constString(t *testing.T, x ast.Expr) string {
	switch x := x.(type) {
	case *ast.BasicLit:
		s, err := strconv.Unquote(x.Value)
		if err != nil {
			t.Fatal(err)
		}
		return s
	case *ast.BinaryExpr:
		return constString(t, x.X) + constString(t, x.Y)
	case *ast.ParenExpr:
		return constString(t, x.X)
	}
	t.Fatalf("not a string constant: %T", x)
	return ""
}

// minMaxTemplates are the host's MIN and MAX, asked of the preprocessor as
// phase 42's build step asks.
func minMaxTemplates(t *testing.T) map[string]string {
	t.Helper()
	probe, err := steps.MinMax()
	if err != nil {
		t.Skip(err)
	}
	tmpl := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(probe)), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "<") {
			tmpl["MIN"] = line
		} else {
			tmpl["MAX"] = line
		}
	}
	return tmpl
}

var minMaxRE = regexp.MustCompile(`\b(MIN|MAX)\(`)

// expandMinMax is phase 42's text expansion: each MIN( and MAX( matched to
// its parenthesis, split at its top-level comma, the template filled.
func expandMinMax(t *testing.T, text []byte) []byte {
	tmpl := minMaxTemplates(t)
	for {
		m := minMaxRE.FindSubmatchIndex(text)
		if m == nil {
			return text
		}
		i, d, j := m[1]-1, 0, m[1]-1
		for ; j < len(text); j++ {
			if text[j] == '(' {
				d++
			} else if text[j] == ')' {
				if d--; d == 0 {
					break
				}
			}
		}
		inner := string(text[i+1 : j])
		d, k := 0, -1
		for x := 0; x < len(inner) && k < 0; x++ {
			switch inner[x] {
			case '(':
				d++
			case ')':
				d--
			case ',':
				if d == 0 {
					k = x
				}
			}
		}
		rep := strings.NewReplacer("ZZA", strings.TrimSpace(inner[:k]), "ZZB", strings.TrimSpace(inner[k+1:])).Replace(tmpl[string(text[m[2]:m[3]])])
		text = append(append(append([]byte(nil), text[:m[0]]...), rep...), text[j+1:]...)
	}
}

// diffAt is the lines around the first difference.
func diffAt(a, b []byte) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	lo := max(0, i-300)
	return "graph: ..." + string(a[lo:min(len(a), i+300)]) + "\ntext:  ..." + string(b[lo:min(len(b), i+300)])
}
