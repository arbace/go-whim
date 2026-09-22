package check

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/edit"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { register("whim159", Whim159) }

// Whim159 is phase 159's check: no struct's last member is an array sized at
// allocation.
//
//  1. THE PREMISE, a partition of every mention of the three types, on the
//     input: its typedef, a pointer to it, the one offsetof() that sized its
//     allocation, and buffheader_T's head block -- no copy, no sizeof, no
//     other instance, so nothing depended on the array being inside the
//     struct.  And a list's head is current only with bh_create_newblock set,
//     which only a new block, made current, clears: nothing but
//     delete_buff_tail()'s NUL is ever written into a head's byte.
//  2. THE CUT, the same partition on the output with sizeof(T) where the
//     offsetof() was, and no one-element array left as any struct's last
//     member.
//  3. THE GATE, the libc surface unchanged.
//  4. THE PROBES: a repeated change (the redo buffer), a recorded and replayed
//     register with an empty recording (the record buffer, and a head's
//     byte), a long message redisplayed by g< (the message chunks) and a
//     pattern (a program) draw the same on both binaries; each CONTROL moves.
func Whim159(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim159", "structhack")
	if err != nil {
		return err
	}
	r := c.r
	for _, side := range []struct{ name, text, sized string }{{"input", c.old, "offsetof"}, {"output", c.new, "sizeof"}} {
		core := side.text[:strings.Index(side.text, "\n#include")+1]
		for _, t := range edit.W159Types {
			classes := []*regexp.Regexp{
				regexp.MustCompile(`^typedef struct \w+ ` + t + `;$|^} ` + t + `;$`),
				regexp.MustCompile(`\b` + t + `\s*\*`),
			}
			sized := regexp.MustCompile(`alloc\(__builtin_offsetof\(` + t + `, \w+\) \+ `)
			if side.sized == "sizeof" {
				sized = regexp.MustCompile(`= alloc\(sizeof\(` + t + `\)\);$`)
			}
			nsized := 0
			for _, l := range linesWith(core, t) {
				switch {
				case sized.MatchString(l):
					nsized++
				case l == "buffblock_T bh_first;" && t == "buffblock_T":
				case classes[0].MatchString(l), classes[1].MatchString(l):
				default:
					r.bad("the %s mentions %s other than as its typedef, a pointer or its allocation: %s", side.name, t, l)
				}
			}
			if nsized != 1 {
				r.bad("the %s allocates a %s by %s %d times", side.name, t, side.sized, nsized)
			}
		}
	}
	if regexp.MustCompile(`(?m)^    \w+ +\w+\[1\];\n}`).MatchString(c.new[:strings.Index(c.new, "\n#include")+1]) {
		r.bad("a struct still ends in a one-element array")
	}
	// a head is made current only with bh_create_newblock set, and only a new
	// block, made current, clears it
	made := regexp.MustCompile(`(?m)^ *(\S+)bh_curr = &\((\S+)bh_first\);\n *(\S+)bh_create_newblock = TRUE;$`).FindAllStringSubmatch(c.old, -1)
	for _, m := range made {
		if m[1] != m[2] || m[1] != m[3] {
			r.bad("a head is made current in one list and marked in another: %s", m[0])
		}
	}
	if n := strings.Count(c.old, "bh_curr = &("); n != len(made) || n == 0 {
		r.bad("%d heads are made current, %d of them with bh_create_newblock set", n, len(made))
	}
	cleared := "        buf->bh_create_newblock = FALSE;\n\n        p->b_next = buf->bh_curr->b_next;\n        buf->bh_curr->b_next = p;\n        buf->bh_curr = p;\n"
	if n := strings.Count(c.old, "bh_create_newblock = FALSE"); n != 1 || strings.Count(c.old, cleared) != 1 {
		r.bad("bh_create_newblock is cleared %d times, not only as a new block becomes current", n)
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("buffblock_T, msgchunk_T and regprog_T are named only as typedefs, pointers and the one allocation each, and buffheader_T's head; a head's byte is written only with NUL")
	r.say("each trailing array is a char_u * to an allocation of its own, and each list head's to a byte of its own")
	if err := c.gate(true); err != nil {
		return err
	}
	ob, nb := c.bins()
	seed := []byte("ione two three\x1b0")
	probes := []struct {
		what      string
		keys, ctl [][]byte
	}{
		{"a repeated change", [][]byte{[]byte("Afoo\x1b..")}, [][]byte{[]byte("Afoo\x1b.")}},
		{"a recorded register and an empty one", [][]byte{[]byte("qbqqaAx\x1bq@a@b")}, [][]byte{[]byte("qbqqaAy\x1bq@a@b")}},
		{"a long message redisplayed", [][]byte{[]byte(":version\r"), []byte("q"), []byte("g<"), []byte("q")}, [][]byte{[]byte(":version\r"), []byte("q")}},
		{"a pattern", [][]byte{[]byte("/t\\(wo\\|hree\\)\\@=\r")}, [][]byte{[]byte("/t\\(wo\\)\\@=\r")}},
	}
	for _, pr := range probes {
		keys := append(append([][]byte{seed}, pr.keys...), []byte(":q!\r"))
		ctl := append(append([][]byte{seed}, pr.ctl...), []byte(":q!\r"))
		s1, _, e1 := stream(ob, keys, nil)
		s2, _, e2 := stream(nb, keys, nil)
		s3, _, e3 := stream(nb, ctl, nil)
		if e1 != nil || e2 != nil || e3 != nil {
			r.say("a probe did not run: %v %v %v", e1, e2, e3)
			return harness.ErrReported
		}
		if s1 != s2 {
			r.bad("%s is drawn differently on the two binaries", pr.what)
		}
		if s3 == s2 {
			r.bad("the CONTROL for %s did not move", pr.what)
		}
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("PROBE: a repeated change, a recorded and an empty register, a long message redisplayed by g< and a pattern draw the same on both binaries; each CONTROL moves")
	return nil
}
