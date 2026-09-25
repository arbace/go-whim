package sweep

import (
	"bytes"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/arbace/go-whim/crefactor/cc"
	"github.com/arbace/go-whim/crefactor/edit"
)

// Prune is the sweep: ONE reachability closure over the syntax tree of the
// text, and the text with everything it did not reach cut out.  It asks gcc
// nothing and type-checks nothing -- cc.Parse is the whole of the front end
// it needs, which every text a phase hands on passes -- and it replaces the
// six deleters that looped to a fixpoint, each seeing one kind of thing:
// what they found between them is one closure, and a closure is found once.
//
// WHAT IS REACHED.  The roots are main and whatever a file-scope
// static_assert names.  A reached thing reaches every name its text mentions,
// and a name is a KEY in one of C's name spaces, told apart by the token
// before it:
//
//	o:NAME  an ordinary identifier -- a function, an object, a typedef, an
//	        enumerator; every declaration of one name is one thing
//	t:TAG   after struct, union or enum
//	m:NAME  after . or -> (a designator's `.name` included)
//
// by NAME, not by type: `p->next` keeps every member called next whose struct
// is kept.  That is the rule the sweep has always had -- a member nothing
// names anywhere -- and over-keeping is the direction that is recoverable.  A
// member lives when its struct does AND its name is mentioned by something
// live.  A local that happens to share a global's name keeps the global,
// which is the same conservative rule funcreach had.
//
// WHAT IS KEPT THAT IS NOT REACHED, the guards, each one a way deleting a
// member or an enumerator would change something the closure cannot see:
//
//   - while a FreezeLayoutIf function (vim: ml_recover) is defined a struct
//     layout is a disk format, and no member goes;
//   - a struct initialised by position -- a brace list with an element that
//     names no member, `{STATUS_GET, -1}` -- keeps every member, since deleting
//     one moves the values after it; and so does every struct such a struct
//     holds by value, whose braces are positional inside it;
//   - a struct is never emptied, nor an enum: every member stays instead;
//   - deleting an enumerator renumbers the implicit ones after it, so the first
//     survivor after each deleted run is written with the value it had, and when
//     that value cannot be computed from the text the run stays.
//
// And one thing that is not at file scope: a local variable its function never
// mentions again, which gcc's -Wunused-variable was the sweep's way of
// finding.  It goes with its initialiser, as it did then, even one that calls
// something -- `int fuzzy = cmdline_fuzzy_complete(pat) && ...;` at phase 60 is
// one, and deleting it is what makes those functions dead -- and Stats counts
// how many such initialisers went, so that one that mattered would be seen.
//
// It loops, because deleting a local can orphan what only its initialiser
// named; a second round finding nothing is the fixpoint.
// Options is what the sweep must be told about the program it sweeps, rather
// than know: nothing in this package names anything in any one program.
type Options struct {
	// Roots are the functions and objects reached whatever else is: the
	// program's entry points.  A whole program has one, `main`.
	Roots []string
	// FreezeLayoutIf names functions whose presence makes every struct layout
	// a format another program reads (vim's ml_recover reads swap files): while
	// one is defined, no member of any struct goes.
	FreezeLayoutIf []string
}

func Prune(src []byte, name string, opt Options) ([]byte, Stats, error) {
	var total Stats
	for round := 1; ; round++ {
		out, st, err := pruneOnce(src, name, opt)
		if err != nil {
			return nil, total, fmt.Errorf("round %d: %w", round, err)
		}
		total.add(st)
		total.Rounds = round
		// The closure is complete in one round: what it did not reach, it cut.
		// Only a deleted local can orphan more -- the one use of another local,
		// or of a file-scope thing -- so only then is there a next round.
		if st.Locals == 0 || bytes.Equal(out, src) {
			return out, total, nil
		}
		if round >= MaxRounds {
			return nil, total, fmt.Errorf("not converging after %d rounds", round)
		}
		src = out
	}
}

// Stats is what one sweep removed, by kind, and what its guards kept.
type Stats struct {
	Rounds                                 int
	Funcs, Objects, Protos, Typedefs, Tags int
	Members, Enumerators, Locals           int
	Pinned                                 int // survivors written with the value they had
	ActingLocals                           int // of Locals, those whose initialiser called or assigned something
	KeptRuns                               int // enumerators kept because a survivor's value is not computable
	Positional                             int // structs initialised by position, whose members all stay
	Lines                                  int
}

func (s *Stats) add(o Stats) {
	s.Funcs += o.Funcs
	s.Objects += o.Objects
	s.Protos += o.Protos
	s.Typedefs += o.Typedefs
	s.Tags += o.Tags
	s.Members += o.Members
	s.Enumerators += o.Enumerators
	s.Locals += o.Locals
	s.Pinned += o.Pinned
	s.ActingLocals += o.ActingLocals
	s.KeptRuns += o.KeptRuns
	if o.Positional > s.Positional {
		s.Positional = o.Positional
	}
	s.Lines += o.Lines
}

func (s Stats) String() string {
	return fmt.Sprintf("functions %d, objects %d, prototypes %d, typedefs %d, tags %d, members %d, "+
		"enumerators %d (%d survivors pinned), locals %d (%d whose initialiser called or assigned) -- "+
		"%d lines, %d rounds; kept: %d enumerators before a survivor with no computable value, the members "+
		"of %d structs initialised by position",
		s.Funcs, s.Objects, s.Protos, s.Typedefs, s.Tags, s.Members, s.Enumerators, s.Pinned, s.Locals, s.ActingLocals,
		s.Lines, s.Rounds, s.KeptRuns, s.Positional)
}

// ent is one thing the closure keeps or cuts.
type ent struct {
	kind byte   // F function, O object, P prototype, T typedef, S struct/union, E enum, M member, N enumerator, D a tag declared, not defined
	name string // the bare name; a member's, an enumerator's
	key  string // what makes it live; a member's is m:NAME and needs its owner too
	refs []string
	live bool

	span [2]int // what deleting it removes: the item in its list, or the whole construct

	// S and E
	owner    *ent   // M, N: the definition; S nested in a member: nil
	members  []*ent // S: in order; E: its enumerators in order
	lists    [][]*ent
	nested   []*ent // M: the struct and union definitions written in it
	declSpan [2]int // M, first of its StructDeclaration: that declaration whole
	pinAll   bool   // S: every member lives with it
	hold     bool   // M: lives whenever its struct does (it defines a tag, or has no name)

	// N
	explicit bool
	nameEnd  int
	val      int64
	valOK    bool
}

// decl is one file-scope declaration, as a unit of deletion.
type decl struct {
	span  [2]int // up to and including the ';' or the function's '}'
	fn    *ent
	fd    *cc.FunctionDefinition
	items []*ent // its declarators, in order
	tags  []*ent // the tag definitions or declarations in its specifiers, outermost
	tagAt [2]int // the specifier's span when there is exactly one tag
	keep  bool   // asm, static_assert, an empty declaration: never cut
}

type local struct {
	d     *cc.Declaration
	items [][2]int
	dead  []bool
}

type analysis struct {
	opt        Options
	src, blank []byte
	path       string

	ents    []*ent
	byKey   map[string][]*ent
	byName  map[string][]*ent // members, by name
	live    map[string]bool
	named   map[string]bool
	decls   []*decl
	roots   []string
	structs map[string][]*ent // tag or typedef name -> the struct definitions it names
	typedef map[string]string // typedef name -> the tag or anonymous key its specifier names
	locals  []*local
	local   map[int]bool // offsets of identifiers that name a local or a parameter, not a file-scope thing
	st      Stats
	queue   []string
}

var identRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

func pruneOnce(src []byte, path string, opt Options) ([]byte, Stats, error) {
	cfg, err := cc.NewConfig("linux", "amd64")
	if err != nil {
		return nil, Stats{}, err
	}
	ast, err := cc.Parse(cfg, []cc.Source{
		{Name: "<predefined>", Value: cfg.Predefined},
		{Name: "<builtin>", Value: cc.Builtin},
		{Name: path, Value: src},
	})
	if err != nil {
		return nil, Stats{}, fmt.Errorf("parse: %w", err)
	}
	a := &analysis{
		src: src, blank: edit.Blank(src), path: path,
		byKey: map[string][]*ent{}, byName: map[string][]*ent{},
		live: map[string]bool{}, named: map[string]bool{},
		structs: map[string][]*ent{}, typedef: map[string]string{},
	}
	a.opt = opt
	a.collect(ast)
	a.enumValues()
	a.positional(ast)
	a.close()
	a.unusedLocals(ast)
	return a.cut()
}

// ---- collecting ------------------------------------------------------------

func (a *analysis) inFile(n cc.Node) bool {
	return n != nil && !reflect.ValueOf(n).IsNil() && n.Position().Filename == a.path
}

func (a *analysis) add(e *ent) *ent {
	a.ents = append(a.ents, e)
	a.byKey[e.key] = append(a.byKey[e.key], e)
	if e.kind == 'M' && e.name != "" {
		a.byName[e.name] = append(a.byName[e.name], e)
	}
	return e
}

func (a *analysis) collect(ast *cc.AST) {
	a.locals_(ast)
	for _, r := range a.opt.Roots {
		a.roots = append(a.roots, "o:"+r)
	}
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || !a.inFile(ed) {
			continue
		}
		lo, hi := a.span(ed)
		d := &decl{span: [2]int{lo, hi}}
		a.decls = append(a.decls, d)
		switch {
		case ed.FunctionDefinition != nil:
			fd := ed.FunctionDefinition
			e := a.add(&ent{kind: 'F', name: fd.Declarator.Name(), key: "o:" + fd.Declarator.Name(), span: d.span})
			e.refs = a.scan(lo, hi, nil)
			d.fn = e
			d.fd = fd
		case ed.Declaration != nil && ed.Declaration.Case == cc.DeclarationDecl:
			a.declaration(ed.Declaration, d)
		case ed.Declaration != nil && ed.Declaration.StaticAssertDeclaration != nil:
			a.roots = append(a.roots, a.scan(lo, hi, nil)...)
			d.keep = true
		default:
			d.keep = true
		}
	}
}

// declaration makes the entities of one file-scope declaration.
func (a *analysis) declaration(x *cc.Declaration, d *decl) {
	specs := x.DeclarationSpecifiers
	slo, shi := a.span(specs)
	tags, bodies := a.tagsIn(specs, nil)
	d.tags = tags
	if len(tags) == 1 {
		d.tagAt = [2]int{slo, shi}
	}
	specRefs := a.scan(slo, shi, bodies)
	for _, t := range tags {
		if strings.HasPrefix(t.key, "a:") {
			specRefs = append(specRefs, t.key)
		}
	}
	isTypedef := hasWord(a.blank[slo:shi], "typedef", bodies, slo)
	for l := x.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
		id := l.InitDeclarator
		if id == nil || id.Declarator == nil {
			continue
		}
		lo, hi := a.span(id)
		nm := id.Declarator.Name()
		kind := byte('O')
		switch {
		case isTypedef:
			kind = 'T'
		case a.isFuncDeclarator(id.Declarator):
			kind = 'P'
		}
		e := a.add(&ent{kind: kind, name: nm, key: "o:" + nm, span: [2]int{lo, hi}})
		e.refs = append(append([]string{}, specRefs...), a.scan(lo, hi, nil)...)
		d.items = append(d.items, e)
		if isTypedef {
			for _, t := range tags {
				a.typedef[nm] = t.key
			}
			if len(tags) == 0 {
				if tn := typeName(specs); tn != "" {
					a.typedef[nm] = "o:" + tn
				}
			}
		}
	}
}

// isFuncDeclarator says the declarator's name is followed by a parameter list.
func (a *analysis) isFuncDeclarator(d *cc.Declarator) bool {
	p := d.DirectDeclarator
	for p != nil {
		switch p.Case {
		case cc.DirectDeclaratorFuncParam, cc.DirectDeclaratorFuncIdent:
			return p.DirectDeclarator != nil && p.DirectDeclarator.Case == cc.DirectDeclaratorIdent
		case cc.DirectDeclaratorDecl:
			return false
		}
		p = p.DirectDeclarator
	}
	return false
}

// typeName is the typedef name a specifier list uses, if any.
func typeName(n cc.Node) (out string) {
	walk(n, func(m cc.Node) bool {
		if ts, ok := m.(*cc.TypeSpecifier); ok && ts.Case == cc.TypeSpecifierTypeName {
			out = ts.Token.SrcStr()
		}
		_, isS := m.(*cc.StructOrUnionSpecifier)
		_, isE := m.(*cc.EnumSpecifier)
		return !isS && !isE
	})
	return out
}

// tagsIn finds the outermost struct, union and enum specifiers under n that
// define or declare a tag, makes their entities, and returns the bodies a
// scan of n's own references must skip.  parent is the member a definition
// nested in a struct belongs to.
func (a *analysis) tagsIn(n cc.Node, parent *ent) (tags []*ent, bodies [][2]int) {
	walk(n, func(m cc.Node) bool {
		switch x := m.(type) {
		case *cc.StructOrUnionSpecifier:
			if x.Case != cc.StructOrUnionSpecifierDef {
				return false
			}
			tags = append(tags, a.structDef(x, parent))
			bodies = append(bodies, [2]int{x.Token2.Position().Offset, x.Token3.Position().Offset + 1})
			return false
		case *cc.EnumSpecifier:
			if x.Case != cc.EnumSpecifierDef {
				return false
			}
			tags = append(tags, a.enumDef(x))
			bodies = append(bodies, [2]int{x.Token3.Position().Offset, x.Token5.Position().Offset + 1})
			return false
		}
		return true
	})
	if len(tags) == 0 {
		// `struct foo;`, a tag declared and not defined: it lives with the tag.
		walk(n, func(m cc.Node) bool {
			if x, ok := m.(*cc.StructOrUnionSpecifier); ok && x.Token.SrcStr() != "" {
				tags = append(tags, a.add(&ent{kind: 'D', name: x.Token.SrcStr(), key: "t:" + x.Token.SrcStr()}))
			}
			return true
		})
	}
	return tags, bodies
}

func (a *analysis) structDef(x *cc.StructOrUnionSpecifier, parent *ent) *ent {
	tag := x.Token.SrcStr()
	key := "t:" + tag
	if tag == "" {
		key = fmt.Sprintf("a:%d", x.Position().Offset)
	}
	s := a.add(&ent{kind: 'S', name: tag, key: key})
	s.span[0], s.span[1] = a.span(x)
	if tag != "" && parent != nil {
		// A tag defined inside a member: the member cannot go while the tag
		// lives, and the tag keeps the struct it is written in.
		parent.hold = true
		s.refs = append(s.refs, parent.owner.key)
	}
	if tag != "" {
		a.structs[tag] = append(a.structs[tag], s)
	}
	a.structs[key] = append(a.structs[key], s)
	for l := x.StructDeclarationList; l != nil; l = l.StructDeclarationList {
		sd := l.StructDeclaration
		if sd == nil {
			continue
		}
		lo, hi := a.span(sd)
		if sd.StaticAssertDeclaration != nil {
			s.refs = append(s.refs, a.scan(lo, hi, nil)...)
			continue
		}
		var list []*ent
		probe := &ent{owner: s}
		tags, bodies := a.tagsIn(sd.SpecifierQualifierList, probe)
		var nested []*ent
		for _, t := range tags {
			if t.kind == 'S' {
				nested = append(nested, t)
			}
		}
		qlo, qhi := a.span(sd.SpecifierQualifierList)
		specRefs := a.scan(qlo, qhi, bodies)
		for _, t := range tags {
			if strings.HasPrefix(t.key, "a:") {
				specRefs = append(specRefs, t.key)
			}
		}
		for dl := sd.StructDeclaratorList; dl != nil; dl = dl.StructDeclaratorList {
			sdr := dl.StructDeclarator
			if sdr == nil {
				continue
			}
			dlo, dhi := a.span(sdr)
			nm := ""
			if sdr.Declarator != nil {
				nm = sdr.Declarator.Name()
			}
			m := a.add(&ent{kind: 'M', name: nm, key: "m:" + nm, owner: s, span: [2]int{dlo, dhi}, hold: probe.hold || nm == ""})
			m.nested = nested
			m.refs = append(append([]string{}, specRefs...), a.scan(dlo, dhi, nil)...)
			list = append(list, m)
			s.members = append(s.members, m)
		}
		if len(list) == 0 {
			// `struct { ... };` or `union { ... };` as a member: anonymous, and
			// its members are the outer struct's.
			m := a.add(&ent{kind: 'M', key: "m:", owner: s, span: [2]int{lo, hi}, hold: true})
			m.nested = nested
			m.refs = specRefs
			list = append(list, m)
			s.members = append(s.members, m)
		}
		// The whole StructDeclaration is what goes when all its declarators do.
		list[0].declSpan = [2]int{lo, hi}
		s.lists = append(s.lists, list)
	}
	return s
}

func (a *analysis) enumDef(x *cc.EnumSpecifier) *ent {
	tag := x.Token2.SrcStr()
	key := "t:" + tag
	if tag == "" {
		key = fmt.Sprintf("a:%d", x.Position().Offset)
	}
	e := a.add(&ent{kind: 'E', name: tag, key: key})
	e.span[0], e.span[1] = a.span(x)
	if x.EnumTypeSpecifier != nil {
		lo, hi := a.span(x.EnumTypeSpecifier)
		e.refs = a.scan(lo, hi, nil)
	}
	for l := x.EnumeratorList; l != nil; l = l.EnumeratorList {
		n := l.Enumerator
		if n == nil {
			continue
		}
		lo, hi := a.span(n)
		nm := n.Token.SrcStr()
		it := a.add(&ent{kind: 'N', name: nm, key: "o:" + nm, owner: e, span: [2]int{lo, hi},
			explicit: n.Case == cc.EnumeratorExpr, nameEnd: n.Token.Position().Offset + len(nm)})
		it.refs = append([]string{key}, a.scan(lo, hi, nil)...)
		e.members = append(e.members, it)
	}
	return e
}

// ---- references ------------------------------------------------------------

// scan returns the keys the text in [lo, hi) mentions, outside skip.
func (a *analysis) scan(lo, hi int, skip [][2]int) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range identRe.FindAllIndex(a.blank[lo:hi], -1) {
		s, e := m[0]+lo, m[1]+lo
		if s > 0 && isIdent(a.blank[s-1]) {
			continue // the tail of a number
		}
		if inside(s, skip) {
			continue
		}
		ns := a.classify(s)
		if ns == "o:" && a.local[s] {
			continue
		}
		k := ns + string(a.blank[s:e])
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}

// classify is the name space the identifier at p is in, from the token
// before it.
func (a *analysis) classify(p int) string {
	i := p - 1
	for i >= 0 && isSpace(a.blank[i]) {
		i--
	}
	if i < 0 {
		return "o:"
	}
	switch a.blank[i] {
	case '.':
		return "m:"
	case '>':
		if i > 0 && a.blank[i-1] == '-' {
			return "m:"
		}
		return "o:"
	}
	if isIdent(a.blank[i]) {
		j := i
		for j >= 0 && isIdent(a.blank[j]) {
			j--
		}
		switch string(a.blank[j+1 : i+1]) {
		case "struct", "union", "enum":
			return "t:"
		}
	}
	return "o:"
}

func isIdent(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

func inside(p int, skip [][2]int) bool {
	for _, r := range skip {
		if r[0] <= p && p < r[1] {
			return true
		}
	}
	return false
}

func hasWord(b []byte, w string, skip [][2]int, base int) bool {
	for _, m := range identRe.FindAllIndex(b, -1) {
		if string(b[m[0]:m[1]]) == w && !inside(m[0]+base, skip) {
			return true
		}
	}
	return false
}

// ---- the closure -----------------------------------------------------------

func (a *analysis) close() {
	a.queue = append(a.queue, a.roots...)
	a.drain()
	for a.guards() {
		a.drain()
	}
}

func (a *analysis) drain() {
	for len(a.queue) > 0 {
		k := a.queue[len(a.queue)-1]
		a.queue = a.queue[:len(a.queue)-1]
		if strings.HasPrefix(k, "m:") {
			nm := k[2:]
			if a.named[nm] {
				continue
			}
			a.named[nm] = true
			for _, m := range a.byName[nm] {
				if m.owner.live {
					a.activate(m)
				}
			}
			continue
		}
		if a.live[k] {
			continue
		}
		a.live[k] = true
		for _, e := range a.byKey[k] {
			if e.kind != 'M' {
				a.activate(e)
			}
		}
	}
}

func (a *analysis) activate(e *ent) {
	if e.live {
		return
	}
	e.live = true
	a.queue = append(a.queue, e.refs...)
	if e.kind == 'S' {
		for _, m := range e.members {
			if e.pinAll || m.hold || a.named[m.name] {
				a.activate(m)
			}
		}
	}
}

// guards keeps what deleting would change in a way the closure cannot see,
// and says whether it kept anything.
func (a *analysis) guards() bool {
	grew := false
	for _, e := range a.ents {
		if !e.live || len(e.members) == 0 {
			continue
		}
		anyLive := false
		for _, m := range e.members {
			anyLive = anyLive || m.live
		}
		if !anyLive {
			// Never empty a struct or an enum out: that is not C.
			for _, m := range e.members {
				a.activate(m)
			}
			grew = true
			continue
		}
		if e.kind != 'E' {
			continue
		}
		// A survivor after a deleted run is pinned to the value it had; one
		// with no value that can be computed keeps the run before it.
		deleted := false
		var run []*ent
		for _, n := range e.members {
			if !n.live {
				deleted = true
				run = append(run, n)
				continue
			}
			if deleted && !n.explicit && !n.valOK {
				for _, r := range run {
					a.activate(r)
				}
				a.st.KeptRuns += len(run)
				grew = true
			}
			deleted, run = false, nil
		}
	}
	return grew
}

// ---- positional initialisers ------------------------------------------------

// positional marks every struct some brace initialiser fills by position.
func (a *analysis) positional(ast *cc.AST) {
	// While a FreezeLayoutIf function is defined, a struct layout is a format
	// another program reads (vim's ml_recover: swap files another vim wrote),
	// and no member of any struct goes.
	for _, name := range a.opt.FreezeLayoutIf {
		for _, e := range a.byKey["o:"+name] {
			if e.kind != 'F' {
				continue
			}
			for _, s := range a.ents {
				if s.kind == 'S' {
					s.pinAll = true
				}
			}
			return
		}
	}
	var mark func(key string)
	marked := map[*ent]bool{}
	mark = func(key string) {
		for _, s := range a.structsOf(key) {
			if marked[s] {
				continue
			}
			marked[s] = true
			s.pinAll = true
			// Every struct it holds by value is filled by position inside it.
			for _, list := range s.lists {
				dl := list[0].declSpan
				if dl[1] <= dl[0] {
					continue
				}
				for _, r := range a.memberTypes(dl, list) {
					mark(r)
				}
			}
		}
	}
	walk(ast.TranslationUnit, func(n cc.Node) bool {
		switch x := n.(type) {
		case *cc.Declaration:
			if x.Case != cc.DeclarationDecl || !a.inFile(x) {
				return true
			}
			key := a.specKey(x.DeclarationSpecifiers)
			if key == "" {
				return true
			}
			for l := x.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
				id := l.InitDeclarator
				if id == nil || id.Initializer == nil || id.Initializer.InitializerList == nil {
					continue
				}
				if byPosition(id.Initializer, arrayDepth(id.Declarator)) {
					mark(key)
				}
			}
		case *cc.PostfixExpression:
			if x.Case != cc.PostfixExpressionComplit || !a.inFile(x) || x.TypeName == nil {
				return true
			}
			key := a.specKey(x.TypeName.SpecifierQualifierList)
			if key != "" && byPosition(&cc.Initializer{InitializerList: x.InitializerList}, 0) {
				mark(key)
			}
		}
		return true
	})
	a.st.Positional = len(marked)
}

// memberTypes is what the members of one StructDeclaration are, when they
// are structs held by value.
func (a *analysis) memberTypes(span [2]int, list []*ent) []string {
	pointer := true
	for _, m := range list {
		if !bytes.Contains(a.blank[m.span[0]:m.span[1]], []byte("*")) {
			pointer = false
		}
	}
	if pointer {
		return nil
	}
	var out []string
	for _, k := range a.scan(span[0], span[1], nil) {
		if strings.HasPrefix(k, "t:") || strings.HasPrefix(k, "o:") || strings.HasPrefix(k, "a:") {
			out = append(out, k)
		}
	}
	// An anonymous struct written inline is filled by position too.
	for _, s := range list[0].nested {
		if s.name == "" {
			out = append(out, s.key)
		}
	}
	return out
}

// structsOf resolves a key -- a tag, a typedef name, an anonymous definition --
// to the struct definitions it names.
func (a *analysis) structsOf(key string) []*ent {
	for i := 0; i < 8; i++ {
		switch {
		case strings.HasPrefix(key, "t:"):
			return a.structs[key[2:]]
		case strings.HasPrefix(key, "a:"):
			return a.structs[key]
		case strings.HasPrefix(key, "o:"):
			next, ok := a.typedef[key[2:]]
			if !ok {
				return nil
			}
			key = next
			continue
		}
		return nil
	}
	return nil
}

// specKey is what a specifier list names as its type: a tag, a typedef name,
// or an anonymous definition written in it.
func (a *analysis) specKey(n cc.Node) (key string) {
	walk(n, func(m cc.Node) bool {
		switch x := m.(type) {
		case *cc.TypeSpecifier:
			if x.Case == cc.TypeSpecifierTypeName {
				key = "o:" + x.Token.SrcStr()
			}
		case *cc.StructOrUnionSpecifier:
			if x.Token.SrcStr() != "" {
				key = "t:" + x.Token.SrcStr()
			} else {
				key = fmt.Sprintf("a:%d", x.Position().Offset)
			}
			return false
		case *cc.EnumSpecifier:
			return false
		}
		return true
	})
	return key
}

func arrayDepth(d *cc.Declarator) int {
	n := 0
	for p := d.DirectDeclarator; p != nil; p = p.DirectDeclarator {
		switch p.Case {
		case cc.DirectDeclaratorArr, cc.DirectDeclaratorStaticArr, cc.DirectDeclaratorArrStatic, cc.DirectDeclaratorStar:
			n++
		}
	}
	return n
}

// byPosition says a brace initialiser, below its array levels, has an
// element that names no member.
func byPosition(in *cc.Initializer, depth int) bool {
	if in == nil || in.InitializerList == nil {
		return false
	}
	for l := in.InitializerList; l != nil; l = l.InitializerList {
		if depth > 0 {
			if byPosition(l.Initializer, depth-1) {
				return true
			}
			continue
		}
		if l.Designation == nil {
			return true
		}
	}
	return false
}

// ---- enumerator values -----------------------------------------------------

// enumValues computes every enumerator's value from the text, in file order,
// where it can: an implicit one is the one before plus one, an explicit one
// is its expression, evaluated.
func (a *analysis) enumValues() {
	env := map[string]int64{}
	for _, e := range a.ents {
		if e.kind != 'E' {
			continue
		}
		var prev int64 = -1
		ok := true
		for _, n := range e.members {
			if n.explicit {
				eq := bytes.IndexByte(a.src[n.nameEnd:n.span[1]], '=')
				v, good := evalConst(string(a.src[n.nameEnd+eq+1:n.span[1]]), env)
				prev, ok = v, good
			} else {
				prev++
			}
			n.val, n.valOK = prev, ok
			if ok {
				env[n.name] = prev
			}
		}
	}
}

// ---- unused locals ----------------------------------------------------------

var effectRe = regexp.MustCompile(`[A-Za-z_]\w*\s*\(|[^=!<>]=[^=]|\+\+|--`)

func (a *analysis) unusedLocals(ast *cc.AST) {
	for _, d := range a.decls {
		if d.fn == nil || !d.fn.live {
			continue
		}
		a.localsIn(ast, d)
	}
}

func (a *analysis) localsIn(ast *cc.AST, d *decl) {
	fd := d.fd
	if fd == nil {
		return
	}
	// A use is charged to the declaration the parser's scopes resolve it to:
	// two locals of one name in two blocks are two locals.  A use that
	// resolves to nothing keeps every local of its name.
	type use struct {
		s    *cc.Scope
		name string
	}
	uses := map[use]int{}
	unresolved := map[string]bool{}
	walk(fd, func(n cc.Node) bool {
		if x, ok := n.(*cc.PrimaryExpression); ok && x.Case == cc.PrimaryExpressionIdent {
			nm := x.Token.SrcStr()
			if s := x.LexicalScope().Declares(x.Token); s != nil {
				uses[use{s, nm}]++
			} else {
				unresolved[nm] = true
			}
		}
		return true
	})
	walk(fd.CompoundStatement, func(n cc.Node) bool {
		bi, ok := n.(*cc.BlockItem)
		if !ok || bi.Declaration == nil || bi.Declaration.Case != cc.DeclarationDecl {
			return true
		}
		x := bi.Declaration
		if lo, hi := a.span(x); hi <= lo {
			return true // the parser's own `__func__`, which is not in the file
		}
		slo, shi := a.span(x.DeclarationSpecifiers)
		spec := a.blank[slo:shi]
		if hasWord(spec, "typedef", nil, 0) || hasWord(spec, "extern", nil, 0) || bytes.IndexByte(spec, '{') >= 0 {
			return true
		}
		l := &local{d: x}
		anyDead := false
		for il := x.InitDeclaratorList; il != nil; il = il.InitDeclaratorList {
			id := il.InitDeclarator
			if id == nil || id.Declarator == nil {
				continue
			}
			lo, hi := a.span(id)
			nm := id.Declarator.Name()
			dead := uses[use{id.Declarator.LexicalScope(), nm}] == 0 && !unresolved[nm]
			if dead && id.Initializer != nil {
				ilo, ihi := a.span(id.Initializer)
				if effectRe.Match(append(append([]byte{' '}, a.blank[ilo:ihi]...), ' ')) &&
					!onlySizeof(a.blank[ilo:ihi]) {
					a.st.ActingLocals++
				}
			}
			l.items = append(l.items, [2]int{lo, hi})
			l.dead = append(l.dead, dead)
			anyDead = anyDead || dead
		}
		if anyDead {
			a.locals = append(a.locals, l)
		}
		return true
	})
}

var callRe = regexp.MustCompile(`([A-Za-z_]\w*)\s*\(`)

// onlySizeof says every call-shaped thing in an initialiser is sizeof, and
// there is no assignment: `size_t n = sizeof(x)` does nothing.
func onlySizeof(b []byte) bool {
	for _, m := range callRe.FindAllSubmatch(b, -1) {
		if string(m[1]) != "sizeof" {
			return false
		}
	}
	return !regexp.MustCompile(`[^=!<>]=[^=]|\+\+|--`).Match(append(append([]byte{' '}, b...), ' '))
}

// ---- cutting ---------------------------------------------------------------

type cutEdit struct {
	a, b int
	with string
}

func (a *analysis) cut() ([]byte, Stats, error) {
	var edits []cutEdit
	del := func(lo, hi int) { edits = append(edits, cutEdit{lo, hi, ""}) }
	count := func(e *ent) {
		switch e.kind {
		case 'F':
			a.st.Funcs++
		case 'O':
			a.st.Objects++
		case 'P':
			a.st.Protos++
		case 'T':
			a.st.Typedefs++
		case 'S', 'E', 'D':
			a.st.Tags++
		case 'M':
			a.st.Members++
		case 'N':
			a.st.Enumerators++
		}
	}
	var inner func(t *ent)
	inner = func(t *ent) {
		switch t.kind {
		case 'S':
			for _, list := range t.lists {
				alive := 0
				for _, m := range list {
					if m.live {
						alive++
						for _, n := range m.nested {
							if n.live {
								inner(n)
							}
						}
					} else {
						count(m)
					}
				}
				if alive == 0 {
					del(list[0].declSpan[0], list[0].declSpan[1])
					continue
				}
				edits = append(edits, listCut(list, func(e *ent) bool { return !e.live })...)
			}
		case 'E':
			for _, n := range t.members {
				if !n.live {
					count(n)
				}
			}
			edits = append(edits, listCut(t.members, func(e *ent) bool { return !e.live })...)
			deleted := false
			for _, n := range t.members {
				if !n.live {
					deleted = true
					continue
				}
				if deleted && !n.explicit {
					edits = append(edits, cutEdit{n.nameEnd, n.nameEnd, " = " + formatValue(n.val)})
					a.st.Pinned++
				}
				deleted = false
			}
		}
	}
	for _, d := range a.decls {
		if d.keep {
			continue
		}
		if d.fn != nil {
			if !d.fn.live {
				count(d.fn)
				del(d.span[0], d.span[1])
			}
			continue
		}
		liveItems, liveTags := 0, 0
		for _, e := range d.items {
			if e.live {
				liveItems++
			}
		}
		for _, t := range d.tags {
			if t.live || (t.kind == 'D' && a.live[t.key]) {
				liveTags++
			}
		}
		if liveItems == 0 && liveTags == 0 {
			if len(d.items) == 0 && len(d.tags) == 0 {
				continue
			}
			for _, e := range d.items {
				count(e)
			}
			for _, t := range d.tags {
				count(t)
			}
			del(d.span[0], d.span[1])
			continue
		}
		for _, t := range d.tags {
			if t.live {
				inner(t)
			}
		}
		if len(d.items) == 0 {
			continue
		}
		if liveItems == 0 {
			// The tag it defines lives and nothing it declares does: what is
			// left is the definition alone, `struct foo { ... };`.
			for _, e := range d.items {
				count(e)
			}
			t := d.tags[0]
			del(d.span[0], t.span[0])
			del(t.span[1], d.span[1]-1)
			continue
		}
		edits = append(edits, listCut(d.items, func(e *ent) bool { return !e.live })...)
		for _, e := range d.items {
			if !e.live {
				count(e)
			}
		}
	}
	for _, l := range a.locals {
		all := true
		for i, dead := range l.dead {
			if dead {
				a.st.Locals++
			}
			all = all && l.dead[i]
		}
		if all {
			lo, hi := a.span(l.d)
			del(lo, hi)
			continue
		}
		items := make([]*ent, len(l.items))
		for i, s := range l.items {
			items[i] = &ent{span: s, live: !l.dead[i]}
		}
		edits = append(edits, listCut(items, func(e *ent) bool { return !e.live })...)
	}

	sort.Slice(edits, func(i, j int) bool {
		if edits[i].a != edits[j].a {
			return edits[i].a > edits[j].a
		}
		return edits[i].b > edits[j].b
	})
	out := append([]byte{}, a.src...)
	last := len(out) + 1
	for _, e := range edits {
		if e.b > last {
			return nil, a.st, fmt.Errorf("overlapping cuts at %d..%d and %d", e.a, e.b, last)
		}
		with := e.with
		if with == "" && e.a > 0 && e.b < len(out) && isIdent(out[e.a-1]) && isIdent(out[e.b]) {
			with = " "
		}
		out = append(out[:e.a], append([]byte(with), out[e.b:]...)...)
		last = e.a
	}
	a.st.Lines = bytes.Count(a.src, []byte{'\n'}) - bytes.Count(out, []byte{'\n'})
	return out, a.st, nil
}

// listCut deletes the dead items of a comma-separated list that keeps at
// least one: a leading dead run up to the first survivor, every other dead
// item with the comma before it.
func listCut(items []*ent, dead func(*ent) bool) []cutEdit {
	var out []cutEdit
	first := 0
	for first < len(items) && dead(items[first]) {
		first++
	}
	if first == len(items) {
		return nil
	}
	if first > 0 {
		out = append(out, cutEdit{items[0].span[0], items[first].span[0], ""})
	}
	for i := first + 1; i < len(items); i++ {
		if dead(items[i]) {
			out = append(out, cutEdit{items[i-1].span[1], items[i].span[1], ""})
		}
	}
	return out
}

// formatValue spells a pinned value as deadenums did: decimal, hexadecimal
// from 65536 up.
func formatValue(v int64) string {
	if v >= 65536 {
		return fmt.Sprintf("0x%x", v)
	}
	return fmt.Sprintf("%d", v)
}

// ---- the tree --------------------------------------------------------------

// span is a node's byte range in the file: its first in-file token to the end
// of its last.
func (a *analysis) span(n cc.Node) (int, int) { return Span(n, a.path, a.src) }

// Span is n's byte range in src, the file parsed as path: its first token in
// that file to the end of its last.  0, 0 when it has none there.
func Span(n cc.Node, path string, src []byte) (int, int) {
	lo, hi := 1<<62, -1
	walkTok(n, func(t cc.Token) {
		p := t.Position()
		if p.Filename != path {
			return
		}
		// The parser writes a synthetic `static const char __func__[] = "name";`
		// into every body, at the position of its `{`: a token there that is
		// not the brace is not in the file.  (Comparing every token with the text
		// is wrong: `bool` is spelled `_Bool` by the front end.)
		if p.Offset < len(src) && src[p.Offset] == '{' && t.SrcStr() != "{" {
			return
		}
		if p.Offset < lo {
			lo = p.Offset
		}
		if e := p.Offset + len(t.SrcStr()); e > hi {
			hi = e
		}
	})
	if hi < 0 {
		return 0, 0
	}
	return lo, hi
}

// fields is, per node type, which fields a walk visits: the exported ones
// holding a node or a token.  Asking reflect for them at every node was most of
// a sweep's time; they are a fact about the type.
var fields sync.Map // reflect.Type -> []fieldAt

type fieldAt struct {
	i     int
	token bool
}

var (
	nodeType  = reflect.TypeOf((*cc.Node)(nil)).Elem()
	tokenType = reflect.TypeOf(cc.Token{})
)

func fieldsOf(t reflect.Type) []fieldAt {
	if f, ok := fields.Load(t); ok {
		return f.([]fieldAt)
	}
	var out []fieldAt
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		switch {
		case sf.Type == tokenType:
			out = append(out, fieldAt{i, true})
		case sf.Type.Implements(nodeType), sf.Type.Kind() == reflect.Interface:
			// a node, or an interface one is stored in (ExpressionNode)
			out = append(out, fieldAt{i, false})
		}
	}
	fields.Store(t, out)
	return out
}

// walkTok calls f on every token under n, in field order.
func walkTok(n cc.Node, f func(cc.Token)) {
	v := reflect.ValueOf(n)
	if n == nil || v.Kind() != reflect.Ptr || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return
	}
	e := v.Elem()
	for _, fa := range fieldsOf(e.Type()) {
		fv := e.Field(fa.i)
		if fa.token {
			f(fv.Interface().(cc.Token))
			continue
		}
		if x, ok := fv.Interface().(cc.Node); ok {
			walkTok(x, f)
		}
	}
}

// walk calls f on every node under n, n included, in source order, and does
// not descend below a node f returns false for.
func walk(n cc.Node, f func(cc.Node) bool) {
	v := reflect.ValueOf(n)
	if n == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return
	}
	if !f(n) {
		return
	}
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return
	}
	e := v.Elem()
	for _, fa := range fieldsOf(e.Type()) {
		if fa.token {
			continue
		}
		if x, ok := e.Field(fa.i).Interface().(cc.Node); ok {
			walk(x, f)
		}
	}
}

// locals_ records every identifier that names something below file scope: a
// use the parser's scopes resolve to a local or a parameter, and the name
// written in a declarator that is not at file scope.  A reference is by name,
// and these are the names that are not the file-scope thing they spell.
func (a *analysis) locals_(ast *cc.AST) {
	a.local = map[int]bool{}
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || !a.inFile(ed) {
			continue
		}
		walk(ed, func(n cc.Node) bool {
			switch x := n.(type) {
			case *cc.PrimaryExpression:
				if x.Case == cc.PrimaryExpressionIdent {
					if s := x.LexicalScope().Declares(x.Token); s != nil && s != ast.Scope {
						a.local[x.Token.Position().Offset] = true
					}
				}
			case *cc.StructDeclarator:
				if x.Declarator != nil {
					if t, ok := declName(x.Declarator); ok {
						a.local[t.Position().Offset] = true
					}
				}
			case *cc.ParameterDeclaration:
				if x.Declarator != nil {
					if t, ok := declName(x.Declarator); ok {
						a.local[t.Position().Offset] = true
					}
				}
			case *cc.Declarator:
				if x.LexicalScope() != ast.Scope {
					if t, ok := declName(x); ok {
						a.local[t.Position().Offset] = true
					}
				}
			}
			return true
		})
	}
}

// declName is the identifier a declarator declares.
func declName(d *cc.Declarator) (cc.Token, bool) {
	for p := d.DirectDeclarator; p != nil; {
		switch p.Case {
		case cc.DirectDeclaratorIdent:
			return p.Token, true
		case cc.DirectDeclaratorDecl:
			if p.Declarator == nil {
				return cc.Token{}, false
			}
			return declName(p.Declarator)
		}
		p = p.DirectDeclarator
	}
	return cc.Token{}, false
}

// Walk calls f on every node under n, n included, in source order, and does
// not descend below a node f returns false for.
func Walk(n cc.Node, f func(cc.Node) bool) { walk(n, f) }

// PositionalMembers is the name of every member of every struct or union
// some brace initializer fills by position -- directly, or held by value
// inside one that is (the sweep's own rule for which members it may not
// delete).  A phase that retypes a member asks it: a value written by
// position is one no assignment shows.
func PositionalMembers(ast *cc.AST, path string, src []byte, opt Options) map[string]bool {
	a := &analysis{
		opt: opt,
		src: src, blank: edit.Blank(src), path: path,
		byKey: map[string][]*ent{}, byName: map[string][]*ent{},
		live: map[string]bool{}, named: map[string]bool{},
		structs: map[string][]*ent{}, typedef: map[string]string{},
	}
	a.collect(ast)
	a.positional(ast)
	out := map[string]bool{}
	for _, e := range a.ents {
		if e.kind == 'S' && e.pinAll {
			for _, m := range e.members {
				out[m.name] = true
			}
		}
	}
	return out
}
