package check

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { register("whim133", Whim133) }

// Whim133 is phase 133's check: one buffer needs no hash table.
//
// 1. WHY IT IS SAFE, on the input, as partitions of assignments:
//   - buflist_new() is its prototype, its definition and ONE call;
//   - every `curbuf = ...` assigns nullptr, curwin->w_buffer or buflist_new()'s
//     result, and every `w_buffer = ...` assigns nullptr or curbuf -- so
//     curbuf is the one buffer or NULL;
//   - buflist_findnr() has one caller, setmark_pos(), which asks for the
//     number of the buffer a mark is set in.
//     The table held the one buffer from buflist_new() to free_buffer(), which
//     also sets curbuf NULL: "the current buffer if its number is nr" answers
//     what the table answered whenever a mark can be set.
//     2. THE CUT: buflist_findnr()'s body is edit.W133FindnrBody exactly, and
//     buf_hashtab, buf_hashtab_add, buf_hashtab_remove and b_key have no
//     mention left -- the sweep took the table, the helpers and the field.
//     3. THE GATE, the libc surface unchanged.
//     4. THE PROBE: marks setmark_pos() reaches through buflist_findnr() --
//     m[, m], m" and their jumps -- write the same bytes on both binaries; the
//     CONTROL, a jump to a mark never set, writes different ones.
func Whim133(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim133", "bufhash")
	if err != nil {
		return err
	}
	r := c.r

	if n := word(c.old, "buflist_new"); n != 3 {
		r.bad("buflist_new has %d mentions on the input; its prototype, definition and one call are 3", n)
	}
	assign := func(text, lhs string) []string {
		re := regexp.MustCompile(`(?m)` + lhs + `\s*=\s*([^=;][^;]*);`)
		var rs []string
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			rs = append(rs, strings.TrimSpace(m[1]))
		}
		return rs
	}
	for _, v := range assign(c.old, `\bcurbuf`) {
		if v != "nullptr" && v != "curwin->w_buffer" && !strings.HasPrefix(v, "buflist_new(") {
			r.bad("curbuf is assigned %q", v)
		}
	}
	for _, v := range assign(c.old, `w_buffer`) {
		if v != "nullptr" && v != "curbuf" {
			r.bad("a window's w_buffer is assigned %q", v)
		}
	}
	calls := regexp.MustCompile(`\bbuflist_findnr\(`).FindAllStringIndex(c.old, -1)
	if len(calls) != 3 { // prototype, definition, one call
		r.bad("buflist_findnr has %d mentions on the input; prototype, definition and one call are 3", len(calls))
	}
	if o, cl, found, _ := cutil.Body([]byte(c.old), "setmark_pos"); !found || !strings.Contains(c.old[o:cl], "buflist_findnr(fnum)") {
		r.bad("buflist_findnr()'s one call is not setmark_pos()'s")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("on the input buflist_new() is called once, curbuf is only ever that buffer, curwin->w_buffer or NULL, w_buffer only curbuf or NULL, and buflist_findnr()'s one caller is setmark_pos(): the table held one buffer")

	o, cl, found, _ := cutil.Body([]byte(c.new), "buflist_findnr")
	if !found || c.new[o:cl+1] != "{\n"+edit.W133FindnrBody+"\n}" {
		r.bad("buflist_findnr()'s body is not the one this phase writes")
	}
	for _, n := range []string{"buf_hashtab", "buf_hashtab_add", "buf_hashtab_remove", "b_key"} {
		if k := word(c.new, n); k != 0 {
			r.bad("%s still has %d mentions", n, k)
		}
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("buflist_findnr() compares the current buffer's number; buf_hashtab, its two helpers and buf_T.b_key are gone (the file lost %d lines)",
		countLines([]byte(c.old))-countLines([]byte(c.new)))

	if err := c.gate(true); err != nil {
		return err
	}
	old, nw := c.bins()
	seed := []byte("ione\rtwo\rthree\x1bggm[jm]Gm\"")
	for _, j := range []string{"'[", "']", "`[", "`]", "'\""} {
		keys := [][]byte{seed, []byte("gg" + j), []byte(":q!\r")}
		so, _, e1 := stream(old, keys, nil)
		sn, _, e2 := stream(nw, keys, nil)
		if e1 != nil || e2 != nil {
			r.say("a probe did not run: %v %v", e1, e2)
			return harness.ErrReported
		}
		if so != sn {
			r.bad("%s after setting it is written differently on the two binaries", j)
		}
	}
	a, _, _ := stream(nw, [][]byte{seed, []byte("gg'["), []byte(":q!\r")}, nil)
	b, _, _ := stream(nw, [][]byte{[]byte("ione\rtwo\rthree\x1b"), []byte("gg'z"), []byte(":q!\r")}, nil)
	if a == b {
		r.bad("the CONTROL did not move: a jump to a mark never set writes the same bytes")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("PROBE: m[ m] m\" and the jumps to them write the same bytes on both binaries, and the CONTROL, a jump to a mark never set, writes different ones")
	return nil
}
