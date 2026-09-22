package check

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { register("whim146", Whim146) }

// Whim146 is phase 146's check: a memline node names its block.
//
//  1. THE CUT: no cast to PTR_BL *, DATA_BL * or bhdr_T * is left but the four
//     allocations; the node is its tag and the two pointers; the blocks begin
//     with no header; the allocators are edit.W146NewData and W146NewPtr.
//  2. EVERY FIELD READ IS OF A NODE THAT IS THERE: a cast of NULL was NULL,
//     and a field read of it would crash.  So each `X->bh_ptr` or
//     `X->bh_data` outside the allocators is a partition member: before it in
//     its function, X was tested against nullptr (alone, or as the value of
//     `(X = ml_...(...)) == nullptr`), or dereferenced for its tag,
//     or taken from the tree's stack, or assigned a node that was -- or X is
//     the root just set to a node checked for NULL.  A read of another kind
//     refuses.
//  3. THE GATE, the libc surface unchanged.  The memline corpus in the
//     recording is the stage's delta check.
//  4. THE PROBE: three hundred lines made, a hundred deleted and the deletion
//     undone -- blocks split and pointer blocks made -- draw the same on both
//     binaries; the CONTROL, one line fewer, moves.
func Whim146(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim146", "memnode")
	if err != nil {
		return err
	}
	r := c.r
	if k := len(regexp.MustCompile(`\((PTR_BL|DATA_BL|bhdr_T) \*\)`).FindAllString(c.new, -1)); k != 4 {
		r.bad("%d casts to a node or a block remain, where the four allocations are 4", k)
	}
	for _, s := range []string{"struct block_hdr\n{\n    short_u     bh_id;\n    struct pointer_block *bh_ptr;\n    struct data_block *bh_data;\n};\n"} {
		if !strings.Contains(c.new, s) {
			r.bad("the node is not its tag and the two pointers")
		}
	}
	for _, n := range []string{"pb_hdr", "db_hdr"} {
		if k := word(c.new, n); k != 0 {
			r.bad("%s has %d mentions left", n, k)
		}
	}
	for _, a := range [][2]string{{"ml_new_data", edit.W146NewData}, {"ml_new_ptr", edit.W146NewPtr}} {
		o, cl, f, _ := cutil.Body([]byte(c.new), a[0])
		if !f || c.new[o:cl+1] != "{\n"+a[1]+"\n}" {
			r.bad("%s()'s body is not the one this phase writes", a[0])
		}
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("no cast to a node or a block is left but the four allocations; a node is its tag and a pointer to its block")

	reads := regexp.MustCompile(`([\w.>-]+?)->bh_(ptr|data)\b`)
	nb := []byte(c.new)
	blank := cutil.Blank(nb)
	n := 0
	for _, fname := range []string{"ml_free_tree", "ml_open", "ml_get_buf", "ml_append_int", "ml_delete_int", "ml_setmarked", "ml_firstmarked", "ml_clearmarked", "ml_flush_line", "ml_find_line", "ml_lineadd"} {
		a, z, ok := cutil.FindDefinition(nb, blank, fname)
		if !ok {
			continue
		}
		fn := c.new[a:z]
		for _, m := range reads.FindAllStringSubmatchIndex(fn, -1) {
			x := fn[m[2]:m[3]]
			if strings.HasSuffix(fn[m[0]:m[1]+10], "= ") || strings.HasPrefix(fn[m[1]:], " = ") {
				continue // a write, in an allocator
			}
			before := fn[:m[0]]
			q := regexp.QuoteMeta(x)
			ok := regexp.MustCompile(`\(\(` + q + ` = ml_\w+\([^;]*\)\) == nullptr\)|\b` + q + `\) == nullptr\)|\b` + q + ` == nullptr\b|\b` + q + `->bh_id\b|\b` + q + ` = ip->ip_block;|\b` + q + ` = hp(_new)?;|\b` + q + ` = hp;`).MatchString(before)
			if x == "buf->b_ml.ml_root" {
				ok = strings.Contains(before, "buf->b_ml.ml_root = hp;") && strings.Contains(before, "if ((hp = ml_new_ptr()) == nullptr)")
			}
			if !ok {
				r.bad("%s(): %s->bh_%s is read with no sign %s is a node", fname, x, fn[m[4]:m[5]], x)
			}
			n++
		}
	}
	if total := len(reads.FindAllString(c.new, -1)) - 2; n != total {
		r.bad("%d reads classified of %d: a read outside the memline functions named here", n, total)
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("each of the %d reads of a node's block is of a node tested for NULL, dereferenced for its tag, taken from the tree's stack or just made", n)
	if err := c.gate(true); err != nil {
		return err
	}
	ob, nbin := c.bins()
	keys := [][]byte{[]byte("300ox\x1bgg100ddu:200\r"), []byte(":q!\r")}
	ctl := [][]byte{[]byte("299ox\x1bgg100ddu:200\r"), []byte(":q!\r")}
	s1, _, e1 := stream(ob, keys, nil)
	s2, _, e2 := stream(nbin, keys, nil)
	s3, _, e3 := stream(nbin, ctl, nil)
	if e1 != nil || e2 != nil || e3 != nil {
		r.say("a probe did not run: %v %v %v", e1, e2, e3)
		return harness.ErrReported
	}
	if s1 != s2 {
		r.bad("making, deleting and restoring lines draws differently on the two binaries")
	}
	if s3 == s2 {
		r.bad("the CONTROL, one line fewer, did not move")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("PROBE: 300 lines made, 100 deleted and restored, draw the same on both binaries; one line fewer moves")
	return nil
}
