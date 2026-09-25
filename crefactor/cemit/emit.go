// Package cemit prints C23 from crefactor/cc's syntax tree, in one canonical
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
	"bytes"
	"fmt"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
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
// THE CANONICAL FORM HAS NO COMMENTS.  Canonical strips every one before the
// parse (stripComments), so neither the tree nor a recovered macro span ever
// holds one: whole-line, trailing, block, inside a brace or inside a macro
// invocation, they all go.
func File(ast *cc.AST, mainFile string, src []byte) ([]byte, error) {
	e := &emitter{src: src, lineAt: lineIndex(src)}
	incl, inclAt := includes(src)
	written := false
	first := true

	sep := func() {
		if !first {
			e.w("\n")
		}
		first = false
	}
	flushIncludes := func() {
		if written || len(incl) == 0 {
			return
		}
		written = true
		sep()
		for _, s := range incl {
			e.w(s + "\n")
		}
	}

	for l := ast.TranslationUnit; l != nil; l = l.TranslationUnit {
		d := l.ExternalDeclaration
		if d == nil || d.Case == cc.ExternalDeclarationEmpty {
			continue
		}
		pos := d.Position()
		if pos.Filename != mainFile {
			continue // a predeclared builtin, or something a header brought in
		}
		if pos.Line > inclAt {
			flushIncludes()
		}
		e.exp = e.scan(d)
		e.emitted = map[int]bool{}
		// A DECLARATION THAT PRINTS NOTHING GETS NO SEPARATOR.  The front end
		// follows a file-scope `static_assert(...);` with a second declaration
		// of its own, empty, which wrote a second blank line after every one.
		if d.Case == cc.ExternalDeclarationDecl && len(e.declLines(d.Declaration)) == 0 {
			continue
		}
		sep()
		e.external(d)
		if e.err != nil {
			return nil, e.err
		}
	}
	flushIncludes()
	return []byte(e.b.String()), nil
}

// stripComments blanks every comment in src -- `//` to the end of the line,
// `/* ... */` wherever it is -- outside string and character literals, and
// keeps every newline, so each token keeps its line and column and a recovered
// macro span keeps its extent (collapse folds the blanks).  A `//` that ends in
// a backslash continues onto the next line, as the preprocessor reads it.
func stripComments(src []byte) []byte {
	out := make([]byte, len(src))
	copy(out, src)
	blank := func(i int) {
		if out[i] != '\n' {
			out[i] = ' '
		}
	}
	for i := 0; i < len(out); {
		switch c := out[i]; {
		case c == '"' || c == '\'':
			i++
			for i < len(out) && out[i] != c && out[i] != '\n' {
				if out[i] == '\\' {
					i++
				}
				i++
			}
			i++
		case c == '/' && i+1 < len(out) && out[i+1] == '/':
			for i < len(out) && out[i] != '\n' {
				if out[i] == '\\' && i+1 < len(out) && out[i+1] == '\n' {
					blank(i)
					i += 2
					continue
				}
				blank(i)
				i++
			}
		case c == '/' && i+1 < len(out) && out[i+1] == '*':
			blank(i)
			blank(i + 1)
			i += 2
			for i < len(out) && !(out[i] == '*' && i+1 < len(out) && out[i+1] == '/') {
				blank(i)
				i++
			}
			if i < len(out) {
				blank(i)
				blank(i + 1)
				i += 2
			}
		default:
			i++
		}
	}
	return out
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
			out = append(out, strings.TrimRight(line, " \t"))
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
	src = stripComments(src)
	// ONLY #include.  The front end preprocesses: a #define is expanded and gone,
	// an #if resolved to one branch, and the printer prints the tree -- so a
	// directive other than #include would be lost from the output without a
	// word.  It is refused instead.  (vim's text has only #include; a foreign
	// file with macros is not this printer's to canonicalise.)
	for i, line := range bytes.Split(src, []byte{'\n'}) {
		t := bytes.TrimLeft(line, " \t")
		if len(t) > 0 && t[0] == '#' && !bytes.HasPrefix(bytes.TrimLeft(t[1:], " \t"), []byte("include")) {
			return nil, fmt.Errorf("%s:%d: %s -- a directive other than #include, which printing the parsed tree would drop", path, i+1, bytes.TrimSpace(line))
		}
	}
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
