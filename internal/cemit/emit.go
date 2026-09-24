// Package cemit prints C23 from internal/cc's syntax tree, in one canonical
// form, to a fixpoint.
//
// WHY A PRINTER AND NOT A FORMATTER.  The pipeline's input is macro-expansion
// residue: `{'Z', nv_Zet,  (0x02|NV_NCH) |NV_NCW, 0} ,` is what slim-vim.c
// really contains, and every text tool downstream has to cope with the spacing
// no one chose.  A printer that takes the FORM from the tree and only the leaf
// text from the tokens gives one shape per construct: one statement per line,
// one declarator per declaration, braces always, an operator spaced the same
// way everywhere.  What a regex has to match after that is a much smaller set.
//
// THE FORM IS A FIXPOINT.  Printing text this package produced gives the same
// bytes back, which is the property that lets it run after every sweep rather
// than once: `Emit(Emit(x)) == Emit(x)`, checked rather than asserted.
//
// WHAT IT REFUSES.  A node it does not print is an error naming the node and
// its position, never a silent omission -- a printer that drops what it does
// not understand would produce a file that still compiles and no longer says
// what it said.
package cemit

import (
	"fmt"
	"strings"

	"github.com/arbace/go-whim/internal/cc"
)

// An emitter prints one translation unit.
type emitter struct {
	b      strings.Builder
	indent int
	err    error

	src    []byte      // the source, for macro-invocation recovery
	lineAt []int       // the offset of each line of it
	exp    *expansions // what the declaration being printed expanded

	// emitted is the expansions already printed in this declaration, so that a
	// macro whose replacement list spans more than one node is printed once.
	emitted map[int]bool
}

// Indent is the canonical indent: four spaces per level, which is what the
// input already uses everywhere a human wrote it.
const Indent = "    "

func (e *emitter) fail(n cc.Node, format string, a ...any) {
	if e.err != nil {
		return
	}
	where := ""
	if n != nil {
		where = n.Position().String() + ": "
	}
	e.err = fmt.Errorf("cemit: %s%s", where, fmt.Sprintf(format, a...))
}

func (e *emitter) w(s string) {
	if e.err == nil {
		e.b.WriteString(s)
	}
}

// line writes one whole line at the current indent.
func (e *emitter) line(s string) {
	if s == "" {
		e.w("\n")
		return
	}
	e.w(strings.Repeat(Indent, e.indent) + s + "\n")
}

// tok is a token's own text.  Identifiers, constants and string literals are
// never re-derived: what the source said is what is printed.
func tok(t cc.Token) string { return t.SrcStr() }

// File prints the canonical form of one translation unit.
//
// THE INCLUDES ARE NOT IN THE TREE.  The front end expands them, so a printer
// that walked the tree alone would inline forty-one system headers and lose the
// line that separates this pipeline's core from its host.  They are read from
// the source as text and written back where they were: everything the source
// declared above them, then the include lines verbatim, then the rest.
// AND NEITHER ARE THE COMMENTS.  The front end throws them away, and
// slim-vim.c's 272 whole-line comments are its map -- `// ==== alloc.c ====`,
// and the two banners that delimit the generated ex_cmdidxs.h block, which a
// phase reads and then removes.  They are read from the source as text too, and
// written back between the declarations they stood between.
func File(ast *cc.AST, mainFile string, src []byte) ([]byte, error) {
	e := &emitter{src: src, lineAt: lineIndex(src)}
	incl, inclAt := includes(src)
	runs, err := commentRuns(src)
	if err != nil {
		return nil, err
	}
	ri := 0
	written := false
	first := true

	sep := func() {
		if !first {
			e.w("\n")
		}
		first = false
	}
	// comments before a line: every run that stood above it in the source.
	comments := func(before int) {
		for ri < len(runs) && runs[ri].line < before {
			sep()
			for _, s := range runs[ri].text {
				e.w(s + "\n")
			}
			ri++
		}
	}
	flushIncludes := func() {
		if written || len(incl) == 0 {
			return
		}
		written = true
		comments(inclAt)
		sep()
		for _, s := range incl {
			e.w(s + "\n")
		}
	}

	for l := ast.TranslationUnit; l != nil; l = l.TranslationUnit {
		d := l.ExternalDeclaration
		if d == nil {
			continue
		}
		pos := d.Position()
		if pos.Filename != mainFile {
			continue // a predeclared builtin, or something a header brought in
		}
		if pos.Line > inclAt {
			flushIncludes()
		}
		comments(pos.Line)
		sep()
		e.exp = e.scan(d)
		e.emitted = map[int]bool{}
		e.external(d)
		if e.err != nil {
			return nil, e.err
		}
	}
	flushIncludes()
	comments(1 << 30)
	return []byte(e.b.String()), nil
}

// A run is consecutive whole-line comments and the line the first of them is on.
type run struct {
	line int
	text []string
}

// commentRuns finds the source's whole-line comments, OUTSIDE ANY BRACE: a
// comment inside a function body has no declaration to be written above, and
// this refuses rather than move it.  Measured on slim-vim.c: 272 comments, all
// of them at column 0 and at depth 0, and no `/* */` at all.
func commentRuns(src []byte) ([]run, error) {
	depth := 0
	line := 1
	atLineStart := true
	var runs []run
	last := -2 // the line number of the last comment taken
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '\n':
			line++
			atLineStart = true
			i++
			continue
		case c == ' ' || c == '\t':
			i++
			continue // indentation does not end the start of a line
		case c == '"' || c == '\'':
			q := c
			i++
			for i < len(src) && src[i] != q {
				if src[i] == '\\' {
					i++
				}
				if i < len(src) && src[i] == '\n' {
					line++
				}
				i++
			}
			i++
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			j := i
			for j < len(src) && src[j] != '\n' {
				j++
			}
			if !atLineStart {
				return nil, fmt.Errorf("cemit: line %d: a comment that is not a whole line", line)
			}
			if depth != 0 {
				return nil, fmt.Errorf("cemit: line %d: a whole-line comment inside a brace", line)
			}
			text := strings.TrimRight(string(src[i:j]), " \t")
			if line == last+1 {
				runs[len(runs)-1].text = append(runs[len(runs)-1].text, text)
			} else {
				runs = append(runs, run{line: line, text: []string{text}})
			}
			last = line
			i = j
			continue
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			return nil, fmt.Errorf("cemit: line %d: a block comment", line)
		case c == '{':
			depth++
			i++
		case c == '}':
			depth--
			i++
		default:
			i++
		}
		if c != '\n' {
			atLineStart = false
		}
	}
	return runs, nil
}

// external writes one external declaration.
func (e *emitter) external(n *cc.ExternalDeclaration) {
	switch n.Case {
	case cc.ExternalDeclarationDecl:
		for _, s := range e.declLines(n.Declaration) {
			e.line(s)
		}
	case cc.ExternalDeclarationFuncDef:
		f := n.FunctionDefinition
		if f.DeclarationList != nil {
			e.fail(n, "an old-style parameter declaration list")
			return
		}
		// THE NAME GOES AT COLUMN 0, under its specifiers, which is vim's own
		// style and the only thing that makes a definition findable by text:
		// every tool that locates one looks for `^name(`.  Measured, when this
		// printed `static void f(...)` on one line instead: 88 of the 163
		// phases lost their function and said so -- "is not defined at file
		// scope any more".
		// AND THE POINTER GOES WITH THE SPECIFIERS, for the same reason:
		// vim writes `static char_u *` and then `ml_get_buf(...)` at column
		// 0, and a `*` left at the head of the declarator would put the name
		// at column 1 where `^name(` cannot see it.
		specs := e.declSpecs(f.DeclarationSpecifiers)
		ptr := e.pointer(f.Declarator.Pointer)
		if head := joinNonEmpty(specs, ptr); head != "" {
			e.w(Indent + head + "\n")
		}
		e.w(e.directDeclarator(f.Declarator.DirectDeclarator) + "\n")
		e.compound(f.CompoundStatement)
	case cc.ExternalDeclarationAsmStmt:
		e.line(strings.TrimSpace(cc.NodeSource(n.AsmStatement)))
	case cc.ExternalDeclarationEmpty:
		// `;` at file scope declares nothing and is not printed.
	default:
		e.fail(n, "external declaration %v", n.Case)
	}
}

func joinNonEmpty(parts ...string) string { return strings.Join(nonEmpty(parts), " ") }

// includes returns the source's `#include` lines and the line the first one is
// on.  They are the only directives the input has: slim-vim.c has 41 and
// nothing else, whim-vim.c has 12 (measured).
func includes(src []byte) ([]string, int) {
	var out []string
	at := 1 << 30
	for i, line := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(line, "#include") {
			out = append(out, line)
			if at == 1<<30 {
				at = i + 1
			}
		}
	}
	return out, at
}

// Canonical parses a translation unit and prints its canonical form.  The
// front end needs a path because a diagnostic without one names nothing.
//
// PARSE AND NOT TRANSLATE, because this printer reads SYNTAX and never a type:
// nothing in this package asks the front end for a `.Type()`, a `.Value()` or a
// resolved field, and `enum : long` survives because the printer takes it from
// the EnumSpecifier rather than from what the enum resolves to.  Measured:
// byte-identical output on whim-vim.c and on editor.c either way.
//
// The reason to do it is what it makes POSSIBLE, not what it saves.  The type
// check refuses a text that is momentarily inconsistent -- a struct member
// removed while a use of it survives in a function that is about to be swept --
// which is exactly the shape of the text BETWEEN a phase's edit and its sweep.
// Parsing prints that text and reaches a fixpoint on a second pass, so
// canonicalisation is available there; translating refuses it.
func Canonical(path string, src []byte) ([]byte, error) {
	cfg, err := cc.NewConfig("linux", "amd64")
	if err != nil {
		return nil, err
	}
	ast, err := cc.Parse(cfg, []cc.Source{
		{Name: "<predefined>", Value: cfg.Predefined},
		{Name: "<builtin>", Value: cc.Builtin},
		{Name: path, Value: string(src)},
	})
	if err != nil {
		return nil, err
	}
	return File(ast, path, src)
}
