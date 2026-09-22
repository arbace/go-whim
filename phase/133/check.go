package p133

// Whim phase 133, the check -- one buffer needs no hash table.
// See phase/133/edit.go, and GOALS.md.
//
// phase/133/check.go proves from the input that curbuf is the one buffer
// or NULL and buflist_findnr() has one caller, requires the table and b_key
// gone, and probes the marks setmark_pos() reaches through buflist_findnr().

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim133", Check) }

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
//     2. THE CUT: buflist_findnr()'s Body is W133FindnrBody exactly, and
//     buf_hashtab, buf_hashtab_add, buf_hashtab_remove and b_key have no
//     mention left -- the sweep took the table, the helpers and the field.
//     3. THE GATE, the libc surface unchanged.
//     4. THE PROBE: marks setmark_pos() reaches through buflist_findnr() --
//     m[, m], m" and their jumps -- write the same bytes on both binaries; the
//     CONTROL, a jump to a mark never set, writes different ones.
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim133", "bufhash")
	if err != nil {
		return err
	}
	r := c.R

	if n := check.Word(c.Old, "buflist_new"); n != 3 {
		r.Bad("buflist_new has %d mentions on the input; its prototype, definition and one call are 3", n)
	}
	assign := func(text, lhs string) []string {
		re := regexp.MustCompile(`(?m)` + lhs + `\s*=\s*([^=;][^;]*);`)
		var rs []string
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			rs = append(rs, strings.TrimSpace(m[1]))
		}
		return rs
	}
	for _, v := range assign(c.Old, `\bcurbuf`) {
		if v != "nullptr" && v != "curwin->w_buffer" && !strings.HasPrefix(v, "buflist_new(") {
			r.Bad("curbuf is assigned %q", v)
		}
	}
	for _, v := range assign(c.Old, `w_buffer`) {
		if v != "nullptr" && v != "curbuf" {
			r.Bad("a window's w_buffer is assigned %q", v)
		}
	}
	calls := regexp.MustCompile(`\bbuflist_findnr\(`).FindAllStringIndex(c.Old, -1)
	if len(calls) != 3 { // prototype, definition, one call
		r.Bad("buflist_findnr has %d mentions on the input; prototype, definition and one call are 3", len(calls))
	}
	if o, cl, found, _ := cutil.Body([]byte(c.Old), "setmark_pos"); !found || !strings.Contains(c.Old[o:cl], "buflist_findnr(fnum)") {
		r.Bad("buflist_findnr()'s one call is not setmark_pos()'s")
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("on the input buflist_new() is called once, curbuf is only ever that buffer, curwin->w_buffer or NULL, w_buffer only curbuf or NULL, and buflist_findnr()'s one caller is setmark_pos(): the table held one buffer")

	o, cl, found, _ := cutil.Body([]byte(c.New), "buflist_findnr")
	if !found || c.New[o:cl+1] != "{\n"+W133FindnrBody+"\n}" {
		r.Bad("buflist_findnr()'s body is not the one this phase writes")
	}
	for _, n := range []string{"buf_hashtab", "buf_hashtab_add", "buf_hashtab_remove", "b_key"} {
		if k := check.Word(c.New, n); k != 0 {
			r.Bad("%s still has %d mentions", n, k)
		}
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("buflist_findnr() compares the current buffer's number; buf_hashtab, its two helpers and buf_T.b_key are gone (the file lost %d lines)",
		check.CountLines([]byte(c.Old))-check.CountLines([]byte(c.New)))

	if err := c.Gate(true); err != nil {
		return err
	}
	old, nw := c.Bins()
	seed := []byte("ione\rtwo\rthree\x1bggm[jm]Gm\"")
	for _, j := range []string{"'[", "']", "`[", "`]", "'\""} {
		keys := [][]byte{seed, []byte("gg" + j), []byte(":q!\r")}
		so, _, e1 := check.Stream(old, keys, nil)
		sn, _, e2 := check.Stream(nw, keys, nil)
		if e1 != nil || e2 != nil {
			r.Say("a probe did not run: %v %v", e1, e2)
			return harness.ErrReported
		}
		if so != sn {
			r.Bad("%s after setting it is written differently on the two binaries", j)
		}
	}
	a, _, _ := check.Stream(nw, [][]byte{seed, []byte("gg'["), []byte(":q!\r")}, nil)
	b, _, _ := check.Stream(nw, [][]byte{[]byte("ione\rtwo\rthree\x1b"), []byte("gg'z"), []byte(":q!\r")}, nil)
	if a == b {
		r.Bad("the CONTROL did not move: a jump to a mark never set writes the same bytes")
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("PROBE: m[ m] m\" and the jumps to them write the same bytes on both binaries, and the CONTROL, a jump to a mark never set, writes different ones")
	return nil
}
