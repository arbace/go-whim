// Package ed is the editing server in Joker, the namespace ed: a session
// on a graph read from its EDN, its buffers and their journal --
// crefactor/graph/view's Server, what `whim view-serve` answers on stdin --
// driven by forms (doc/LISP-SANDBOX.md, *The editor in the box*).  In the
// box there is no C compiler: the configuration it would give and the
// headers it reads come from the store, a bundle `whim store DIR cc FILE`
// writes (crefactor/cc's Host), so that an edit's fragment is parsed and
// checked there as on the host.
//
//	(ed/start edn cc)  a session on the graph edn, cc the C host's bundle
//	                   (nil: the host's compiler, as jokerhost has one);
//	                   the number of nodes
//	(ed/req line)      one request, view-serve's: {:head H :body B}, or an
//	                   error thrown with its message
//	(ed/! line)        the same, the body printed: the head
//
// `write NAME` puts the C in the store and points ref NAME at it.  A
// buffer opened --fallout closes over a deletion's uses with vim's options
// (internal/whim/vimgraph).
package ed

import (
	"errors"
	"fmt"
	"time"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/crefactor/graph/view"
	"github.com/arbace/go-whim/guest/joker/box"
	"github.com/arbace/go-whim/internal/whim/vimgraph"
	. "github.com/candid82/joker/core"
)

// Install interns the namespace ed's functions, the C written to s.
// Joker's global environment must be initialised and its lock held.
func Install(s box.Store) {
	var sv *view.Server
	ns := GLOBAL_ENV.EnsureSymbolIsNamespace(MakeSymbol("ed"))
	def := func(name string, fn func(args []Object) Object) {
		ns.Intern(MakeSymbol(name)).Value = &Proc{Fn: fn, Name: "ed/" + name}
	}
	fail := func(what string, err error) { panic(RT.NewError("ed/" + what + ": " + err.Error())) }
	def("start", func(args []Object) Object {
		CheckArity(args, 2, 2)
		edn := EnsureArgIsString(args, 0).S
		if _, none := args[1].(Nil); none {
			cc.SetHost(nil)
		} else {
			h, err := cc.ReadBundle([]byte(EnsureArgIsString(args, 1).S))
			if err != nil {
				fail("start", err)
			}
			cc.SetHost(h)
		}
		t := time.Now()
		g, err := graph.ReadEDN([]byte(edn))
		if err != nil {
			fail("start", err)
		}
		n := 0
		g.Walk(func(*graph.Node) bool { n++; return true })
		fo := vimgraph.FallOut
		sv = view.NewServer(g, "<store>", view.ServerOptions{FallOut: &fo, Write: func(name string, c []byte) error {
			if s == nil {
				return errors.New("no store to write to")
			}
			h, err := s.Put(c)
			if err != nil {
				return err
			}
			return s.SetRef(name, h, nil)
		}})
		return NewHashMap(
			MakeKeyword("nodes"), MakeInt(n),
			MakeKeyword("read-ms"), MakeInt(int(time.Since(t).Milliseconds())))
	})
	req := func(args []Object) (string, string) {
		CheckArity(args, 1, 1)
		if sv == nil {
			panic(RT.NewError("ed: no session: (ed/start edn cc) first"))
		}
		head, body, err := sv.Answer(EnsureArgIsString(args, 0).S)
		if err != nil {
			panic(RT.NewError(err.Error()))
		}
		return head, body
	}
	def("req", func(args []Object) Object {
		head, body := req(args)
		return NewHashMap(MakeKeyword("head"), MakeString(head), MakeKeyword("body"), MakeString(body))
	})
	def("!", func(args []Object) Object {
		head, body := req(args)
		fmt.Fprint(Stdout, body)
		return MakeString(head)
	})
}
