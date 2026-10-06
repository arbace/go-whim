package guest

import (
	"bytes"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/vmm"
)

// TestJokerEditGuest: the editor in the box (doc/LISP-SANDBOX.md, *The
// editor in the box*) -- the Joker guest's namespace ed on whim-vim.c's
// graph and the C host's bundle, both read from a store: a statement typed
// into vim_snprintf's view is applied (its fragment parsed and checked on
// the bundle's headers, va_list among them), the C printed has it, undo
// takes it back, and the C written to the store is whim-vim.c byte for
// byte; ml_clearmarked deleted whole is pending, being called, and applied
// in a buffer opened --fallout (vim's options, internal/whim/vimgraph),
// its call going with it.  Without the bundle the same edit is refused:
// the box has no compiler to ask.  It needs TamaGo and /dev/kvm.
func TestJokerEditGuest(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	img := jokerImage(t)
	const path = "../src/whim-vim.c"
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
	bundle, err := graph.HostBundle(path, src)
	if err != nil {
		t.Fatal(err)
	}
	st, err := vmm.OpenDirStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, b := range map[string][]byte{"graph": edn, "cc": bundle} {
		h, err := st.Put(b)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.SetRef(name, h, nil); err != nil {
			t.Fatal(err)
		}
	}
	run := func(cc string) string {
		keys := `(ed/start (box/load "graph") ` + cc + `)
(ed/req "open def vim_snprintf")
(def at (+ (joker.string/index-of (:body (ed/req "text")) "(def str_l int)") 15))
(println "<<change" (:head (ed/req (str "change " at " " at " \"\\n  (= str_l 0)\""))) ">>")
(println "<<c" (:body (ed/req "c vim_snprintf")) ">>")
(println "<<undo" (:head (ed/req "undo")) ">>")
(println "<<write" (:head (ed/req "write edited")) ">>")
(ed/req "open def ml_clearmarked")
(println "<<plain" (:head (ed/req (str "change 0 " (count (:body (ed/req "text"))) " \"\""))) ">>")
(ed/req "open --fallout def ml_clearmarked")
(println "<<fallout" (:head (ed/req (str "change 0 " (count (:body (ed/req "text"))) " \"\""))) ">>")
(println "<<gone" (try (ed/req "c ml_clearmarked") (catch Error e (ex-message e))) ">>")
`
		con := &console{in: []byte(keys)}
		code, err := vmm.Run(vmm.Config{Image: img, Host: con, Store: st, Args: []string{"joker"}})
		if err != nil {
			t.Fatal(err)
		}
		if code != 0 {
			t.Fatalf("exit %d:\n%s", code, con.out.String())
		}
		return con.out.String()
	}
	out := run(`(box/load "cc")`)
	for _, want := range []string{"<<change applied ", "    str_l = 0;\n    va_start(ap, fmt);", "<<undo  >>", "<<write " + strconv.Itoa(len(src)) + " >>",
		"<<plain pending ", "--fallout", "<<fallout applied ", "<<gone "} {
		if !strings.Contains(out, want) {
			t.Fatalf("no %q in:\n%s", want, out)
		}
	}
	h, ok := st.Ref("edited")
	if !ok {
		t.Fatal("no ref edited")
	}
	c := make([]byte, len(src)+1)
	n, _ := st.Get(h, 0, c)
	if !bytes.Equal(c[:n], src) {
		t.Fatalf("the C written (blob %s, %d bytes) is not whim-vim.c", hex.EncodeToString(h[:6]), n)
	}
	if out := run("nil"); strings.Contains(out, "<<change applied") {
		t.Fatalf("an edit applied with no C host:\n%s", out)
	}
}
