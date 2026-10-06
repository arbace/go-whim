package guest

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/graph/view"
	"github.com/arbace/go-whim/editor"
	"github.com/arbace/go-whim/vmm"
)

// The graph views in Joker (guest/joker/gview, doc/LISP-SANDBOX.md, *The
// views in Joker, in the box*) held to the Go's, as crefactor/graph/view's
// clj_test.go holds the Clojure's: every case's text the same bytes as
// `whim view` prints, and the same error where there is one -- on the
// host, by the host runner (guest/joker/cmd/jokerhost view --batch), and
// in the box, by the Joker guest's REPL.

// TestJokerViews runs the views' cases on the host runner: on the views'
// sample, the golden views and, for every name it declares, uses, and def
// and callers of every function, type of every struct a typedef names; on
// whim-vim.c's graph, clj_test.go's fixed cases and every 25th of its
// generated ones (JOKER_VIEWS_ALL=1: all of them) -- each on the bytecode
// VM and again compiled to closures (--closures, joker/core/closure.go).
func TestJokerViews(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go")
	}
	host := filepath.Join(t.TempDir(), "jokerhost")
	if err := JokerHost(host); err != nil {
		t.Fatal(err)
	}
	for _, mode := range [][]string{nil, {"--closures"}} {
		name := "vm"
		if mode != nil {
			name = "closures"
		}
		t.Run("sample/"+name, func(t *testing.T) {
			ix := viewIndex(t, "../crefactor/graph/view/testdata/sample.c")
			jokerBatch(t, host, mode, ix, append(sampleCases(), generatedCases(ix, 1)...))
		})
		t.Run("product/"+name, func(t *testing.T) {
			ix := viewIndex(t, "../src/whim-vim.c")
			every := 25
			if os.Getenv("JOKER_VIEWS_ALL") != "" {
				every = 1
			}
			jokerBatch(t, host, mode, ix, append(productCases(), generatedCases(ix, every)...))
		})
	}
}

// TestJokerViewsGuest: the same views in the box -- the Joker guest built
// with TamaGo, once, a graph's EDN put in a store of the test's as the ref
// graph, its REPL told to read it (box/load) and index it and to answer
// cases as the server does (gview.main/serve-lines) -- each answer the
// Go's: every sample case on the sample's graph, and clj_serve_test.go's
// fixed cases on whim-vim.c's; on the bytecode VM and compiled to
// closures (--closures).  It needs TamaGo and
// /dev/kvm, and skips without them.
func TestJokerViewsGuest(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	img := jokerImage(t)
	for _, mode := range [][]string{nil, {"--closures"}} {
		name := "vm"
		if mode != nil {
			name = "closures"
		}
		t.Run("sample/"+name, func(t *testing.T) {
			ix := viewIndex(t, "../crefactor/graph/view/testdata/sample.c")
			guestViews(t, img, mode, ix, append(sampleCases(), generatedCases(ix, 1)...))
		})
		t.Run("product/"+name, func(t *testing.T) {
			ix := viewIndex(t, "../src/whim-vim.c")
			guestViews(t, img, mode, ix, productCases())
		})
	}
}

// guestViews runs the Joker guest img on a store holding ix's graph and
// holds its answers to cases to the Go's.
func guestViews(t *testing.T, img []byte, mode []string, ix *view.Index, cases [][]string) {
	t.Helper()
	edn, err := ix.G.EDN()
	if err != nil {
		t.Fatal(err)
	}
	st, err := vmm.OpenDirStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h, err := st.Put(edn)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetRef("graph", h, nil); err != nil {
		t.Fatal(err)
	}
	var keys strings.Builder
	keys.WriteString("(def st (gview.main/open-string (box/load \"graph\")))\n(select-keys st [:read-ms :index-ms :nodes])\n(do (println \"<<frames>>\") (gview.main/serve-lines st [")
	for _, c := range cases {
		keys.WriteString(strconv.Quote(strings.Join(c, "\t")) + " ")
	}
	keys.WriteString("]) nil)\n")
	con := &console{in: []byte(keys.String())}
	start := time.Now()
	code, err := vmm.Run(vmm.Config{Image: img, Host: con, Store: st, Args: append([]string{"joker"}, mode...)})
	if err != nil {
		t.Fatal(err)
	}
	out := con.out.String()
	i := strings.Index(out, "<<frames>>\n")
	if code != 0 || i < 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	frames := parseFrames(t, out[i+len("<<frames>>\n"):], len(cases))
	bad := 0
	for i, c := range cases {
		want, wantErr := goView(ix, c)
		got := frames[i]
		if wantErr != "" {
			want = "error " + wantErr + "\n"
		} else {
			want = "ok " + want
		}
		if got != want {
			bad++
			if bad <= 5 {
				t.Errorf("%q: the guest's view differs:\n%s\nthe Go's:\n%s", c, got, want)
			}
		}
	}
	if bad > 0 {
		t.Fatalf("%d of %d views differ", bad, len(cases))
	}
	stats := ""
	if m := regexp.MustCompile(`\{:read-ms [^}]*\}`).FindString(out); m != "" {
		stats = ", " + m
	}
	t.Logf("%d of %d views the same bytes in the box (%v%s)", len(cases), len(cases), time.Since(start).Round(time.Millisecond), stats)
}

// console is a Host on which the guest's console reads in and writes out.
type console struct {
	recorder
	in []byte
}

func (c *console) WaitForInput(int64) bool { return true }
func (c *console) ReadInput(p []byte) int32 {
	n := copy(p, c.in)
	c.in = c.in[n:]
	return int32(n)
}
func (c *console) Exit(code int32) { panic(editor.Exit(code)) }

var frameHead = regexp.MustCompile(`^(ok|error) (\d+)\n`)

// parseFrames reads n answers framed as the server frames them, each
// "ok " and its text, or "error " and its message.
func parseFrames(t *testing.T, s string, n int) []string {
	t.Helper()
	var out []string
	for len(out) < n {
		m := frameHead.FindStringSubmatch(s)
		if m == nil {
			t.Fatalf("answer %d: no frame at %q", len(out)+1, s[:min(len(s), 200)])
		}
		size, _ := strconv.Atoi(m[2])
		body := s[len(m[0]) : len(m[0])+size]
		s = s[len(m[0])+size:]
		end := ";;end\n"
		if size > 0 && !strings.HasSuffix(body, "\n") {
			end = "\n;;end\n"
		}
		if !strings.HasPrefix(s, end) {
			t.Fatalf("answer %d: no end after %d bytes", len(out)+1, size)
		}
		s = s[len(end):]
		out = append(out, m[1]+" "+body)
	}
	return out
}

// viewIndex is a C file's graph, imported, written as Lisp and read back,
// as crefactor/graph/view's tests have it; it skips when there is no file.
func viewIndex(t *testing.T, path string) *view.Index {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("no %s", path)
	}
	g, _, err := graph.Import(filepath.Join(t.TempDir(), filepath.Base(path)), src)
	if err != nil {
		t.Fatal(err)
	}
	h, err := graph.Read(g.Lisp())
	if err != nil {
		t.Fatal(err)
	}
	return view.NewIndex(h)
}

// jokerBatch runs cases through jokerhost view --batch on ix's EDN and
// holds every one to goView.
func jokerBatch(t *testing.T, host string, mode []string, ix *view.Index, cases [][]string) {
	t.Helper()
	edn, err := ix.G.EDN()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ednFile, casesFile, out := filepath.Join(dir, "graph.edn"), filepath.Join(dir, "cases"), filepath.Join(dir, "out")
	var lines []string
	for _, c := range cases {
		lines = append(lines, strings.Join(c, "\t"))
	}
	if err := os.WriteFile(ednFile, edn, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(casesFile, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	cmd := exec.Command(host, append(append([]string(nil), mode...), "view", "--batch", casesFile, out, ednFile)...)
	cmd.Env = append(os.Environ(), "TMPDIR="+dir)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("jokerhost: %v\n%s", err, b)
	}
	t.Logf("jokerhost: %d views in %v: %s", len(cases), time.Since(start).Round(time.Millisecond), strings.TrimSpace(string(b)))
	bad := 0
	for i, c := range cases {
		want, wantErr := goView(ix, c)
		got, _ := os.ReadFile(filepath.Join(out, fmt.Sprint(i+1, ".out")))
		gotErr, _ := os.ReadFile(filepath.Join(out, fmt.Sprint(i+1, ".err")))
		ge := strings.TrimSpace(strings.TrimPrefix(string(gotErr), "  view-joker"))
		if !bytes.Equal(got, []byte(want)) || ge != wantErr {
			bad++
			if bad <= 5 {
				t.Errorf("%q: the Joker view differs (%d bytes against %d; error %q against %q)", c, len(got), len(want), ge, wantErr)
			}
		}
	}
	if bad > 0 {
		t.Fatalf("%d of %d views differ", bad, len(cases))
	}
	t.Logf("%d of %d views the same bytes", len(cases), len(cases))
}

// sampleCases are crefactor/graph/view's golden views on its sample, and
// each flag and error once.
func sampleCases() [][]string {
	return [][]string{
		{"callers", "fact"}, {"--depth", "0", "callers", "even"}, {"--ids", "--depth", "0", "callers", "even"},
		{"callees", "main"}, {"uses", "opt"}, {"--show", "fn", "uses", "fact"}, {"--show", "node", "uses", "buf_T"},
		{"member", "buf_T.b_ml"}, {"type", "buf_T"}, {"--ids", "def", "get"},
		{"--show", "node", "follow", "typed typed< ^fn", "get/b"},
		{"--show", "none", "callers", "fact"}, {"--stop", "defn", "--depth", "0", "callers", "even"},
		{"uses", "get/n"}, {"uses", "A"}, {"type", "struct other"}, {"--ids", "member", "other.b_ml"},
		{"uses", "#12"}, {"follow", "contains", "main"}, {"--depth", "0", "follow", "contains<", "#29"},
		{"uses", "no_such_name"}, {"member", "get"}, {"follow", "nostep", "main"}, {"uses", "#99999"},
		{"type", "nothing"}, {"uses", "get/zz"},
	}
}

// productCases are clj_serve_test.go's fixed cases on whim-vim.c.
func productCases() [][]string {
	return [][]string{
		{"uses", "p_wiv"}, {"--ids", "uses", "p_wiv"}, {"callers", "ml_get"}, {"--depth", "0", "callers", "ml_get"},
		{"--ids", "callers", "ml_get_buf"}, {"callees", "main"}, {"--depth", "0", "callees", "main"},
		{"member", "buf_T.b_ml"}, {"member", "pos_T.lnum"}, {"--ids", "member", "memline_T.ml_line_count"},
		{"type", "pos_T"}, {"type", "struct vimoption"}, {"--ids", "def", "ex_substitute"}, {"def", "options"},
		{"uses", "NUL"}, {"--show", "fn", "uses", "ml_get"}, {"--show", "node", "uses", "p_wiv"},
		{"--show", "none", "callers", "ml_get"}, {"uses", "ex_substitute/lnum"}, {"--stop", "defn", "callers", "ml_get"},
		{"follow", "refers< call ^fn", "ml_get"}, {"--depth", "3", "follow", "typed<", "pos_T"},
		{"uses", "no_such_name"}, {"member", "ml_get"},
	}
}

// generatedCases are clj_serve_test.go's generated ones -- for every name
// a top-level form declares, uses of it; def and callers to depth 1 of
// every function defined; type of every struct or union a typedef names --
// every one, or every Nth.
func generatedCases(ix *view.Index, every int) [][]string {
	top := map[string][]*graph.Node{}
	for _, f := range ix.G.Forms {
		if name := graph.DeclName(f); name != "" {
			top[name] = append(top[name], f)
		}
	}
	var names []string
	for name := range top {
		names = append(names, name)
	}
	sort.Strings(names)
	var cases [][]string
	for _, name := range names {
		cases = append(cases, []string{"uses", name})
		for _, d := range top[name] {
			switch {
			case d.Is("defn"):
				cases = append(cases, []string{"def", name}, []string{"--depth", "1", "callers", name})
			case d.Is("typedef"):
				if s, err := ix.Aggregate(name); err == nil && (s.Is("struct") || s.Is("union")) {
					cases = append(cases, []string{"type", name})
				}
			}
		}
	}
	var out [][]string
	for i, c := range cases {
		if i%every == 0 {
			out = append(out, c)
		}
	}
	return out
}

// goView is what `whim view ARGS` prints for one case, or its error:
// clj_test.go's, on crefactor/graph/view's exported names.
func goView(ix *view.Index, args []string) (string, string) {
	var opt view.Options
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
			opt.Show, _ = view.ParseShow(args[i])
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
	p := view.Printer{IDs: ids}
	for _, root := range roots {
		var t *view.Tree
		switch name {
		case "callers":
			t = view.Callers(ix, root, opt)
		case "callees":
			t = view.Callees(ix, root, opt)
		case "uses":
			t = view.Uses(ix, root, opt)
		case "member":
			t = view.Member(ix, root, opt)
		case "type":
			s, err := ix.Aggregate(rest[1])
			if err != nil {
				return "", err.Error()
			}
			t = view.Type(ix, s, opt)
		case "def":
			b.WriteString(p.Form(view.Def(ix, root)))
			continue
		case "follow":
			steps, err := view.ParseSteps(rest[1])
			if err != nil {
				return "", err.Error()
			}
			d := 1
			if opt.Depth < 0 {
				d = 0
			} else if opt.Depth > 0 {
				d = opt.Depth
			}
			t = view.Build(ix, root, view.Spec{Name: "follow", Child: "to", Rel: view.Steps(steps), Depth: d, Stop: opt.Stop, Show: opt.Show})
			t.Notes = append(t.Notes, "steps: "+rest[1])
		}
		b.WriteString(p.Tree(ix, t))
	}
	return b.String(), ""
}

// jokerImage is the Joker guest's image, built once for the package's
// tests.
func jokerImage(t *testing.T) []byte {
	t.Helper()
	gocmd := needTamaGo(t)
	jokerOnce.Do(func() {
		dir, err := os.MkdirTemp("", "joker.")
		if err != nil {
			jokerErr = err
			return
		}
		defer os.RemoveAll(dir)
		jokerImg, jokerErr = JokerImage(gocmd, Native(), dir)
	})
	if jokerErr != nil {
		t.Fatal(jokerErr)
	}
	return jokerImg
}

var (
	jokerOnce sync.Once
	jokerImg  []byte
	jokerErr  error
)

// TestJokerClosures holds the closure backend (joker/core/closure.go) to
// the bytecode VM on testdata/closures.joke, each form's output or error
// (the Joker REPL's lines) the same bytes either way.
func TestJokerClosures(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go")
	}
	host := filepath.Join(t.TempDir(), "jokerhost")
	if err := JokerHost(host); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile("testdata/closures.joke")
	if err != nil {
		t.Fatal(err)
	}
	run := func(mode ...string) string {
		cmd := exec.Command(host, mode...)
		cmd.Stdin = bytes.NewReader(src)
		b, _ := cmd.CombinedOutput()
		return string(b)
	}
	vm, cl := run(), run("--closures")
	if vm != cl {
		t.Fatalf("the VM:\n%s\nclosures:\n%s", vm, cl)
	}
	if !strings.Contains(vm, "still here 55") {
		t.Fatalf("the forms did not all run:\n%s", vm)
	}
	t.Logf("%d bytes of answers the same", len(vm))
}
