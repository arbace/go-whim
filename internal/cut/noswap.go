package cut

import (
	"bytes"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// noswapStubs: the flag matters.  check_need_swap() and changed() call
// ml_open_file() on every edit, and a body that simply returned would search
// 'directory' again on the next keystroke if the flag were left set.
var noswapStubs = []struct{ name, body string }{
	{"ml_open_file", "buf->b_may_swap = FALSE;"},
	{"ml_preserve", ""},
	{"ml_sync_all", ""},
	{"ml_setname", ""},
}

// NoSwap takes the swap file away.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): the four bodies FRAG (BodyC),
// their old lengths the text's on the C view; the SEA_RECOVER arm an else
// cut, found once in the file; mf_sync's tail a DropIf by its condition
// (history keeps the text version).
func NoSwap(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("noswap", e, w)
	total := 0
	for _, s := range noswapStubs {
		if e.Defn(s.name) == nil {
			v.Die("%s is not defined at file scope any more", s.name)
			return v.Done()
		}
		was := 0
		v.InFunction(s.name, func(v *graph.Verbs) { was = bodyLines(v.Text()) })
		q := graph.NewVerbs("noswap", e, io.Discard)
		q.BodyC(s.name, s.body, s.name)
		if err := q.Done(); err != nil {
			return err
		}
		total += was
		shape := "empty"
		if s.body != "" {
			shape = "one"
		}
		v.Sayf("%-14s was %3d lines, is now %s", s.name, was, shape)
	}

	// Counted over the WHOLE file and not capped at one: two arms would
	// refuse.  It is an else-if arm, and it is the one that recovers.
	const arm = "(if (== swap_exists_action SEA_RECOVER) _)"
	arms := v.Find(arm)
	ok := len(arms) == 1 && e.Parent(arms[0]).Is("if")
	if ok {
		v.In(arms[0], func(v *graph.Verbs) { ok = v.Count("(call ml_recover FALSE)") == 1 })
	}
	if !ok {
		v.Die("expected one SEA_RECOVER arm, matched %d -- it is "+
			"the only way into ml_recover once :recover is retired, and leaving it keeps "+
			"576 lines alive", len(arms))
		return v.Done()
	}
	v.Cut(arm, 1, "the SEA_RECOVER arm of the ATTENTION prompt")

	// A ROW IS WHAT INITIALISES ITS GLOBAL, so a row can only go once nothing
	// reads the global -- otherwise a `char_u *` stays NULL for ever and the
	// first dereference is a segfault.  'swapsync' is p_sws, read in
	// mf_sync()'s MFS_FLUSH tail, removed here; it is also the only caller of
	// sync().  ('directory' is p_dir, read by recover_names() until record
	// 21, so it could not go here; 'updatecount' is p_uc, a long, whose
	// orphan reads 0 -- "never create a swap file".)
	v.DropIf("(&& (paren (& flags MFS_FLUSH)) (!= (deref p_sws) NUL))", 1,
		"mf_sync's fsync/sync tail, the last reader of p_sws")

	if v.Failed() {
		return v.Done()
	}
	v.Sayf("%d lines stubbed; %d findswapname mentions left for the sweep",
		total, bytes.Count(v.Text(), []byte("findswapname")))
	return v.Done()
}
