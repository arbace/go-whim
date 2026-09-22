package edit

import (
	"io"
	"strings"
)

func init() { register("whim156", Whim156) }

// W156Sentinel is the size pass's sentinel as the input spells it.
const W156Sentinel = "((char_u *) -1)"

// Whim156 gives the regexp compiler's size pass a real node.
//
// bt_regcomp() compiles a pattern twice: once to count the program's bytes,
// with regcode set to JUST_CALC_SIZE, ((char_u *)-1), and once to emit them.
// In the first pass every node the compiler makes is that sentinel, returned
// and compared but never dereferenced, so one code path serves both passes.
// It is an integer made a pointer, which the Go transpilation replaced with a
// one-byte allocation of its own (tx/FINDINGS.md; internal/ccx's Casts).
// Here too: the sentinel is the address of reg_calc_size_node, a static byte
// nothing reads or writes, and all fourteen uses compare with or assign it.
func Whim156(text []byte, w io.Writer) ([]byte, error) {
	p := ph{tag: "calcsize", w: w}
	s := string(text)
	if n := strings.Count(s, W156Sentinel); n != 14 {
		return nil, p.die("the size pass's sentinel is written %d times, and this phase was written against 14", n)
	}
	s = strings.ReplaceAll(s, W156Sentinel, "reg_calc_size_node")
	o, err := p.literal([]byte(s), "static char_u   *regcode;\n", "static char_u   *regcode;\nstatic char_u   reg_calc_size_node[1];\n",
		"the size pass's node is a static byte, compared by address and never read", 1)
	if err != nil {
		return nil, err
	}
	p.say("its fourteen uses compare with or assign the address of that byte, not (char_u *)-1")
	return o, nil
}
