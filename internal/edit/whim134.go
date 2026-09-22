package edit

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
)

func init() { register("whim134", Whim134) }

var (
	w134Empty = regexp.MustCompile(`(?m)^([ \t]*)(if \(.*\)|else if \(.*\)|else)\n([ \t]*)\{\n[ \t]*\}\n`)
	w134Fn    = regexp.MustCompile(`[A-Za-z_]\w*\s*\(`)
	w134Write = regexp.MustCompile(`(^|[^=!<>])=($|[^=])|\+\+|--`)
	w134Else  = regexp.MustCompile(`^[ \t]*else\b`)
)

// W134Pure: a condition that only reads -- no call, no assignment, no ++ or --.
// Casts and sizeof read nothing a call could change; they are allowed.
func W134Pure(cond string) bool {
	c := regexp.MustCompile(`\((?:const\s+)?(?:unsigned\s+)?[A-Za-z_]\w*\s*\**\s*\)`).ReplaceAllString(cond, "")
	c = regexp.MustCompile(`\bsizeof\s*\(`).ReplaceAllString(c, "(")
	return !w134Fn.MatchString(c) && !w134Write.MatchString(c)
}

// W134Fold is the rule, applied once over the whole core, exported so the check
// applies the identical one.  It returns the text and how many blocks went.
//
//	if (C) {}      no else after, C pure        -> nothing
//	else {}                                     -> nothing
//	else if (C) {} last of its chain, C pure    -> nothing
//
// Nothing else: an empty block whose condition does something, an empty if
// with an else after it, and every empty loop stay.
func W134Fold(core []byte) ([]byte, int) {
	n := 0
	for {
		changed := false
		for _, m := range w134Empty.FindAllSubmatchIndex(core, -1) {
			head := string(core[m[4]:m[5]])
			after := core[m[1]:]
			var cond string
			switch {
			case head == "else":
			case bytes.HasPrefix([]byte(head), []byte("if (")):
				cond = head[4 : len(head)-1]
			default: // else if
				cond = head[9 : len(head)-1]
			}
			if head != "else" && (!W134Pure(cond) || w134Else.Match(after)) {
				continue
			}
			core = append(append([]byte{}, core[:m[0]]...), core[m[1]:]...)
			n++
			changed = true
			break
		}
		if !changed {
			return core, n
		}
	}
}

// W134Rule is the phase: W134Fold and DeadStores, each to its fixpoint, in turn
// until neither changes anything -- a flag tested only by an empty if is only
// stored once the if goes, and a store that goes can leave a block empty.
func W134Rule(core []byte) ([]byte, int, []string) {
	n := 0
	var took []string
	for {
		var k int
		var t []string
		core, k = W134Fold(core)
		core, t = DeadStores(core)
		n += k
		took = append(took, t...)
		if k == 0 && len(t) == 0 {
			return core, n, took
		}
	}
}

// Whim134 folds the blocks phase 132 left empty.
//
// Taking out 273 frees left 33 blocks that had held nothing else (`if
// (allocated) { vim_free(p); }` is `if (allocated) { }`), beside the empty
// blocks the pipeline had already left: 49 in all.  An empty block
// guarded by a condition that only reads does nothing, and goes; so does an
// empty else, and an empty else-if that ends its chain.  What the condition
// computed is then read by nothing: a local only ever given a value goes with
// its stores (DeadStores), and the sweep takes the rest.
// The Go transpilation never had these blocks (tx/FINDINGS.md, 9).
func Whim134(text []byte, w io.Writer) ([]byte, error) {
	p := ph{tag: "empty", w: w}
	i := bytes.Index(text, []byte("\n#include"))
	if i < 0 {
		return nil, p.die("no #include: the boundary is not where this phase expects it")
	}
	core, n, took := W134Rule(text[:i+1])
	if n < 20 {
		return nil, p.die("%d empty blocks fold; this phase was written against the 30 or so phase 132 leaves", n)
	}
	p.say(fmt.Sprintf("%d empty blocks fold away: an if whose condition only reads, an empty else, an empty else-if that ends its chain", n))
	p.say(fmt.Sprintf("%d locals only given values once their tests went, and go with their stores: %s", len(took), strings.Join(took, " ")))
	return append(core, text[i+1:]...), nil
}
