package check

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { register("whim137", Whim137) }

var w137Use = regexp.MustCompile(`\(\((\w+)\)->b_ct_di\.di_tv\.vval\)`)

// Whim137 is phase 137's check: the changedtick is a number.
//
//  1. THE PARTITION, on the input: every mention of b_ct_di is the field, one
//     of the reads and writes of its number `((X)->b_ct_di.di_tv.vval)`, or
//     init_changedtick()'s cast -- so nothing read the typval's type, its lock
//     or the item's flags, which init_changedtick() alone wrote.
//  2. THE CUT: b_ct_di has no mention; b_changedtick is the field, the body of
//     init_changedtick() (edit.W137InitBody) and exactly as many reads and
//     writes as the input had, each where the input had one (edit.W137Tick,
//     applied to the input's lines, gives the output's).
//  3. THE GATE, the libc surface unchanged.  The recording -- every insert,
//     undo and search it drives reads the tick -- is the stage's delta check.
func Whim137(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim137", "tick")
	if err != nil {
		return err
	}
	r := c.r
	uses := len(w137Use.FindAllString(c.old, -1))
	other := 0
	for _, l := range linesWith(c.old, "b_ct_di") {
		switch {
		case l == "dictitem16_T b_ct_di;":
		case l == "dictitem_T *di = (dictitem_T *)&buf->b_ct_di;":
		case w137Use.MatchString(l):
		default:
			other++
			r.bad("b_ct_di is used otherwise: %s", l)
		}
	}
	if word(c.old, "b_ct_di") != uses+2 {
		r.bad("b_ct_di has %d mentions on the input, where the field, the cast and %d uses of the number are %d", word(c.old, "b_ct_di"), uses, uses+2)
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("on the input b_ct_di is the field, init_changedtick()'s cast and %d reads and writes of the number: nothing read its type, lock or flags", uses)

	if k := word(c.new, "b_ct_di"); k != 0 {
		r.bad("b_ct_di has %d mentions left", k)
	}
	o, cl, found, _ := cutil.Body([]byte(c.new), "init_changedtick")
	if !found || c.new[o:cl+1] != "{\n"+edit.W137InitBody+"\n}" {
		r.bad("init_changedtick()'s body is not the one this phase writes")
	}
	if k := word(c.new, "b_changedtick"); k != uses+2 {
		r.bad("b_changedtick has %d mentions, where the field, the initialiser and %d uses are %d", k, uses, uses+2)
	}
	// each use is where the input had one: the input's lines holding a use,
	// rewritten by the rule, are the output's lines holding one, in order
	var want []string
	for _, l := range strings.Split(c.old, "\n") {
		if w137Use.MatchString(l) {
			s, _ := edit.W137Tick(l)
			want = append(want, s)
		}
	}
	var got []string
	for _, l := range strings.Split(c.new, "\n") {
		if strings.Contains(l, "->b_changedtick") && !strings.Contains(l, "buf->b_changedtick = 0;") {
			got = append(got, l)
		}
	}
	if strings.Join(want, "\n") != strings.Join(got, "\n") {
		r.bad("the uses of the tick are not the input's, rewritten by the rule")
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("the field is a varnumber_T, init_changedtick() sets it to 0, and its %d reads and writes are the input's, in place; b_ct_di is gone", uses)
	return c.gate(true)
}
