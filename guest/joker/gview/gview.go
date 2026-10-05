// Package gview is crefactor/graph/view's named views in Joker: the
// Clojure of crefactor/graph/view/clj/src/gview ported to the Clojure
// dialect forked in ../joker (doc/LISP-SANDBOX.md, *The views in Joker, in
// the box*).  The sources are Joker -- graph.joke, view.joke, main.joke,
// the namespaces gview.graph, gview.view and gview.main -- embedded here so
// that the host runner (../cmd/jokerhost) and the guest (../main.go) load
// the same bytes with no file system.
package gview

import (
	"embed"
	"strings"

	. "github.com/candid82/joker/core"
)

//go:embed graph.joke view.joke main.joke
var src embed.FS

// Files are the namespaces' sources in the order they load: each requires
// only those before it.
var Files = []string{"graph.joke", "view.joke", "main.joke"}

// Load evaluates the three namespaces into Joker's global environment,
// which must be initialised (InitEnv, ProcessCoreData) and its lock held.
// The current namespace is put back after.
func Load() error {
	cur := GLOBAL_ENV.CurrentNamespace()
	defer GLOBAL_ENV.SetCurrentNamespace(cur)
	for _, f := range Files {
		b, err := src.ReadFile(f)
		if err != nil {
			return err
		}
		if err := ProcessReader(NewReader(strings.NewReader(string(b)), f), "", EVAL); err != nil {
			return err
		}
	}
	return nil
}

// Source is one namespace's source, by its file's name.
func Source(name string) ([]byte, error) { return src.ReadFile(name) }

// SetVar interns ns/name, the namespace created when it is missing, and
// gives it a string.
func SetVar(ns, name, value string) {
	n := GLOBAL_ENV.EnsureSymbolIsNamespace(MakeSymbol(ns))
	n.Intern(MakeSymbol(name)).Value = MakeString(value)
}
