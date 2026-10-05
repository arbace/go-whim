package graph

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/arbace/go-whim/crefactor/cc"
)

// THE HEADERS' NAMES (doc/GRAPH-MIGRATION.md, B2e).  The graph holds an
// `#include` as a form, `(include "<stdio.h>")`, and what the headers
// declare that the file uses as external nodes, typed from cc at import.
// Neither says WHICH header declares a name, and a graph read back from its
// Lisp has no cc behind it.  So the include edits ask the headers
// themselves: a Header is what one `#include` line provides, parsed by cc
// on its own (the same linux/amd64 configuration the importer parses the
// file with) -- the ordinary names declared at file scope (functions,
// objects, typedefs, enumerators), the tags, and the macros -- less what a
// translation unit with no include at all has (the compiler's predefined
// and builtin names, which no header has to provide).  A header is parsed
// once a process.

// A Header is what one include line provides.
type Header struct {
	// Spec is the include's operand as written: `<stdio.h>`.
	Spec string
	// Names are the ordinary identifiers it declares at file scope (C's
	// one name space for functions, objects, typedefs and enumerators), and
	// its tags as `struct NAME`, `union NAME`, `enum NAME`.
	Names map[string]bool
	// Macros are the macros it defines.
	Macros map[string]bool
	// Bare are its object-like macros whose replacement names nothing (no
	// identifier in it): a constant, which the importer gives no edge.
	Bare map[string]bool
}

// Provides says h declares or defines name: an ordinary name, a tag
// (`struct NAME`) or a macro.
func (h *Header) Provides(name string) bool { return h.Names[name] || h.Macros[name] }

var headers struct {
	sync.Mutex
	cfg   *cc.Config
	none  *Header // a unit with no include: the compiler's own
	byKey map[string]*Header
	errs  map[string]error
}

// probeName is the one declaration a probe unit adds, since a translation
// unit may not be empty.
const probeName = "graph_header_probe_"

// HeaderOf is what `#include SPEC` provides, SPEC as written (`<stdio.h>`
// or `"x.h"`), parsed by cc once a process.
func HeaderOf(spec string) (*Header, error) {
	headers.Lock()
	defer headers.Unlock()
	if err := initHeaders(); err != nil {
		return nil, err
	}
	if h := headers.byKey[spec]; h != nil {
		return h, nil
	}
	if err := headers.errs[spec]; err != nil {
		return nil, err
	}
	h, err := probe(headers.cfg, spec)
	if err != nil {
		err = fmt.Errorf("#include %s: %v", spec, err)
		headers.errs[spec] = err
		return nil, err
	}
	for n := range headers.none.Names {
		delete(h.Names, n)
	}
	for n := range headers.none.Macros {
		delete(h.Macros, n)
		delete(h.Bare, n)
	}
	headers.byKey[spec] = h
	return h, nil
}

// compilerProvides says name needs no header: the compiler declares or
// defines it in a unit with no include, or it is a builtin cc knows by its
// name alone.
func compilerProvides(name string) bool {
	if strings.HasPrefix(name, "__builtin_") {
		return true
	}
	headers.Lock()
	defer headers.Unlock()
	return initHeaders() == nil && headers.none.Provides(name)
}

// initHeaders makes the configuration and the unit with no include, once;
// headers is locked.
func initHeaders() error {
	if headers.byKey != nil {
		return nil
	}
	cfg, err := ccConfig()
	if err != nil {
		return err
	}
	none, err := probe(cfg, "")
	if err != nil {
		return fmt.Errorf("a unit with no include: %v", err)
	}
	headers.cfg, headers.none = cfg, none
	headers.byKey, headers.errs = map[string]*Header{}, map[string]error{}
	return nil
}

// probe parses a unit of the one include line (none for spec "") and a
// declaration, and lists what its file scope declares and its macros.
func probe(cfg *cc.Config, spec string) (*Header, error) {
	src := "typedef int " + probeName + ";\n"
	if spec != "" {
		src = "#include " + spec + "\n" + src
	}
	ast, err := cc.Parse(cfg, []cc.Source{
		{Name: "<predefined>", Value: cfg.Predefined},
		{Name: "<builtin>", Value: cc.Builtin},
		{Name: "graph-header-probe.c", Value: src},
	})
	if err != nil {
		return nil, err
	}
	h := &Header{Spec: spec, Names: map[string]bool{}, Macros: map[string]bool{}, Bare: map[string]bool{}}
	for name, ns := range ast.Scope.Nodes {
		if name == probeName {
			continue
		}
		for _, n := range ns {
			switch x := n.(type) {
			case *cc.Declarator, *cc.Enumerator:
				h.Names[name] = true
			case *cc.StructOrUnionSpecifier:
				kw := "struct"
				if x.StructOrUnion != nil && x.StructOrUnion.Case == cc.StructOrUnionUnion {
					kw = "union"
				}
				h.Names[kw+" "+name] = true
			case *cc.EnumSpecifier:
				h.Names["enum "+name] = true
			}
		}
	}
	for name, m := range ast.Macros {
		h.Macros[name] = true
		if !m.IsFnLike && !slices.ContainsFunc(m.ReplacementList(), func(t cc.Token) bool { return t.Ch == rune(cc.IDENTIFIER) }) {
			h.Bare[name] = true
		}
	}
	return h, nil
}
