package view

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arbace/go-whim/crefactor/graph"
)

// TestClojureViews holds clj/, the named views written in Clojure over the
// graph's EDN (doc/GRAPH.md, *Views in Clojure, over the EDN*), to these:
// on whim-vim.c's graph written as EDN, clj/view-clj --batch prints for
// every case exactly what `whim view` prints -- the same bytes, and the
// same error where there is one.  The cases are a fixed set (each view and
// flag, unlimited depths, ids) and, for every name the file declares,
// `uses` of it, and for every function defined `def` and `callers` to
// depth 1, and `type` of every struct a typedef names.  It needs java and
// Clojure's jars (GRAPH_CLOJURE_CP, else ~/.m2), and skips without them.
func TestClojureViews(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	if _, err := exec.LookPath("java"); err != nil {
		t.Skip("no java")
	}
	if os.Getenv("GRAPH_CLOJURE_CP") == "" {
		home, _ := os.UserHomeDir()
		if _, err := os.Stat(filepath.Join(home, ".m2/repository/org/clojure/clojure/1.12.5/clojure-1.12.5.jar")); err != nil {
			t.Skip("no Clojure jar: GRAPH_CLOJURE_CP is not set and ~/.m2 has none")
		}
	}
	ix, _ := product(t)
	edn, err := ix.G.EDN()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ednFile := filepath.Join(dir, "graph.edn")
	if err := os.WriteFile(ednFile, edn, 0o644); err != nil {
		t.Fatal(err)
	}

	cases := clojureCases(ix)
	var lines []string
	for _, c := range cases {
		lines = append(lines, strings.Join(c, "\t"))
	}
	casesFile, out := filepath.Join(dir, "cases"), filepath.Join(dir, "out")
	if err := os.WriteFile(casesFile, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	cmd := exec.Command("clj/view-clj", "--batch", casesFile, out, ednFile)
	cmd.Env = append(os.Environ(), "TMPDIR="+dir)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("view-clj: %v\n%s", err, b)
	} else {
		t.Logf("view-clj: %d views in %v: %s", len(cases), time.Since(start).Round(time.Millisecond), strings.TrimSpace(string(b)))
	}
	bad := 0
	for i, c := range cases {
		want, wantErr := goView(ix, c)
		got, _ := os.ReadFile(filepath.Join(out, fmt.Sprint(i+1, ".out")))
		gotErr, _ := os.ReadFile(filepath.Join(out, fmt.Sprint(i+1, ".err")))
		ge := strings.TrimSpace(strings.TrimPrefix(string(gotErr), "  view-clj"))
		if string(got) != want || ge != wantErr {
			bad++
			if bad <= 5 {
				t.Errorf("%q: the Clojure view differs (%d bytes against %d; error %q against %q)", c, len(got), len(want), ge, wantErr)
			}
		}
	}
	if bad > 0 {
		t.Fatalf("%d of %d views differ", bad, len(cases))
	}
	t.Logf("%d of %d views the same bytes", len(cases), len(cases))
}

// goView is what `whim view ARGS` prints for one case (cmd/whim/view.go's
// dispatch, --c aside), or its error.
func goView(ix *Index, args []string) (string, string) {
	var opt Options
	ids := false
	var rest []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--ids":
			ids = true
		case "--depth":
			i++
			fmt.Sscan(args[i], &opt.Depth)
			if opt.Depth == 0 {
				opt.Depth = -1
			}
		case "--show":
			i++
			opt.Show, _ = ParseShow(args[i])
		case "--stop":
			i++
			opt.Stop = strings.Split(args[i], ",")
		default:
			rest = append(rest, args[i])
		}
	}
	name, need := rest[0], 2
	if name == "follow" {
		need = 3
	}
	roots, err := ix.Find(rest[need-1])
	if err == nil && name == "member" {
		p := ix.Parent(roots[0])
		if !(roots[0].Is("member") || p != nil && graph.IsTypeDef(p) && (p.Is("struct") || p.Is("union"))) {
			err = fmt.Errorf("%s is not a member: member is S.M", rest[need-1])
		}
	}
	if err != nil {
		return "", err.Error()
	}
	var b strings.Builder
	p := Printer{IDs: ids}
	for _, root := range roots {
		var t *Tree
		switch name {
		case "callers":
			t = Callers(ix, root, opt)
		case "callees":
			t = Callees(ix, root, opt)
		case "uses":
			t = Uses(ix, root, opt)
		case "member":
			t = Member(ix, root, opt)
		case "type":
			s, err := ix.Aggregate(rest[1])
			if err != nil {
				return "", err.Error()
			}
			t = Type(ix, s, opt)
		case "def":
			b.WriteString(p.Form(Def(ix, root)))
			continue
		case "follow":
			steps, err := ParseSteps(rest[1])
			if err != nil {
				return "", err.Error()
			}
			d := 1
			if opt.Depth < 0 {
				d = 0
			} else if opt.Depth > 0 {
				d = opt.Depth
			}
			t = Build(ix, root, Spec{Name: "follow", Child: "to", Rel: Steps(steps), Depth: d, Stop: opt.Stop, Show: opt.Show})
			t.Notes = append(t.Notes, "steps: "+rest[1])
		}
		b.WriteString(p.Tree(ix, t))
	}
	return b.String(), ""
}
