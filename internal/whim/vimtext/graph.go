package vimtext

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
)

// The shapes above on the graph, as part 49a, part 49b and phase 49 read
// them there: the include forms, and the core's block of ordinary
// declarations.

// IncludeRun is the graph's include forms, refused unless every one is an
// `#include <...>` of a system header and they stand on consecutive forms:
// the first of them is the line between the core and the host, and nothing
// above it is an include.
func IncludeRun(e *graph.Editor) ([]*graph.Node, error) {
	incs := e.Includes()
	if len(incs) == 0 {
		return nil, fmt.Errorf("the file has no include form, so there is no boundary and no way to " +
			"tell a core call from a host one")
	}
	forms := e.Graph().Forms
	at := -1
	for i, f := range forms {
		if f == incs[0] {
			at = i
			break
		}
	}
	for k, inc := range incs {
		if at+k >= len(forms) || forms[at+k] != inc {
			return nil, fmt.Errorf("the %d include forms are not consecutive, so the first is not a boundary",
				len(incs))
		}
		if !strings.HasPrefix(graph.IncludeSpec(inc), "<") {
			return nil, fmt.Errorf("an include is not an `#include <...>` of a system header: %s",
				graph.IncludeSpec(inc))
		}
	}
	return incs, nil
}

// IsOrdinaryDecl is a form of the core's block of ordinary declarations: a
// function's prototype, not `static` -- a libc function the core declares
// for itself.
func IsOrdinaryDecl(f *graph.Node) bool {
	if !f.Is("def") || len(f.Kids) < 3 {
		return false
	}
	for _, k := range f.Kids[1:] {
		if !k.IsList() && (k.Atom == "static" || k.Atom == "extern") {
			return false
		}
	}
	t := graph.DeclType(f)
	return t != nil && t.Is("fn")
}

// FormC is one form's C, its last newline taken off: a declaration's line.
func FormC(f *graph.Node) (string, error) {
	c, err := graph.FormsC([]*graph.Node{f})
	if err != nil {
		return "", err
	}
	return string(bytes.TrimRight(c, "\n")), nil
}

// OrdinaryBlock is the core's block of ordinary declarations: the run of
// consecutive core forms around the one whose C is seed, every one an
// ordinary declaration (IsOrdinaryDecl), with their C.  Refused unless seed
// is the C of exactly one core form.
func OrdinaryBlock(e *graph.Editor, seed string) ([]*graph.Node, []string, error) {
	core := e.Core()
	name := DeclNameRe.ReplaceAllString(seed, "$1")
	at := -1
	for i, f := range core {
		if !f.Is("def") || graph.DeclName(f) != name {
			continue
		}
		c, err := FormC(f)
		if err != nil {
			return nil, nil, err
		}
		if c == seed {
			if at >= 0 {
				at = -2
				break
			}
			at = i
		}
	}
	if at < 0 || !IsOrdinaryDecl(core[at]) {
		return nil, nil, fmt.Errorf("the core does not declare `%s` exactly once, so this phase has not "+
			"been handed the file it was written for", seed)
	}
	lo, hi := at, at
	for lo > 0 && IsOrdinaryDecl(core[lo-1]) {
		lo--
	}
	for hi+1 < len(core) && IsOrdinaryDecl(core[hi+1]) {
		hi++
	}
	block := append([]*graph.Node(nil), core[lo:hi+1]...)
	lines := make([]string, len(block))
	for i, f := range block {
		c, err := FormC(f)
		if err != nil {
			return nil, nil, err
		}
		lines[i] = c
	}
	return block, lines, nil
}

// SplitCore is a C view cut at its first `#include` line, as the text
// phases cut the file: the core above it, the host from it on, and the
// line's index (the core's line count).
func SplitCore(text []byte) (core, host []byte, line int) {
	at := 0
	for at < len(text) {
		if bytes.HasPrefix(text[at:], []byte("#include")) {
			return text[:at], text[at:], bytes.Count(text[:at], []byte{'\n'})
		}
		nl := bytes.IndexByte(text[at:], '\n')
		if nl < 0 {
			break
		}
		at += nl + 1
	}
	return text, nil, bytes.Count(text, []byte{'\n'})
}
