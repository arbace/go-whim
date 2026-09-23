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
func File(ast *cc.AST, mainFile string, src []byte) ([]byte, error) {
	e := &emitter{src: src, lineAt: lineIndex(src)}
	incl, inclAt := includes(src)
	written := false
	first := true

	flushIncludes := func() {
		if written || len(incl) == 0 {
			return
		}
		written = true
		if !first {
			e.w("\n")
		}
		for _, s := range incl {
			e.w(s + "\n")
		}
		first = false
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
		if !first {
			e.w("\n")
		}
		first = false
		e.exp = e.scan(d)
		e.emitted = map[int]bool{}
		e.external(d)
		if e.err != nil {
			return nil, e.err
		}
	}
	flushIncludes()
	return []byte(e.b.String()), nil
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
		if specs := e.declSpecs(f.DeclarationSpecifiers); specs != "" {
			e.w(Indent + specs + "\n")
		}
		e.w(e.declarator(f.Declarator) + "\n")
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
func Canonical(path string, src []byte) ([]byte, error) {
	cfg, err := cc.NewConfig("linux", "amd64")
	if err != nil {
		return nil, err
	}
	ast, err := cc.Translate(cfg, []cc.Source{
		{Name: "<predefined>", Value: cfg.Predefined},
		{Name: "<builtin>", Value: cc.Builtin},
		{Name: path, Value: string(src)},
	})
	if err != nil {
		return nil, err
	}
	return File(ast, path, src)
}
