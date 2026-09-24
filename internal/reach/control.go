package reach

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/cc"
	"github.com/arbace/go-whim/internal/ccx"
	"github.com/arbace/go-whim/internal/dead"
)

// The control is gcc.  A closure that reports nothing is indistinguishable
// from one whose roots swallowed the program, so what makes the partition
// evidence is a second instrument that can disagree with it: gcc's call graph
// names every unreferenced static function, object and prototype, which is the
// F/O/P half of the closure one level deep (REACHABILITY.md measured
// 13/98/85 exact on its richest text).  Three controls:
//
//   - Agreement: on the text, gcc's unused set is exactly the closure's
//     unreachable F/O/P that nothing refers to;
//   - Planted: on a copy with one dead thing of every kind appended, the
//     closure reports exactly the plants on top of what it reported before,
//     and gcc names exactly the plants it can see;
//   - Positional: for every member held by a positional initialiser, gcc is
//     asked what deleting it does, and must not be silent where the tree
//     says an element is left with nowhere to go.

// The gcc sentences, as internal/dead reads them: the option in brackets is
// what says a function from a variable.
var (
	gccNeverDefined = regexp.MustCompile(`'(\w+)' declared 'static' but never defined`)
	gccDeadFunction = regexp.MustCompile(`'(\w+)' defined but not used \[-Wunused-function\]`)
	gccDeadVariable = regexp.MustCompile(`(?:'(\w+)' defined but not used|unused variable '(\w+)')` +
		` \[-Wunused(?:-const)?-variable=?\]`)
	gccDiagnostic = regexp.MustCompile(`^[^:]+:\d+:\d+: (warning|error): (.*)$`)
)

// gccUnused is one name gcc called unused, and the line it named.
type gccUnused struct {
	name string
	line int
}

// readUnused sorts gcc's warnings into the unused ones and the rest.
func readUnused(ws []dead.Warning) (unused []gccUnused, other []dead.Warning) {
	for _, w := range ws {
		var nm string
		for _, re := range []*regexp.Regexp{gccNeverDefined, gccDeadFunction, gccDeadVariable} {
			if m := re.FindStringSubmatch(w.Text); m != nil {
				nm = m[1]
				if nm == "" && len(m) > 2 {
					nm = m[2]
				}
				break
			}
		}
		if nm == "" {
			other = append(other, w)
			continue
		}
		unused = append(unused, gccUnused{nm, w.Line})
	}
	return unused, other
}

// locals is every block-scope declarator in the file, as name:line: what gcc
// may call unused that is no entity of the closure.
func locals(ast *cc.AST, path string) map[string]bool {
	out := map[string]bool{}
	walk(ast.TranslationUnit, func(n cc.Node) {
		if d, ok := n.(*cc.Declarator); ok && d.Name() != "" && d.LexicalScope() != ast.Scope &&
			d.Position().Filename == path {
			out[fmt.Sprintf("%s:%d", d.Name(), d.Position().Line)] = true
		}
	})
	return out
}

const (
	agreeBoth    = "unreachable, nothing refers to it, and gcc names it"
	agreeDeeper  = "unreachable, referred to only by the unreachable: gcc looks one level"
	agreeLocal   = "gcc names a block-scope declaration, which is no entity"
	plantFound   = "a planted entity the closure reports"
	plantGcc     = "a planted entity gcc names too"
	plantGccNot  = "a planted entity gcc cannot see (a type, a member, an enumerator, a callee)"
	plantKept    = "an unreachable entity of the text, reported again on the planted copy"
	gccSilent    = "deleted, and gcc is silent: the move is invisible to it"
	gccWarnsOnly = "deleted, and gcc only WARNS -- %s -- and exits %d"
	gccRefuses   = "deleted, and gcc refuses it (exit %d): %s"
)

// Agreement asks gcc for its unused set and requires it to be the closure's
// unreachable F/O/P that nothing refers to, both ways.
func Agreement(c *Closure, ast *cc.AST, path string) (ccx.Result, []gccUnused, error) {
	res := ccx.Result{Title: "control: gcc's unused set against the closure's unreachable F, O, P", Classes: map[string]int{
		agreeBoth: 0, agreeDeeper: 0, agreeLocal: 0,
	}}
	ws, err := dead.GccWarnings(path, "")
	if err != nil {
		return res, nil, err
	}
	unused, other := readUnused(ws)
	for _, w := range other {
		res.Left = append(res.Left, ccx.Finding{Fn: "gcc", Where: fmt.Sprintf("%s:%d", path, w.Line),
			What: "a warning this control does not read: " + w.Text})
	}
	loc := locals(ast, path)
	named := map[string]bool{}
	for _, u := range unused {
		if loc[fmt.Sprintf("%s:%d", u.name, u.line)] {
			res.Classes[agreeLocal]++
			continue
		}
		e := c.byKey["n:"+u.name]
		where := fmt.Sprintf("%s:%d", path, u.line)
		switch {
		case e == nil || (e.Kind != "F" && e.Kind != "O" && e.Kind != "P"):
			res.Left = append(res.Left, ccx.Finding{Fn: u.name, Where: where, What: "gcc calls it unused, and it is no F, O or P"})
		case e.reached:
			res.Left = append(res.Left, ccx.Finding{Fn: e.ID, Where: where, What: "gcc calls it unused, and the closure reaches it"})
		case e.Referenced():
			res.Left = append(res.Left, ccx.Finding{Fn: e.ID, Where: where, What: "gcc calls it unused, and the closure has something referring to it"})
		default:
			named[e.key] = true
			res.Classes[agreeBoth]++
		}
	}
	for _, e := range c.Unreachable() {
		if e.Kind != "F" && e.Kind != "O" && e.Kind != "P" || named[e.key] {
			continue
		}
		if e.Referenced() {
			res.Classes[agreeDeeper]++
			continue
		}
		res.Left = append(res.Left, ccx.Finding{Fn: e.ID, Where: fmt.Sprintf("%s:%d", path, e.Line),
			What: "unreachable and nothing refers to it, and gcc is silent"})
	}
	return res, unused, nil
}

// plant is one dead thing of every kind, and a call chain two deep.  It is
// appended at the end of the text, where every type it could need is defined.
const plant = `
static int reach_plant_callee(void) { return 1; }
static int reach_plant_caller(void) { return reach_plant_callee(); }
static int reach_plant_object;
static int reach_plant_proto(void);
typedef struct reach_plant_S { int reach_plant_member; } reach_plant_T;
enum reach_plant_E { REACH_PLANT_N = 1 };
`

// What the closure must report from the plant, and which of it gcc must name.
var (
	plantIDs = []string{
		"F:reach_plant_callee", "F:reach_plant_caller", "O:reach_plant_object", "P:reach_plant_proto",
		"T:reach_plant_T", "S:reach_plant_S", "M:reach_plant_S.reach_plant_member",
		"E:reach_plant_E", "N:REACH_PLANT_N",
	}
	plantGccNames = []string{"reach_plant_caller", "reach_plant_object", "reach_plant_proto"}
)

// Planted appends the plant to a copy of the text and requires the closure to
// report exactly the plant on top of what it reported on the text, and gcc to
// name exactly the plants its one level can see on top of what it named.
func Planted(c *Closure, src []byte, gccBefore []gccUnused, dir string) (ccx.Result, error) {
	res := ccx.Result{Title: "control: a planted copy -- one dead thing of every kind", Classes: map[string]int{
		plantFound: 0, plantGcc: 0, plantGccNot: 0, plantKept: 0,
	}}
	path := filepath.Join(dir, "planted.c")
	if err := os.WriteFile(path, append(append([]byte{}, src...), plant...), 0o644); err != nil {
		return res, err
	}
	ast, err := ccx.Parse(path)
	if err != nil {
		return res, fmt.Errorf("the planted copy does not parse: %v", err)
	}
	pc := Analyze(ast, path, append(append([]byte{}, src...), plant...))
	before := map[string]bool{}
	for _, e := range c.Unreachable() {
		before[e.ID] = true
	}
	planted := map[string]bool{}
	for _, id := range plantIDs {
		planted[id] = true
	}
	after := map[string]bool{}
	for _, e := range pc.Unreachable() {
		after[e.ID] = true
		switch {
		case planted[e.ID]:
			res.Classes[plantFound]++
		case before[e.ID]:
			res.Classes[plantKept]++
		default:
			res.Left = append(res.Left, ccx.Finding{Fn: e.ID, Where: fmt.Sprintf("%s:%d", path, e.Line),
				What: "unreachable on the planted copy only, and not planted"})
		}
	}
	for _, id := range plantIDs {
		if !after[id] {
			res.Left = append(res.Left, ccx.Finding{Fn: id, Where: path, What: "planted dead, and the closure does not report it"})
		}
	}
	for id := range before {
		if !after[id] {
			res.Left = append(res.Left, ccx.Finding{Fn: id, Where: c.Path, What: "unreachable on the text, reached on the planted copy"})
		}
	}

	ws, err := dead.GccWarnings(path, "")
	if err != nil {
		return res, err
	}
	unused, _ := readUnused(ws)
	was := map[gccUnused]bool{}
	for _, u := range gccBefore {
		was[u] = true
	}
	want := map[string]bool{}
	for _, n := range plantGccNames {
		want[n] = true
	}
	got := map[string]bool{}
	for _, u := range unused {
		switch {
		case want[u.name]:
			got[u.name] = true
			res.Classes[plantGcc]++
		case was[u]:
		default:
			res.Left = append(res.Left, ccx.Finding{Fn: u.name, Where: fmt.Sprintf("%s:%d", path, u.line),
				What: "gcc calls it unused on the planted copy only, and it is not a plant gcc can see"})
		}
	}
	for _, n := range plantGccNames {
		if !got[n] {
			res.Left = append(res.Left, ccx.Finding{Fn: n, Where: path, What: "planted unreferenced, and gcc does not name it"})
		}
	}
	res.Classes[plantGccNot] = len(plantIDs) - len(got)
	return res, nil
}

// Positional deletes, one at a time, every unreachable member an initialiser
// places a value in or after by position, compiles the copy with
// -fsyntax-only -Wall -Wextra, and reports what gcc says: an error, only
// warnings (and its exit status), or nothing.  The tree predicts one thing gcc
// must see -- an initialiser that fills the LAST member leaves an element with
// nowhere to go -- and a silent gcc there refuses.  A silent gcc anywhere else
// is the case the tree alone sees: values shift and nothing complains.
// REACHABILITY.md measured termrequest_T.tr_start: a warning, exit 0.
func Positional(c *Closure, src []byte, dir string) (ccx.Result, error) {
	res := ccx.Result{Title: "control: gcc on each positionally initialised member, deleted", Classes: map[string]int{}}
	path := filepath.Join(dir, "positional.c")
	var before map[string]bool
	for _, e := range c.Unreachable() {
		if e.Kind != "M" || c.Moves(e) == 0 {
			continue
		}
		if before == nil {
			// What gcc says about the text as it stands is not what the
			// deletion made it say.
			warned, errors, _, err := syntaxOnly(path, src)
			if err != nil {
				return res, err
			}
			if len(errors) > 0 {
				return res, fmt.Errorf("%s: gcc -fsyntax-only refuses the text itself: %s", c.Path, errors[0])
			}
			before = warned
		}
		where := fmt.Sprintf("%s:%d", c.Path, e.Line)
		d := c.memberDecl[e.key]
		if d[2] != 1 {
			res.Left = append(res.Left, ccx.Finding{Fn: e.ID, Where: where, What: "its declaration declares other members too: not deleted alone"})
			continue
		}
		a := bytes.LastIndexByte(src[:d[0]], '\n') + 1
		b := d[1] + bytes.IndexByte(src[d[1]:], '\n') + 1
		if strings.TrimSpace(string(src[a:b])) != strings.TrimSpace(string(src[d[0]:d[1]])) {
			res.Left = append(res.Left, ccx.Finding{Fn: e.ID, Where: where, What: "its declaration shares a line: not deleted alone"})
			continue
		}
		warned, errors, status, err := syntaxOnly(path, append(append([]byte{}, src[:a]...), src[b:]...))
		if err != nil {
			return res, err
		}
		var said []string
		for w := range warned {
			if !before[w] {
				said = append(said, w)
			}
		}
		sort.Strings(said)
		switch {
		case len(errors) > 0:
			res.Classes[fmt.Sprintf(gccRefuses, status, errors[0])]++
		case len(said) > 0:
			res.Classes[fmt.Sprintf(gccWarnsOnly, strings.Join(said, "; "), status)]++
		case c.FillsLast(e):
			res.Left = append(res.Left, ccx.Finding{Fn: e.ID, Where: where,
				What: "deleted, gcc is silent, though an initialiser fills the last member and one element now has nowhere to go"})
		default:
			res.Classes[gccSilent]++
		}
	}
	sort.Slice(res.Left, func(i, j int) bool { return res.Left[i].Fn < res.Left[j].Fn })
	return res, nil
}

// syntaxOnly writes text to path and asks gcc what it thinks of it: the
// distinct warnings, the errors and the exit status.
func syntaxOnly(path string, text []byte) (map[string]bool, []string, int, error) {
	if err := os.WriteFile(path, text, 0o644); err != nil {
		return nil, nil, 0, err
	}
	cmd := exec.Command("gcc", "-fsyntax-only", "-Wall", "-Wextra", "-Wno-unused-parameter", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	status := 0
	if err := cmd.Run(); err != nil {
		x, ok := err.(*exec.ExitError)
		if !ok {
			return nil, nil, 0, err
		}
		status = x.ExitCode()
	}
	warned, errors := map[string]bool{}, []string{}
	for _, l := range strings.Split(stderr.String(), "\n") {
		m := gccDiagnostic.FindStringSubmatch(l)
		switch {
		case m == nil:
		case m[1] == "error":
			errors = append(errors, m[2])
		default:
			warned[m[2]] = true
		}
	}
	return warned, errors, status, nil
}
