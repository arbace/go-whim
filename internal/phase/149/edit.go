package p149

// Whim phase 149 -- the allocation-failure branches fold.  See GOAL.md.
//
// Every NULL test of a never-NULL allocation's result that follows it folds,
// the never-NULL functions found to a fixpoint from host_alloc(); labels only
// the folded branches jumped to go (internal/gen/FINDINGS.md, 9).
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim149", Edit) }

// W149Base is where never-NULL starts: host_alloc() returns a pointer into the
// arena or ends the process (phase 148's check proves it on its input).
var W149Base = []string{"host_alloc"}

var (
	w149Def    = regexp.MustCompile(`(?m)^([A-Za-z_]\w*)\(`)
	w149Return = regexp.MustCompile(`\breturn\b\s*([^;]*);`)
	w149Cast   = `(?:\(\s*(?:const\s+)?(?:unsigned\s+)?(?:struct\s+)?\w+\s*\*+\s*\)\s*)?`
)

// w149Funcs is every function defined in text: name -> [start, end), found in
// one pass -- a name at the start of a line, its parameter list, then a brace
// -- where the start is the line before, which holds the return type.
func w149Funcs(text []byte) map[string][2]int {
	b := cutil.Blank(text)
	fs := map[string][2]int{}
	for _, m := range w149Def.FindAllSubmatchIndex(text, -1) {
		name := string(text[m[2]:m[3]])
		if _, seen := fs[name]; seen {
			continue
		}
		rp := cutil.Match(b, m[1]-1)
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
		e := cutil.Match(b, k)
		if e < 0 {
			continue
		}
		st := bytes.LastIndexByte(text[:m[0]-1], '\n') + 1
		fs[name] = [2]int{st, e + 1}
	}
	return fs
}

// w149IsNN: an expression is a call of a never-NULL function, under a cast.
func w149IsNN(e string, nn map[string]bool) bool {
	m := regexp.MustCompile(`^` + w149Cast + `([A-Za-z_]\w*)\(`).FindStringSubmatch(strings.TrimSpace(e))
	if m == nil || !nn[m[1]] {
		return false
	}
	// the call is the whole expression
	e = strings.TrimSpace(e)
	return strings.HasSuffix(e, ")") && cutil.Match(cutil.Blank([]byte(e)), strings.Index(e, m[1]+"(")+len(m[1])) == len(e)-1
}

// w149LocalNN: v is a local of fn whose every assignment is a never-NULL call,
// that is never incremented, and whose address is never taken.
func w149LocalNN(fn, v string, nn map[string]bool) bool {
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
		if !w149IsNN(rhs, nn) {
			return false
		}
	}
	return true
}

// W149NN is the set of functions that never return NULL, to its fixpoint on
// text: host_alloc(), and every function returning a pointer whose every
// return is a never-NULL call or a local assigned only never-NULL calls.
func W149NN(text []byte) map[string]bool {
	nn := map[string]bool{}
	for _, b := range W149Base {
		nn[b] = true
	}
	fs := w149Funcs(text)
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
			rets := w149Return.FindAllStringSubmatch(fn, -1)
			ok := len(rets) > 0
			for _, rt := range rets {
				e := strings.TrimSpace(rt[1])
				if w149IsNN(e, nn) {
					continue
				}
				if regexp.MustCompile(`^` + w149Cast + `([A-Za-z_]\w*)$`).MatchString(e) {
					v := regexp.MustCompile(`([A-Za-z_]\w*)$`).FindString(e)
					if w149LocalNN(fn, v, nn) {
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

// W149Rule folds every NULL test of a never-NULL allocation's result that
// directly follows it, recomputing the never-NULL set between rounds, to the
// fixpoint.  It returns the text, the final set, how many tests folded, and
// the sites it had to leave (an always-true test with an else).
func W149Rule(core []byte) ([]byte, map[string]bool, int, []string) {
	folded := 0
	var left []string
	seenLeft := map[string]bool{}
	for {
		nn := W149NN(core)
		names := make([]string, 0, len(nn))
		for n := range nn {
			names = append(names, n)
		}
		sort.Strings(names)
		alt := strings.Join(names, "|")
		// `v = f(...) ;` -- or `T *v = f(...);`, a declaration -- then `if (v OP
		// nullptr)` at the same indentation: on the next line, or after one line
		// that only stores v somewhere (`wp->w_frame = frp;`), which cannot
		// change it.  (The declaration and the store were the two shapes this
		// rule missed: map_add's and new_frame's tests, which the Go printed
		// as nil checks of a new() that staticcheck calls never true.)
		r1 := regexp.MustCompile(`(?m)^( *)(?:[A-Za-z_][\w ]*?\*+ *)?([\w.>\[\]-]+) = +` + w149Cast + `(?:` + alt + `)\([^;\n]*\) ?;\n(?:( *)[\w.>\[\]-]+ = ([\w.>\[\]-]+);\n)?( *)if \(([\w.>\[\]-]+) (==|!=) nullptr\)\n`)
		// `if ((v = f(...)) == nullptr)`
		r2 := regexp.MustCompile(`(?m)^( *)if \(\(([\w.>\[\]-]+) = (` + w149Cast + `(?:` + alt + `)\([^;\n]*\))\) == nullptr\)\n`)
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
				if w149IsNN(call, nn) {
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
			if !w149IsNN(call, nn) {
				continue
			}
			ifAt := m[10]
			op := string(core[m[14]:m[15]])
			marked := append(append(append([]byte{}, core[:ifAt]...), []byte(string(core[m[10]:m[11]])+"if (__w149__)\n")...), core[m[1]:]...)
			var Out []byte
			var err error
			if op == "==" {
				Out, err = cutil.FoldNever(marked, `if \(__w149__\)`, 1)
			} else {
				Out, err = cutil.FoldAlways(marked, `if \(__w149__\)`, 1)
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
			return w149Labels(core), W149NN(core), folded, left
		}
	}
}

var w149Label = regexp.MustCompile(`(?m)^([A-Za-z_]\w*):\n`)

// w149Labels drops a label nothing jumps to any more: its gotos were in the
// branches that folded.  A label is its function's, and a label with a goto
// left in the function stays.
func w149Labels(core []byte) []byte {
	fs := w149Funcs(core)
	type cut struct{ a, z int }
	var cuts []cut
	for _, r := range fs {
		fn := core[r[0]:r[1]]
		for _, m := range w149Label.FindAllSubmatchIndex(fn, -1) {
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

// Whim149 folds the allocation-failure branches.
//
// Since phase 148 no allocation can fail, and a function whose every return is
// an allocation, or a local only ever assigned one, cannot return NULL either:
// vim_strsave(), vim_strnsave() and the rest, found to a fixpoint from
// host_alloc() (W149NN).  Every NULL test of such a call's result that follows
// it directly is never true when it is `== nullptr` and always true when it is
// `!= nullptr`, and folds; folding one can make another function never-NULL,
// so the rule recomputes the set and goes on until nothing changes.  The
// branches that released memory, said E342 or returned FAIL go, and so does a
// label only they jumped to; the sweep takes what only they used.  The Go transpilation never had them (internal/gen/FINDINGS.md, 9).
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "allocnull", W: w}
	i := bytes.Index(text, []byte("\n#include"))
	if i < 0 {
		return nil, p.Die("no #include: the boundary is not where this phase expects it")
	}
	core, nn, n, left := W149Rule(text[:i+1])
	if n < 80 {
		return nil, p.Die("%d allocation-failure tests fold; this phase was written against a hundred or so", n)
	}
	p.Say(fmt.Sprintf("%d NULL tests of a never-NULL allocation's result fold; %d functions never return NULL", n, len(nn)))
	for _, l := range left {
		p.Say("left: " + l)
	}
	return append(core, text[i+1:]...), nil
}
