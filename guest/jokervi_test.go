package guest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/vmm"
)

// viKeys drive the modal editor (gview/vi.joke) through what it does: on
// def get, down to `(= opt 0)`, the 0 deleted and 2 typed (the edit
// pending, then applied); then on it again undo, :rename at the cursor, :w,
// and a form evaluated on the command line.  After each session the REPL
// prints what the editor ended with and the graph's C.
const viKeys = `(def st (gview.vi/edit "def get"))
jjjjjj$hxi2` + "\x1b" + `:q
(println "<<edit" (:msg st) ">>")
(println "<<c" (:body (ed/req "c get")) ">>")
(def st (gview.vi/edit "def get"))
u:rename n2
:w edited
:(+ 1 2)
:q
(println "<<last" (:msg st) ">>")
(println "<<renamed" (:body (ed/req "c n2")) ">>")
`

// viWant is what viKeys must print: the edit applied, its C, undone and
// the function renamed, the command line's value last.
var viWant = []string{"<<edit applied ", "opt = 2;", "<<last 3 >>", "n2(buf_T *b, int n)", "opt = 0;"}

// TestJokerVi runs the modal editor on the host runner, its keys from
// stdin, on the views' sample in a store of the test's.
func TestJokerVi(t *testing.T) {
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
	st := viStore(t, "../crefactor/graph/view/testdata/sample.c", false)
	cmd := exec.Command(host, "--gview", "--store", st.dir)
	cmd.Stdin = strings.NewReader(`(ed/start (box/load "graph") nil)` + "\n" + viKeys)
	cmd.Env = append(os.Environ(), "LINES=12", "COLUMNS=60")
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	viCheck(t, string(b), st.DirStore)
}

// TestJokerViGuest is the same session in the box, on whim-vim.c's graph
// and the C host's bundle: the edit's fragment checked on the bundle's
// headers, as the editor in the box is (TestJokerEditGuest).
func TestJokerViGuest(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	img := jokerImage(t)
	st := viStore(t, "../src/whim-vim.c", true)
	keys := strings.ReplaceAll(viKeys, `"def get"`, `"def vim_snprintf"`)
	keys = strings.Replace(keys, "jjjjjj$hxi2", "jjo  (= str_l 0)", 1)
	keys = strings.ReplaceAll(keys, `"c get"`, `"c vim_snprintf"`)
	// n2 is a local of win_redr_ruler, which a call renamed would name: the
	// rename is refused there, rightly
	keys = strings.NewReplacer(":rename n2", ":rename vsnp2", `"c n2"`, `"c vsnp2"`).Replace(keys)
	con := &console{in: []byte(`(ed/start (box/load "graph") (box/load "cc"))` + "\n" + keys)}
	code, err := vmm.Run(vmm.Config{Image: img, Host: con, Store: st.DirStore, Args: []string{"joker"}})
	if err != nil {
		t.Fatal(err)
	}
	out := con.out.String()
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	for _, want := range []string{"<<edit applied ", "    str_l = 0;\n    va_start(ap, fmt);", "<<last 3 >>", "vsnp2(char *str, usize str_m", "    va_end(ap);"} {
		if !strings.Contains(out, want) {
			t.Fatalf("no %q in:\n%s", want, tail(out))
		}
	}
	if _, ok := st.Ref("edited"); !ok {
		t.Fatal(":w wrote no ref")
	}
}

type viDir struct {
	*vmm.DirStore
	dir string
}

// viStore is a store holding path's graph as the ref graph, and with cc
// the C host's bundle as the ref cc.
func viStore(t *testing.T, path string, cc bool) viDir {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	g, _, err := graph.Import(path, src)
	if err != nil {
		t.Fatal(err)
	}
	edn, err := g.EDN()
	if err != nil {
		t.Fatal(err)
	}
	blobs := map[string][]byte{"graph": edn}
	if cc {
		if blobs["cc"], err = graph.HostBundle(path, src); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	st, err := vmm.OpenDirStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	for name, b := range blobs {
		h, err := st.Put(b)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.SetRef(name, h, nil); err != nil {
			t.Fatal(err)
		}
	}
	return viDir{st, dir}
}

func viCheck(t *testing.T, out string, st *vmm.DirStore) {
	t.Helper()
	for _, want := range viWant {
		if !strings.Contains(out, want) {
			t.Fatalf("no %q in:\n%s", want, tail(out))
		}
	}
	if _, ok := st.Ref("edited"); !ok {
		t.Fatal(":w wrote no ref")
	}
}

// tail is out's last 3,000 bytes, the screens' escapes taken out.
func tail(out string) string {
	if len(out) > 3000 {
		out = out[len(out)-3000:]
	}
	return strings.NewReplacer("\x1b[H", "", "\x1b[2J", "", "\x1b[7m", "", "\x1b[0m", "").Replace(out)
}
