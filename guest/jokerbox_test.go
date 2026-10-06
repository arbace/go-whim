package guest

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arbace/go-whim/vmm"
)

// txnForms use the store's transactions (box/txn.go): committed once and
// spent; two on one snapshot, the second refused; one escaped from
// with-txn spent; an error in with-txn's body leaving the ref as it was.
const txnForms = `(def t (box/txn "n"))
(println "<<1" (box/commit! t "a") (box/spent? t) (box/load "n") ">>")
(println "<<2" (try (box/commit! t "b") (catch Error e (ex-message e))) ">>")
(def t1 (box/txn "n"))
(def t2 (box/txn "n"))
(println "<<3" (box/txn-value t1) (box/commit! t1 "x") (box/commit! t2 "y") (box/load "n") ">>")
(def leak (box/with-txn [t "n"] (box/commit! t "z") t))
(println "<<4" (box/load "n") (box/spent? leak) (try (box/commit! leak "q") (catch Error e (ex-message e))) ">>")
(println "<<5" (try (box/with-txn [t "n"] (throw (ex-info "boom" {}))) (catch Error e (ex-message e))) (box/load "n") ">>")
`

var txnWant = []string{
	"<<1 true true a >>",
	"<<2 box/commit!: the transaction on n is spent >>",
	"<<3 a true false x >>",
	"<<4 z true box/commit!: the transaction on n is spent >>",
	"<<5 boom z >>",
}

// TestJokerTxn holds the transactions on the host runner and in the box.
func TestJokerTxn(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	check := func(t *testing.T, out string) {
		for _, w := range txnWant {
			if !strings.Contains(out, w) {
				t.Fatalf("no %q in:\n%s", w, out)
			}
		}
	}
	t.Run("host", func(t *testing.T) {
		if _, err := exec.LookPath("go"); err != nil {
			t.Skip("no go")
		}
		host := filepath.Join(t.TempDir(), "jokerhost")
		if err := JokerHost(host); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(host, "--store", t.TempDir())
		cmd.Stdin = strings.NewReader(txnForms)
		b, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v\n%s", err, b)
		}
		check(t, string(b))
	})
	t.Run("box", func(t *testing.T) {
		img := jokerImage(t)
		st, err := vmm.OpenDirStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		con := &console{in: []byte(txnForms)}
		if _, err := vmm.Run(vmm.Config{Image: img, Host: con, Store: st, Args: []string{"joker"}}); err != nil {
			t.Fatal(err)
		}
		check(t, con.out.String())
	})
}
