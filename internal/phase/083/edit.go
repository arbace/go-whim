package p083

// Whim phase 83 (formerly 160) -- no line getter takes a cookie.  See GOAL.md.
//
// do_cmdline() handed its line getter a void * cookie that every call passed
// as nullptr and no getter read.  It goes, with find_func_t, a typedef nothing
// names: what is left of void * in the core is memory.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim83", Edit) }

// Edit takes the cookie Out of the line getters.
//
// do_cmdline() takes a function that gets the next line and a void * it hands
// that function, which vim's script and user-function readers used.  Here
// every call passes nullptr, do_one_cmd() and :append's reader only pass it
// on, and neither getter left -- getexline(), getcmdkeycmd() -- reads it: a
// parameter that is always nullptr and read by nothing.  It goes from the
// getter's type, from the functions that pass it and from exarg_T (whose
// member, and find_func_t, a typedef nothing names, the collection takes).  What is left of void * in the
// core is the functions of bytes, the allocators and a growarray's storage
// (internal/ccx's VoidPtrs).
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): one PARAM edit drops the
// cookie from the five functions, the three getter parameters and exarg_T's
// getter member -- one family of function types -- and the argument from
// every call, through the pointers too; do_one_cmd()'s store of it is cut
// first.  History keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("cookie", e, w)
	n := v.Mentions("cookie")
	v.Expect(n == 19, "cookie is named %d times; this phase was written against 19", n)
	v.InFunction("do_one_cmd", func(v *graph.Verbs) {
		v.Cut("(= (. ea cookie) cookie)", 1, "do_one_cmd() keeps none")
	})
	getline := v.One("(ea_getline _)", "exarg_T's line getter")
	if v.Failed() {
		return v.Done()
	}
	param := func(fn, name string) graph.ParamDrop {
		var p *graph.Node
		if d := e.Defn(fn); d != nil {
			for _, q := range graph.DeclType(d).Kids[1].Kids {
				if q.IsList() && len(q.Kids) >= 2 && q.Kids[0].Atom == name {
					p = q
				}
			}
		}
		if p == nil {
			v.Die("%s has no parameter %s", fn, name)
		}
		return graph.ParamDrop{Decl: p, I: 1}
	}
	fn := func(name string) graph.ParamDrop {
		ds := e.FileDecls(name)
		if len(ds) == 0 {
			v.Die("%s is not declared", name)
			return graph.ParamDrop{}
		}
		return graph.ParamDrop{Decl: ds[0], I: e.ParamIndex(name, "cookie")}
	}
	drops := []graph.ParamDrop{
		fn("do_cmdline"), fn("getline_equal"), fn("do_one_cmd"), fn("getexline"), fn("getcmdkeycmd"),
		param("do_cmdline", "fgetline"), param("getline_equal", "fgetline"), param("getline_equal", "func"),
		param("do_one_cmd", "fgetline"), {Decl: getline, I: 1},
	}
	if v.Failed() {
		return v.Done()
	}
	st := v.DropParams(drops, graph.ParamOptions{},
		"do_cmdline(), getline_equal(), do_one_cmd(), getexline() and getcmdkeycmd() take no cookie, nor does a line getter's type")
	// 9 cookies and 9 getter types; 6 + 1 + 4 + 1 + 1 calls
	v.Expect(st.Params == 18 && st.Calls == 13 && st.Args == 13, "the cookie: %s, where this phase was written against 18 parameters, 13 calls and 13 arguments", st)
	v.Say("6 calls of do_cmdline() pass none; :append's reader passes none; getline_equal() is asked without one (4); do_cmdline() gets its next line and runs a command without one")
	// exarg_T's own member is left, named by nothing now: the collection takes it.
	n = v.Mentions("cookie")
	v.Expect(n == 1, "cookie is still named %d times, beside exarg_T's member", n-1)
	return v.Done()
}
