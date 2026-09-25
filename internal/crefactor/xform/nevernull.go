package xform

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	ctext "github.com/arbace/go-whim/internal/crefactor/text"
)

// NeverNullKnobs is what NeverNull is told.
type NeverNullKnobs struct {
	// Core is where the core ends; the tests fold in the core only.
	Core Core
	// Roots are the functions that never return NULL by the code base's own
	// argument: an allocator that ends the process rather than fail.
	Roots []string
}

// NeverNull is the step that folds every NULL test of a never-NULL
// function's result directly after the call: `== nullptr` is never true and
// `!= nullptr` always is.  The never-NULL functions are found to a fixpoint
// from k.Roots: a function returning a pointer whose every return is a
// never-NULL call, or a local assigned only never-NULL calls.  Folding one
// test can make another function never-NULL, so the set is recomputed until
// nothing changes; a label only the folded branches jumped to goes.
//
// Its one argument is a floor: `--at-least N` refuses when fewer than N
// tests fold.  Without it there is none.
func NeverNull(k NeverNullKnobs) Step {
	return func(text []byte, args []string, w io.Writer) ([]byte, error) {
		p := ctext.Ph{Tag: "allocnull", W: w}
		f, err := flags(p.Tag, args, "--at-least")
		if err != nil {
			return nil, err
		}
		i := k.Core(text)
		if i < 0 {
			return nil, p.Die("the core does not end where this step was told it does")
		}
		core, nn, n, left := NeverNullRule(text[:i+1], k.Roots)
		if n < f["--at-least"] {
			return nil, p.Die("%d NULL tests fold, fewer than the %d this step was told to expect", n, f["--at-least"])
		}
		p.Say(fmt.Sprintf("%d NULL tests of a never-NULL function's result fold; %d functions never return NULL", n, len(nn)))
		for _, l := range left {
			p.Say("left: " + l)
		}
		return append(core, text[i+1:]...), nil
	}
}

var (
	nnDef    = regexp.MustCompile(`(?m)^([A-Za-z_]\w*)\(`)
	nnReturn = regexp.MustCompile(`\breturn\b\s*([^;]*);`)
	nnCast   = `(?:\(\s*(?:const\s+)?(?:unsigned\s+)?(?:struct\s+)?\w+\s*\*+\s*\)\s*)?`
)

// nnFuncs is every function defined in text: name -> [start, end), found in
// one pass -- a name at the start of a line, its parameter list, then a brace
// -- where the start is the line before, which holds the return type.
func nnFuncs(text []byte) map[string][2]int {
	b := ctext.Blank(text)
	fs := map[string][2]int{}
	for _, m := range nnDef.FindAllSubmatchIndex(text, -1) {
		name := string(text[m[2]:m[3]])
		if _, seen := fs[name]; seen {
			continue
		}
		rp := ctext.Match(b, m[1]-1)
		if rp < 0 {
			continue
		}
		k := rp + 1
		for k < len(b) && (b[k] == ' ' || b[k] == '\n') {
			k++
		}
		if k >= len(b) || b[k] != '{' {
			continue
		}
		e := ctext.Match(b, k)
		if e < 0 {
			continue
		}
		st := bytes.LastIndexByte(text[:m[0]-1], '\n') + 1
		fs[name] = [2]int{st, e + 1}
	}
	return fs
}

// nnIsNN: an expression is a call of a never-NULL function, under a cast.
func nnIsNN(e string, nn map[string]bool) bool {
	m := regexp.MustCompile(`^` + nnCast + `([A-Za-z_]\w*)\(`).FindStringSubmatch(strings.TrimSpace(e))
	if m == nil || !nn[m[1]] {
		return false
	}
	// the call is the whole expression
	e = strings.TrimSpace(e)
	return strings.HasSuffix(e, ")") && ctext.Match(ctext.Blank([]byte(e)), strings.Index(e, m[1]+"(")+len(m[1])) == len(e)-1
}

// nnLocalNN: v is a local of fn whose every assignment is a never-NULL call,
// that is never incremented, and whose address is never taken.
func nnLocalNN(fn, v string, nn map[string]bool) bool {
	q := regexp.QuoteMeta(v)
	if !regexp.MustCompile(`(?m)^\s+[\w ]+?\*+\s*` + q + `;`).MatchString(fn) {
		return false // not a pointer local declared without an initialiser
	}
	if regexp.MustCompile(`&\s*` + q + `\b|\b` + q + `\s*(\+\+|--|\+=|-=)|(\+\+|--)\s*` + q + `\b`).MatchString(fn) {
		return false
	}
	asg := regexp.MustCompile(`\b`+q+`\s*=([^=][^;]*);`).FindAllStringSubmatch(fn, -1)
	if len(asg) == 0 {
		return false
	}
	for _, a := range asg {
		rhs := strings.TrimSpace(a[1])
		// `(v = f(...)) == nullptr` inside a condition: the rhs runs to the `)` of the assignment
		if i := strings.Index(rhs, ") =="); i >= 0 {
			rhs = rhs[:i]
		}
		if !nnIsNN(rhs, nn) {
			return false
		}
	}
	return true
}

// NeverNullSet is the set of functions that never return NULL, to its fixpoint on
// text: the roots, and every function returning a pointer whose every
// return is a never-NULL call or a local assigned only never-NULL calls.
func NeverNullSet(text []byte, roots []string) map[string]bool {
	nn := map[string]bool{}
	for _, b := range roots {
		nn[b] = true
	}
	fs := nnFuncs(text)
	for {
		changed := false
		for name, r := range fs {
			if nn[name] {
				continue
			}
			fn := string(text[r[0]:r[1]])
			head := fn[:strings.Index(fn, name+"(")]
			if !strings.Contains(head, "*") {
				continue
			}
			rets := nnReturn.FindAllStringSubmatch(fn, -1)
			ok := len(rets) > 0
			for _, rt := range rets {
				e := strings.TrimSpace(rt[1])
				if nnIsNN(e, nn) {
					continue
				}
				if regexp.MustCompile(`^` + nnCast + `([A-Za-z_]\w*)$`).MatchString(e) {
					v := regexp.MustCompile(`([A-Za-z_]\w*)$`).FindString(e)
					if nnLocalNN(fn, v, nn) {
						continue
					}
				}
				ok = false
				break
			}
			if ok {
				nn[name] = true
				changed = true
			}
		}
		if !changed {
			return nn
		}
	}
}

// NeverNullRule folds every NULL test of a never-NULL allocation's result that
// directly follows it, recomputing the never-NULL set between rounds, to the
// fixpoint.  It returns the text, the final set, how many tests folded, and
// the sites it had to leave (an always-true test with an else).
func NeverNullRule(core []byte, roots []string) ([]byte, map[string]bool, int, []string) {
	folded := 0
	var left []string
	seenLeft := map[string]bool{}
	for {
		nn := NeverNullSet(core, roots)
		names := make([]string, 0, len(nn))
		for n := range nn {
			names = append(names, n)
		}
		sort.Strings(names)
		alt := strings.Join(names, "|")
		// `v = f(...) ;` -- or `T *v = f(...);`, a declaration -- then `if (v OP
		// nullptr)` at the same indentation: on the next line, or after one line
		// that only stores v somewhere (`n->next = q;`), which cannot
		// change it.  (The declaration and the store were the two shapes this
		// rule once missed: two tests a Go translation printed
		// as nil checks of a new() that staticcheck calls never true.)
		r1 := regexp.MustCompile(`(?m)^( *)(?:[A-Za-z_][\w ]*?\*+ *)?([\w.>\[\]-]+) = +` + nnCast + `(?:` + alt + `)\([^;\n]*\) ?;\n(?:( *)[\w.>\[\]-]+ = ([\w.>\[\]-]+);\n)?( *)if \(([\w.>\[\]-]+) (==|!=) nullptr\)\n`)
		// `if ((v = f(...)) == nullptr)`
		r2 := regexp.MustCompile(`(?m)^( *)if \(\(([\w.>\[\]-]+) = (` + nnCast + `(?:` + alt + `)\([^;\n]*\))\) == nullptr\)\n`)
		changed := false
		for {
			m := r2.FindSubmatchIndex(core)
			hit := false
			for off := 0; m != nil; {
				m[0], m[1] = m[0]+off, m[1]+off
				for k := 2; k < len(m); k++ {
					m[k] += off
				}
				ind, v, call := string(core[m[2]:m[3]]), string(core[m[4]:m[5]]), string(core[m[6]:m[7]])
				if nnIsNN(call, nn) {
					repl := ind + v + " = " + call + ";\n" + ind + "if (" + v + " == nullptr)\n"
					core = append(append(append([]byte{}, core[:m[0]]...), repl...), core[m[1]:]...)
					hit = true
					break
				}
				off = m[1]
				m = r2.FindSubmatchIndex(core[off:])
			}
			if !hit {
				break
			}
			changed = true
		}
		for from := 0; ; {
			m := r1.FindSubmatchIndex(core[from:])
			if m == nil {
				break
			}
			for k := range m {
				if m[k] >= 0 { // an optional group that did not match stays -1
					m[k] += from
				}
			}
			from = m[1]
			// groups: 1 indent, 2 v, 3-4 the optional store (indent, what it
			// stores), 5 the if's indent, 6 its variable, 7 its operator
			if string(core[m[2]:m[3]]) != string(core[m[10]:m[11]]) || string(core[m[4]:m[5]]) != string(core[m[12]:m[13]]) {
				continue
			}
			if m[6] >= 0 && (string(core[m[6]:m[7]]) != string(core[m[2]:m[3]]) || string(core[m[8]:m[9]]) != string(core[m[4]:m[5]])) {
				continue // the line between is not a store of v at the same depth
			}
			line := core[m[0]:m[1]]
			nl := bytes.IndexByte(line, '\n')
			call := string(line[bytes.Index(line, []byte(" = "))+3 : nl])
			call = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(call), ";"))
			if !nnIsNN(call, nn) {
				continue
			}
			ifAt := m[10]
			op := string(core[m[14]:m[15]])
			marked := append(append(append([]byte{}, core[:ifAt]...), []byte(string(core[m[10]:m[11]])+"if (__nevernull__)\n")...), core[m[1]:]...)
			var Out []byte
			var err error
			if op == "==" {
				Out, err = ctext.FoldNever(marked, `if \(__nevernull__\)`, 1)
			} else {
				Out, err = ctext.FoldAlways(marked, `if \(__nevernull__\)`, 1)
			}
			if err != nil {
				key := fmt.Sprintf("%s %s nullptr after %s", string(core[m[12]:m[13]]), op, call)
				if !seenLeft[key] {
					seenLeft[key] = true
					left = append(left, key+": "+err.Error())
				}
				continue
			}
			core = Out
			folded++
			changed = true
			from = m[0]
		}
		if !changed {
			return nnLabels(core), NeverNullSet(core, roots), folded, left
		}
	}
}

var nnLabel = regexp.MustCompile(`(?m)^([A-Za-z_]\w*):\n`)

// nnLabels drops a label nothing jumps to any more: its gotos were in the
// branches that folded.  A label is its function's, and a label with a goto
// left in the function stays.
func nnLabels(core []byte) []byte {
	fs := nnFuncs(core)
	type cut struct{ a, z int }
	var cuts []cut
	for _, r := range fs {
		fn := core[r[0]:r[1]]
		for _, m := range nnLabel.FindAllSubmatchIndex(fn, -1) {
			l := string(fn[m[2]:m[3]])
			if !regexp.MustCompile(`\bgoto ` + l + `;`).Match(fn) {
				cuts = append(cuts, cut{r[0] + m[0], r[0] + m[1]})
			}
		}
	}
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].a > cuts[j].a })
	for _, c := range cuts {
		core = append(append([]byte{}, core[:c.a]...), core[c.z:]...)
	}
	return core
}
