package check

import (
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/harness"
)

func init() { register("whim129", Whim129) }

var (
	w129Row  = regexp.MustCompile(`\{"([a-z]+)",\s*(?:"[a-z]*"|nullptr),\s*([A-Z_|]+),\s*\(char_u \*\)&(p_[a-z_]+)`)
	w129Decl = regexp.MustCompile(`(?m)^static ([a-z_]+(?: [a-z_]+)?)\s+(\**)(p_[a-z_]+)(?:\[[^]]*\])?;`)
)

// w129Bool is every P_BOOL row of options[] whose variable is a global, with
// the global's declared type: the partition this phase is about.
func w129Bool(text string) (ok map[string]string, bad map[string]string) {
	decl := map[string]string{}
	for _, m := range w129Decl.FindAllStringSubmatch(text, -1) {
		decl[m[3]] = m[1] + m[2]
	}
	ok, bad = map[string]string{}, map[string]string{}
	for _, m := range w129Row.FindAllStringSubmatch(text, -1) {
		if !strings.Contains(m[2], "P_BOOL") {
			continue
		}
		t := decl[m[3]]
		if t == "int" {
			ok[m[1]] = m[3]
		} else {
			bad[m[1]] = m[3] + " is " + t
		}
	}
	return ok, bad
}

// Whim129 is phase 129's check: p_emoji is an int.
//
//  1. THE PARTITION, on both sides: every P_BOOL row of options[] names a
//     variable declared `int`.  On the input exactly one does not, 'emoji',
//     and on the output none does -- so the phase fixed the one case there was,
//     and the check fails if another appears or this one comes back.
//  2. THE MENTIONS: p_emoji is its declaration, the options row and the one
//     reader in utf_char2cells(), before and after.
//  3. THE GATE: a silent compile, the libc surface unchanged as a set.
//  4. THE PROBE the recording cannot make, since no case types an emoji: a line
//     holding one is written in the same bytes by the binary the phase was
//     handed and by the one it made; and the CONTROL, `+set noemoji`, writes
//     different bytes -- so the probe sees the option at all.
func Whim129(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim129", "emoji")
	if err != nil {
		return err
	}
	r := c.r

	okIn, badIn := w129Bool(c.old)
	okOut, badOut := w129Bool(c.new)
	if len(badIn) != 1 || badIn["emoji"] == "" {
		r.bad("the input's boolean options that are not int are %v -- this phase was written against exactly 'emoji'", badIn)
	}
	if len(badOut) != 0 {
		r.bad("boolean options whose variable is not int remain: %v", badOut)
	}
	if len(okOut) != len(okIn)+1 || len(okOut) < 20 {
		r.bad("the int boolean rows went %d -> %d, expected one more (and a table this size has many)", len(okIn), len(okOut))
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("every one of the %d P_BOOL rows of options[] with a global variable now names an int; the input had %d and one exception, %s",
		len(okOut), len(okIn), badIn["emoji"])

	want := []string{
		"static int      p_emoji;",
		"if (p_emoji && intable(emoji_wide, sizeof(emoji_wide), c))",
		"(char_u *)&p_emoji, PV_NONE, did_set_ambiwidth, nullptr,",
	}
	got := linesWith(c.new, "p_emoji")
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		r.bad("p_emoji's mentions are not its declaration, its row and its reader:\n    %s", strings.Join(got, "\n    "))
	}
	if n := countLines([]byte(c.old)) - countLines([]byte(c.new)); n != 0 {
		r.bad("the file changed length by %d lines; this phase rewrites one", n)
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("p_emoji: the declaration, the options row and the reader in utf_char2cells(), and nothing else; no line added or removed")

	if err := c.gate(true); err != nil {
		return err
	}

	old, nw := c.bins()
	keys := [][]byte{[]byte("iab\xf0\x9f\x98\x80cd\x1b"), []byte(":q!\r")}
	so, _, e1 := stream(old, keys, nil)
	sn, _, e2 := stream(nw, keys, nil)
	sc, _, e3 := stream(nw, keys, []string{"+set noemoji"})
	if e1 != nil || e2 != nil || e3 != nil {
		r.say("a probe did not run: %v %v %v", e1, e2, e3)
		return harness.ErrReported
	}
	if so != sn {
		r.bad("the emoji line is written differently: %s on the input's binary, %s on this one", so, sn)
	}
	if sc == sn {
		r.bad("the CONTROL did not move: `+set noemoji` writes the same bytes (%s), so the probe cannot see the option", sc)
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("PROBE: `ab<U+1F600>cd` is written in the same bytes by both binaries (%s), and `+set noemoji` writes different ones (%s): the option still works, read through the int", sn, sc)
	return nil
}
