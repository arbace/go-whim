package graph

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/edit"
)

// THE SEED'S SPELLING ON THE GRAPH (R3, doc/GRAPH-MIGRATION.md): phase 0's
// parts 0a and 0c, which were crefactor/xform's NullptrUsize and Attrs on
// the text.  Each asserts what the text step asserted and reports what it
// reported, the counts the text took of the file (literals, mentions,
// lines) asked of the C view, and the edits made on nodes: NULL's tokens
// and size_t's uses respelled, a typedef spliced, attributes deleted and
// respelled.  Generic: what a code base adds is NullptrKnobs.

// NullptrKnobs is what NullptrUsize is told.
type NullptrKnobs struct {
	// NullLiterals are the string literals that hold `NULL`, which stay as
	// they are; when not nil, any other set refuses.
	NullLiterals []string
}

const r3Typedef = "typedef typeof(sizeof(0)) usize;"

var (
	r3NullRE  = regexp.MustCompile(`\bNULL\b`)
	r3SizeRE  = regexp.MustCompile(`\bsize_t\b`)
	r3IntroRE = regexp.MustCompile(`(?m)^[^\n]*\btypedef\b[^\n]*\busize\b[^\n]*$`)
)

// r3Directives says the file's include forms are its first forms, each an
// include of a system header, and no other directive is in it: the text's
// "exactly its N directives on its first N lines", of forms.
func (v *Verbs) r3Directives() int {
	n := len(v.e.Includes())
	for i, f := range v.e.g.Forms {
		switch {
		case f.Is("directive"):
			v.Die("a directive is not an `#include <...>` of a system header, and no step may add one")
			return 0
		case IsInclude(f) && (i >= n || !strings.HasPrefix(IncludeSpec(f), "<")):
			v.Die("the file does not have exactly its %d preprocessor directives as its first %d forms, each an `#include <...>`", n, n)
			return 0
		}
	}
	return n
}

// r3View is the file's C view, or a refusal.
func (v *Verbs) r3View() []byte {
	if v.Err != nil {
		return nil
	}
	t, err := v.e.g.C()
	if err != nil {
		v.Die("the C view: %v", err)
		return nil
	}
	return t
}

// r3Strings are the string-shaped atoms in the file that are not string
// literals of its C -- a macro invocation's text, a verbatim form's -- and
// the forms that hold them.
func (v *Verbs) r3Texts(f func(holder, atom *Node)) {
	for _, form := range v.e.g.Forms {
		Walk(form, func(n *Node) bool {
			if n.list && len(n.Kids) > 1 && !n.Kids[1].list {
				switch n.Head() {
				case "macro", "macro-decl", "verbatim", "attr-text", "asm-label":
					f(n, n.Kids[1])
				}
			}
			return true
		})
	}
}

// NullptrUsize gives the file two names the language supplies in place of
// two a header does: every NULL token becomes nullptr, a C23 keyword, and
// every use of size_t a use of usize, declared by ONE new typedef directly
// below the file's includes, `typedef typeof(sizeof(0)) usize;` -- size_t
// by definition, on any target -- and every `(void *)NULL` plain nullptr,
// which is typed, so the cast says nothing.  A string literal is data and
// is not a token: it is not touched.
//
// It refuses a size_t written where no use of the header's typedef is (a
// macro's text), a file whose directives are not its includes at its top,
// a file that already names usize or nullptr, a literal holding size_t, a
// set of literals holding NULL other than k's, and fewer than casts
// `(void *)NULL`.
func (v *Verbs) NullptrUsize(k NullptrKnobs, casts int) {
	if v.Err != nil {
		return
	}
	e := v.e
	p := edit.Ph{Tag: v.Tag, W: io.Discard}
	text := v.r3View()
	nInc := v.r3Directives()
	if v.Err != nil {
		return
	}
	for _, name := range []string{"usize", "nullptr"} {
		if n := edit.MentionCount(text, name); n != 0 {
			v.Die("`%s` already occurs %d times -- this step introduces it, so an existing "+
				"mention means the step has already run or the name is taken", name, n)
			return
		}
	}
	v.Sayf("%d directives, every one an `#include <...>` on the first %d lines, and "+
		"`usize` and `nullptr` at zero mentions", nInc, nInc)

	// ---- 1. the literals: data, which no token edit reaches -------------
	spans, err := edit.LiteralSpans(p, text)
	if err != nil {
		v.Die("%v", err)
		return
	}
	var holding []string
	inLits := 0
	for _, s := range spans {
		if n := len(r3NullRE.FindAll(text[s[0]:s[1]], -1)); n > 0 {
			holding = append(holding, string(text[s[0]:s[1]]))
			inLits += n
		}
		if r3SizeRE.Match(text[s[0]:s[1]]) {
			v.Die("a literal contains `size_t`, and a rename may not reach into data")
			return
		}
	}
	if k.NullLiterals != nil {
		a, b := append([]string(nil), holding...), append([]string(nil), k.NullLiterals...)
		sort.Strings(a)
		sort.Strings(b)
		if strings.Join(a, "\x00") != strings.Join(b, "\x00") {
			j := strings.Join(a, " / ")
			if j == "" {
				j = "none"
			}
			v.Die("the literals containing `NULL` are not the %d this step was told of: %s", len(k.NullLiterals), j)
			return
		}
	}
	v.Sayf("%d string and character literals, %d of which contain `NULL` and none of which "+
		"contains `size_t`; a rename outside them leaves them as they are", len(spans), len(holding))

	// ---- 2. every size_t is a use of the header's typedef ---------------
	// The text's partition (casts and declarations, nothing left over) is
	// by edge here: every mention outside the literals is a use of the
	// external typedef, so a typedef serves it; a cast is a use that is a
	// cast's or a sizeof's whole type, `(size_t)`.
	var ext *Node
	for _, x := range e.g.Externs {
		if x.Is("extern-typedef") && len(x.Kids) > 1 && x.Kids[1].Atom == "size_t" {
			ext = x
		}
	}
	var uses []*Node
	if ext != nil {
		uses = e.Uses(ext)
	}
	textual := len(r3SizeRE.FindAll(text, -1))
	if len(uses) != textual {
		v.Die("`size_t` is written %d times and %d of them are uses of the header's typedef: the rest "+
			"are in text no rename by edge reaches", textual, len(uses))
		return
	}
	cast, decl := 0, 0
	for _, u := range uses {
		if u.list {
			v.Die("`size_t` is named in %s's text, which a respelling cannot reach", label(u))
			return
		}
		if q := e.Parent(u); q != nil && (q.Is("cast") || q.Is("sizeof-type") || q.Is("alignof-type")) && len(q.Kids) > 1 && q.Kids[1] == u {
			cast++
		} else {
			decl++
		}
	}
	v.Sayf("%d mentions of `size_t`, ALL of them type-name positions: %d casts and %d "+
		"declarations, and nothing left over", cast+decl, cast, decl)

	// ---- 3. the typedef, directly below the includes --------------------
	incs := e.Includes()
	if len(incs) == 0 {
		v.Die("no include for the typedef to go below")
		return
	}
	ns, err := e.SpliceC(Frag{At: e.SpotAfter(incs[len(incs)-1]), Src: r3Typedef})
	if err != nil {
		v.Die("the typedef: %v", err)
		return
	}
	var usize *Node
	for _, n := range ns[0] {
		if n.Is("typedef") {
			usize = n
		}
	}
	if usize == nil {
		v.Die("the typedef: no typedef form spliced")
		return
	}

	// ---- 4. the substitution: tokens and uses, never data ---------------
	nCast := 0
	var nulls []*Node
	for _, f := range e.g.Forms {
		Walk(f, func(n *Node) bool {
			if n.Is("cast") && len(n.Kids) == 3 && n.Kids[1].Is("ptr") && len(n.Kids[1].Kids) == 2 &&
				!n.Kids[1].Kids[1].list && n.Kids[1].Kids[1].Atom == "void" && !n.Kids[2].list && n.Kids[2].Atom == "NULL" {
				nulls = append(nulls, n)
				nCast++
				return false
			}
			if !n.list && n.Atom == "NULL" {
				nulls = append(nulls, n)
			}
			return true
		})
	}
	if err := e.RespellTokens(nulls, "nullptr"); err != nil {
		v.Die("`NULL` -> `nullptr`: %v", err)
		return
	}
	inText := 0
	var respell [][2]*Node
	v.r3Texts(func(holder, atom *Node) {
		if r3SizeRE.MatchString(atom.Atom) {
			v.Die("`size_t` is said in %s's text", label(holder))
		}
		if k := len(r3NullRE.FindAllString(atom.Atom, -1)); k > 0 {
			inText += k
			respell = append(respell, [2]*Node{holder, atom})
		}
	})
	if v.Err != nil {
		return
	}
	for _, r := range respell {
		if err := e.RespellText(r[0], r3NullRE.ReplaceAllString(r[1].Atom, "nullptr")); err != nil {
			v.Die("`NULL` in %s's text: %v", label(r[0]), err)
			return
		}
	}
	if _, err := e.RetargetUses(ext, usize); ext != nil && err != nil {
		v.Die("`size_t` -> `usize`: %v", err)
		return
	}
	v.Sayf("`NULL` -> `nullptr` at %d sites and `size_t` -> `usize` at %d, in one pass and "+
		"outside every literal", len(nulls)+inText, len(uses))
	if nCast < casts {
		v.Die("%d `(void *)NULL` sites, and this step was told there are at least %d", nCast, casts)
		return
	}
	v.Sayf("%d `(void *)nullptr` -> `nullptr`: the cast existed for the variadic hazard, and "+
		"`nullptr` is typed, so it says nothing a reader needs", nCast)

	// ---- 5. what the file is now ----------------------------------------
	text = v.r3View()
	if v.r3Directives(); v.Err != nil {
		return
	}
	at := slicesIndex(e.g.Forms, usize)
	if at != nInc {
		v.Die("the typedef is not the form directly below the %d includes", nInc)
		return
	}
	for _, c := range []struct {
		name string
		want int
	}{{"NULL", inLits}, {"size_t", 0}} {
		if n := edit.MentionCount(text, c.name); n != c.want {
			v.Die("`%s` has %d mentions after the cut, expected %d", c.name, n, c.want)
			return
		}
	}
	after, err := edit.LiteralSpans(p, text)
	if err != nil {
		v.Die("%v", err)
		return
	}
	var still []string
	for _, s := range after {
		if r3NullRE.Match(text[s[0]:s[1]]) {
			still = append(still, string(text[s[0]:s[1]]))
		}
	}
	if strings.Join(still, "\x00") != strings.Join(holding, "\x00") {
		v.Die("the literals holding `NULL` are not the ones they were")
		return
	}
	intro := r3IntroRE.FindAllString(string(text), -1)
	if len(intro) != 1 || intro[0] != r3Typedef {
		v.Die("`usize` is introduced by something other than exactly one typedef: %s -- it is "+
			"a TYPE NAME and not a static object, and every one of its uses is a type-name position",
			strings.Join(intro, " / "))
		return
	}
	line := bytes.Count(text[:bytes.Index(text, []byte(r3Typedef))], []byte("\n")) + 1
	v.Sayf("the %d `NULL` in literals are the only `NULL` left, `size_t` is at zero, `usize` "+
		"is a typedef on line %d and %d runs of two blank lines, exactly as before",
		inLits, line, p.BlankRuns(text))
}

func slicesIndex(s []*Node, n *Node) int {
	for i, x := range s {
		if x == n {
			return i
		}
	}
	return -1
}

// DeleteAttr deletes an attribute form from the declaration that carries
// it -- a parameter, a local's or the file's def, a defn's prefix -- and
// nothing else: refused for an attribute that is a statement's (an
// `attributed` form is the statement; respell or delete it).
func (e *Editor) DeleteAttr(a *Node) error {
	p, i := e.index(a)
	if i < 0 {
		return fmt.Errorf("delete attribute #%d: not in the graph", a.ID)
	}
	if !isAttrForm(a) || p.Is("attributed") || i < 1 {
		return fmt.Errorf("delete attribute #%d (%s): not an attribute of a declaration", a.ID, label(a))
	}
	return e.spliceAs("delete", p, i, i+1, nil, placeItem)
}

// RespellAttr writes a GNU attribute form `__attribute__((x))` as C23's
// `[[x]]`, a fresh node in its place: an attribute is not an expression,
// and has no type to keep or clear.
func (e *Editor) RespellAttr(a *Node) error {
	if !a.Is("attr") || len(a.Kids) != 2 || a.Kids[1].list {
		return fmt.Errorf("respell attribute #%d (%s): not a GNU attribute of one word", a.ID, label(a))
	}
	n := NewList(NewAtom("std-attr"), NewAtom(a.Kids[1].Atom))
	if err := e.Replace(a, n); err != nil {
		return err
	}
	out := e.Untyped[:0]
	for _, u := range e.Untyped {
		if u != n {
			out = append(out, u)
		}
	}
	e.Untyped = out
	return nil
}

// AttrKind is an attribute form's name: `unused`, `format` for
// `__attribute__((format(printf, 1, 2)))`; "" for a form that is not one.
func AttrKind(a *Node) string {
	if !a.Is("attr") && !a.Is("std-attr") || len(a.Kids) < 2 {
		return ""
	}
	if k := a.Kids[1]; !k.list {
		return k.Atom
	} else {
		return k.Head()
	}
}

// r3Attrs are the file's attribute forms: an `attr` or `std-attr` list
// that is not a parameter or a member named attr -- an element of a declaration after
// its first, or an attributed statement's.
func (e *Editor) r3Attrs() []*Node {
	var out []*Node
	for _, f := range e.g.Forms {
		Walk(f, func(n *Node) bool {
			if !n.list {
				return false
			}
			for i, k := range n.Kids {
				if i == 0 || !k.Is("attr") && !k.Is("std-attr") {
					continue
				}
				if isParamList(n) || isMemberForm(e, k) {
					continue // a parameter or a member named attr
				}
				out = append(out, k)
			}
			return true
		})
	}
	return out
}

var (
	r3AttrRE   = regexp.MustCompile(`__attribute__\(\((\w+)`)
	r3WordsRE  = regexp.MustCompile(`attribute|fallthrough|unused`)
	r3FallRE   = regexp.MustCompile(`(?m)^[ ]*__attribute__\(\(fallthrough\)\);$`)
	r3C23RE    = regexp.MustCompile(`(?m)^[ ]*\[\[fallthrough\]\];$`)
	r3PadRE    = regexp.MustCompile(`  [,)]`)
	r3AttrKind = []string{"unused", "fallthrough", "format", "format_arg", "cold"}
	r3AttrKept = []string{"format", "format_arg", "cold"}
)

// Attrs takes the GNU attributes that say nothing under the flags the
// sweep compiles with, and respells the one C23 has: every `unused` -- on a
// function definition's parameter or a local -- deleted, every
// `__attribute__((fallthrough));` statement `[[fallthrough]];`, and the
// `format`, `format_arg` and `cold` attributes, which are the flag, kept.
// It refuses an attribute of another kind, an `unused` anywhere else, a
// literal holding `__attribute__`, and a file that has C23's spelling
// already.
func (v *Verbs) Attrs() {
	if v.Err != nil {
		return
	}
	e := v.e
	p := edit.Ph{Tag: v.Tag, W: io.Discard}
	text := v.r3View()
	nInc := v.r3Directives()
	if v.Err != nil {
		return
	}
	spans, err := edit.LiteralSpans(p, text)
	if err != nil {
		v.Die("%v", err)
		return
	}
	inLit := 0
	var bad, words []string
	for _, s := range spans {
		lit := string(text[s[0]:s[1]])
		inLit += strings.Count(lit, "[[")
		if strings.Contains(lit, "__attribute__") {
			bad = append(bad, lit)
		}
		if r3WordsRE.MatchString(lit) {
			words = append(words, lit)
		}
	}
	if len(bad) > 0 {
		v.Die("a literal holds `__attribute__`, and no substitution below may reach inside a string: %s", strings.Join(bad, " / "))
		return
	}
	attrs := e.r3Attrs()
	for _, a := range attrs {
		if a.Is("std-attr") {
			v.Die("a C23 attribute is in the file already (%s) -- this step introduces C23 attribute "+
				"syntax, so an existing one means the step has already run", label(a))
			return
		}
	}
	if k := strings.Count(string(text), "[[") - inLit; k != 0 {
		v.Die("`[[` already occurs %d times outside the literals", k)
		return
	}
	v.Sayf("%d directives, every one an `#include <...>` on the first %d lines, and "+
		"`[[` at zero occurrences outside the literals (%d inside)", nInc, nInc, inLit)
	v.Sayf("%d string and character literals, NONE holding `__attribute__`.  %d hold "+
		"the English words and none is reachable by either substitution: %s",
		len(spans), len(words), strings.Join(words, " / "))

	kinds := map[string]int{}
	var extra []string
	for _, a := range attrs {
		k := AttrKind(a)
		kinds[k]++
		if !edit.Contains(r3AttrKind, k) && !edit.Contains(extra, k) {
			extra = append(extra, k)
		}
	}
	if n := len(r3AttrRE.FindAllString(string(text), -1)); n != len(attrs) {
		v.Die("the C view writes %d `__attribute__` and the graph holds %d attribute forms", n, len(attrs))
		return
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		v.Die("the file holds an attribute this step has never looked at: %s -- its decisions below "+
			"are about %s and nothing else", strings.Join(extra, " "), strings.Join(r3AttrKind, " "))
		return
	}
	parts := make([]string, len(r3AttrKind))
	for i, k := range r3AttrKind {
		parts[i] = fmt.Sprintf("%s %d", k, kinds[k])
	}
	v.Sayf("%d `__attribute__` in the file, and every one is one of five kinds: %s",
		len(attrs), strings.Join(parts, ", "))

	// the unused: on a definition's parameter, or a local's declaration
	var unused, falls []*Node
	headers := map[*Node]bool{}
	nLocal := 0
	keepForms := map[*Node]bool{}
	for _, a := range attrs {
		switch AttrKind(a) {
		case "unused":
			unused = append(unused, a)
			d := e.Parent(a)
			fn := e.Function(a)
			switch {
			case d != nil && d.Is("def") && fn != nil && fn != d:
				nLocal++
			case d != nil && fn != nil && e.r3DefnParam(fn, d):
				headers[fn] = true
			default:
				v.Die("the `unused` in %s is not on a function DEFINITION's parameter nor on a local's "+
					"declaration, so this may be an attribute on an object, a type or a field and the "+
					"step has no decision for those", label(e.TopForm(a)))
				return
			}
		case "fallthrough":
			if s := e.Parent(a); s == nil || !s.Is("attributed") || len(s.Kids) != 2 || e.Item(s) != s {
				v.Die("a `fallthrough` attribute in %s is not a statement of its own", label(e.TopForm(a)))
				return
			}
			falls = append(falls, a)
		default:
			keepForms[e.TopForm(a)] = true
		}
	}
	v.Sayf("%d `__attribute__((unused))`: %d in the parameter list of a function DEFINITION -- "+
		"%d header lines, every one followed by `{` -- and %d on a local's declaration, a "+
		"statement of its own; not one on an object, a type or a field.  An unused "+
		"parameter is not diagnosed under -Wno-unused-parameter, and a local nothing reads "+
		"is the sweep's to delete, which is why they say nothing",
		len(unused), len(unused)-nLocal, len(headers), nLocal)
	if n := len(r3FallRE.FindAllString(string(text), -1)); n != len(falls) {
		v.Die("%d of the %d `fallthrough` attributes are a whole line of their own", n, len(falls))
		return
	}
	v.Sayf("%d `__attribute__((fallthrough));`, every one a standalone statement on a line of "+
		"its own, so the swap to the C23 spelling is one-for-one and reaches nothing else", len(falls))
	wantKeep := 0
	for _, k := range r3AttrKept {
		wantKeep += kinds[k]
	}
	v.Sayf("%d `format`/`format_arg` on %d lines KEPT, and they are the only attributes doing "+
		"work nothing else does: without them the compiler checks no format string "+
		"they name; %d `cold` beside them kept too", kinds["format"]+kinds["format_arg"], len(keepForms),
		kinds["cold"])

	padBefore := len(r3PadRE.FindAllString(string(text), -1))
	for _, a := range unused {
		if err := e.DeleteAttr(a); err != nil {
			v.Die("%v", err)
			return
		}
	}
	for _, a := range falls {
		if err := e.RespellAttr(a); err != nil {
			v.Die("%v", err)
			return
		}
	}
	v.Sayf("%d `__attribute__((unused))` deleted with the space before them, and %d "+
		"`__attribute__((fallthrough));` respelled `[[fallthrough]];`", len(unused), len(falls))

	out := v.r3View()
	if v.Err != nil {
		return
	}
	L, lines := strings.Split(string(out), "\n"), strings.Split(string(text), "\n")
	if len(L) != len(lines) {
		v.Die("the file is %d lines and the input was %d -- both edits are WITHIN lines and "+
			"neither may add or remove one", len(L)-1, len(lines)-1)
		return
	}
	var gotK, wantK []string
	for _, m := range r3AttrRE.FindAllStringSubmatch(string(out), -1) {
		gotK = append(gotK, m[1])
	}
	for _, k := range r3AttrKept {
		for i := 0; i < kinds[k]; i++ {
			wantK = append(wantK, k)
		}
	}
	sort.Strings(gotK)
	sort.Strings(wantK)
	if strings.Join(gotK, " ") != strings.Join(wantK, " ") {
		v.Die("the attributes left are %s, and they must be exactly the %d format, %d format_arg and %d cold",
			strings.Join(gotK, " "), kinds["format"], kinds["format_arg"], kinds["cold"])
		return
	}
	if len(r3FallRE.FindAllString(string(out), -1)) > 0 || strings.Contains(string(out), "__attribute__((unused))") {
		v.Die("an `unused` or a GNU `fallthrough` survives the substitution")
		return
	}
	if len(r3C23RE.FindAllString(string(out), -1)) != len(falls) {
		v.Die("the %d C23 statements are not %d standalone lines", len(falls), len(falls))
		return
	}
	if k := len(r3PadRE.FindAllString(string(out), -1)); k != padBefore {
		v.Die("the edit left %d doubled spaces before a `,` or `)` where there were %d", k, padBefore)
		return
	}
	changed, unusedLines := 0, 0
	for i := range L {
		if L[i] != lines[i] {
			changed++
			if strings.Contains(lines[i], "__attribute__((unused))") {
				unusedLines++
			}
		}
	}
	if changed != unusedLines+len(falls) {
		v.Die("%d lines changed, expected %d -- the %d lines holding an `unused` and the %d fallthrough "+
			"statements, and nothing else", changed, unusedLines+len(falls), unusedLines, len(falls))
		return
	}
	v.Sayf("%d attributes -> %d, the same %d lines, %d changed -- the %d lines that held an "+
		"`unused` and the %d fallthrough statements -- and the doubled-space count unmoved at %d",
		len(attrs), len(gotK), len(L)-1, changed, unusedLines, len(falls), padBefore)
}

// r3DefnParam says d is a parameter of the function definition fn's own
// parameter list.
func (e *Editor) r3DefnParam(fn, d *Node) bool {
	pl := e.Parent(d)
	if pl == nil || !isParamList(pl) {
		return false
	}
	ft := e.Parent(pl)
	return ft != nil && ft.Is("fn") && e.Parent(ft) == fn
}

// RespellTokens replaces each of ns -- a token with no edge (a macro's,
// `NULL`), or an expression whose C is one -- by a fresh token spelled to,
// one act each, KEEPING THE TYPED EDGES ABOVE IT: the caller says the new
// token is typed as what it replaces (`nullptr` for `NULL` and for
// `(void *)NULL`: cc types each `void *`), which Replace cannot know of an
// atom without a type and so clears.
func (e *Editor) RespellTokens(ns []*Node, to string) error {
	with := make([]*Node, len(ns))
	for i := range ns {
		with[i] = NewAtom(to)
	}
	return e.replaceKeepingTypes(ns, with)
}

// RespellText says a macro invocation's (or a verbatim form's) text anew,
// its edges and the typed edges above it kept: the caller says the text
// means what it meant -- a token in it respelled as RespellTokens does.
func (e *Editor) RespellText(holder *Node, text string) error {
	if !holder.list || len(holder.Kids) < 2 || holder.Kids[1].list || !isStringAtom(holder.Kids[1].Atom) {
		return fmt.Errorf("respell #%d (%s): not a form with a text", holder.ID, label(holder))
	}
	return e.replaceKeepingTypes([]*Node{holder.Kids[1]}, []*Node{NewAtom(text)})
}

func (e *Editor) replaceKeepingTypes(ns, with []*Node) error {
	type kept struct{ q, t *Node }
	var saved []kept
	for i, n := range ns {
		for q := e.Parent(n); q != nil && q.Type != nil && !IsStatement(q) && q.up != e.top[0]; q = e.Parent(q) {
			saved = append(saved, kept{q, q.Type})
		}
		if err := e.Replace(n, with[i]); err != nil {
			return err
		}
	}
	back := map[*Node]bool{}
	for _, k := range saved {
		if k.q.Type == nil && e.Live(k.q) {
			k.q.Type = k.t
			e.typed(k.q)
			back[k.q] = true
		}
	}
	out := e.Untyped[:0]
	for _, u := range e.Untyped {
		if !back[u] {
			out = append(out, u)
		}
	}
	e.Untyped = out
	return nil
}
