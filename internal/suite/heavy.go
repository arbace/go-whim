package suite

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"
)

// THE HEAVY CASE: the cases are short, and time nothing -- a Clojure editor
// that ran 48 times the C's time on real work answered them all in 0.38 s
// each, flag or no flag (doc/CLOJURE-IDIOMS.md, item 0).  This one session
// is work: 5,000 lines, three substitutions and a :g with :normal, run on
// every editor of the run, one at a time after the others, required to
// answer as the C candidate does and TIMED.  Each editor's time is reported
// beside the C's; an editor more than heavyBound times the C's time fails
// the run.  The bound is a ratio measured within the run, so the machine's
// load moves both sides of it.  Measured when it was set: the Go editor 0.6
// times the C, the Java 2.4 and the Clojure 9.3 -- and, the JIT's huge-method
// limit left on, the Java 3.8 and the Clojure 52, which is what it is there
// to refuse.
var heavyKeys = []byte("ithe quick brown fox jumps over the lazy dog 0123456789\x1byy5000p" +
	":%s/o/0/g\r:%s/\\v(qu)(i)/\\2\\1/g\rgg:g/f0x/normal wwdw\rG:%s/e/E/g\r:q!\r")

const heavyBound = 25

// heavyLimit is how long one editor may take on the heavy case before it is
// killed: long enough that a slow editor is measured and refused by the
// bound, not by the limit.
const heavyLimit = 5 * time.Minute

// checkHeavy runs the heavy case on the reference and the candidate, then
// the Go editor and the JVM editors of b, and reports their times.
func checkHeavy(w io.Writer, rev string, b *builds) error {
	ref, refCode, err := runLimit(b.ref, nil, heavyKeys, heavyLimit)
	if err != nil {
		return fmt.Errorf("the heavy case on %s: %w", rev, err)
	}
	if bytes.Contains(ref, []byte("Error reading input")) {
		return fmt.Errorf("the heavy case ran out of input on %s: its keys never reach :q!", rev)
	}
	type editor struct{ name, bin string }
	eds := []editor{{"C", b.cand}, {"Go", b.goBin}}
	for _, e := range b.jvm {
		eds = append(eds, editor{e.label(""), e.bin})
	}
	var parts, slow []string
	var c time.Duration
	for i, e := range eds {
		start := time.Now()
		out, code, err := runLimit(e.bin, nil, heavyKeys, heavyLimit)
		d := time.Since(start)
		if err != nil {
			return fmt.Errorf("the heavy case on the %s editor: %w", e.name, err)
		}
		if code != refCode || !bytes.Equal(out, ref) {
			fmt.Fprintf(w, "  heavy        the %s editor answers differently from %s\n", e.name, rev)
			return fmt.Errorf("suite: the heavy case moved on the %s editor", e.name)
		}
		if i == 0 {
			c = d
			parts = append(parts, fmt.Sprintf("C %dms", d.Milliseconds()))
			continue
		}
		r := float64(d) / float64(c)
		parts = append(parts, fmt.Sprintf("%s %dms (%.1fx)", e.name, d.Milliseconds(), r))
		if r > heavyBound {
			slow = append(slow, e.name)
		}
	}
	fmt.Fprintf(w, "  heavy        5,000 lines, 3 :s and a :g, answered as %s does: %s\n", rev, strings.Join(parts, ", "))
	if len(slow) > 0 {
		return fmt.Errorf("suite: the heavy case took %s more than %d times the C's time", strings.Join(slow, " and "), heavyBound)
	}
	return nil
}
