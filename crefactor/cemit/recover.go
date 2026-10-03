package cemit

import (
	"github.com/arbace/go-whim/crefactor/cc"
)

// THE PRINTER'S DECISIONS THAT ARE NOT IN THE TREE, exported for a converter
// that walks the same tree as File and must come out with the same text
// (crefactor/clisp, C as s-expressions).  Everything else this package prints
// is a function of the syntax alone, which such a converter can follow case by
// case; the macro-invocation recovery and the include lines are a function of
// the source text as well, and are given here rather than written twice.

// Parse is Canonical's front half: comments blanked, any directive but
// #include refused, the text parsed (linux/amd64, the predefined and builtin
// sources first).  It returns the tree and the blanked source, which is what
// File and NewMacros are handed.
func Parse(path string, src []byte) (*cc.AST, []byte, error) {
	src = stripComments(src)
	if err := onlyIncludes(path, src); err != nil {
		return nil, nil, err
	}
	cfg, err := cc.NewConfig("linux", "amd64")
	if err != nil {
		return nil, nil, err
	}
	ast, err := cc.Parse(cfg, []cc.Source{
		{Name: "<predefined>", Value: cfg.Predefined},
		{Name: "<builtin>", Value: cc.Builtin},
		{Name: path, Value: string(src)},
	})
	if err != nil {
		return nil, nil, err
	}
	return ast, src, nil
}

// Includes is the source's `#include` lines as File writes them, and the line
// the first one is on: File writes them all together, before the first
// external declaration of the main file that starts below that line.
func Includes(src []byte) ([]string, int) { return includes(src) }

// FuncName reports whether a block-scope declaration is the `__func__` the
// front end synthesises, which File does not print.
func FuncName(n *cc.Declaration) bool { return funcName(n) }

// Macros is File's macro-invocation recovery for one translation unit.  Enter
// starts an external declaration; the queries then answer exactly as File's
// printer does at the same node, provided they are asked in the order File
// asks them -- Member records what it has printed, as File does.
type Macros struct{ e emitter }

// NewMacros is the recovery over src, the blanked source Parse returned.
func NewMacros(src []byte) *Macros {
	return &Macros{e: emitter{src: src, lineAt: lineIndex(src)}}
}

// Enter scans one external declaration, as File does before printing it.
func (m *Macros) Enter(n *cc.ExternalDeclaration) {
	m.e.exp = m.e.scan(n)
	m.e.emitted = map[int]bool{}
}

// Node is the invocation a whole node came from: an expression, a
// declaration or a statement printed as text.
func (m *Macros) Node(n cc.Node) (string, bool) { return m.e.fromMacro(n) }

// Stmt is the invocation a whole statement came from, with the source's own
// semicolon after it (stmtMacro's text, which ends in `;`).
func (m *Macros) Stmt(n cc.Node) (string, bool) { return m.e.stmtMacro(n) }

// SpecToken is a type-specifier keyword as the source spelled it.
func (m *Macros) SpecToken(t cc.Token) string { return m.e.specToken(t) }

// Member is a member selection's name as File prints it: the token, or the
// text of a member-designator macro the selection straddles.  drop reports
// that the macro was printed by an inner selection already, so this one
// prints its operand alone.
func (m *Macros) Member(t cc.Token) (name string, drop bool) {
	off, isMacro := m.e.atExpansion(t)
	if !isMacro {
		return tok(t), false
	}
	if m.e.emitted[off] {
		return "", true
	}
	text, ok := m.e.exp.text(off)
	if !ok {
		return tok(t), false
	}
	m.e.emitted[off] = true
	return text, false
}
