package togo

// hssplit.go is the module a translation unit becomes, or the modules
// (doc/HASKELL-IDIOMS.md, item 8).  Split (Profile.HsParts), the functions
// go into parts by the call graph: its strongly connected components,
// callees first, cut into parts of about equal size, no component divided,
// so that a part imports only parts before it -- the ones it calls into.
// The accessors, the offsets, the patterns and the types are a module of
// their own, Defs, which every part imports; the top module is the segment's
// size, newEditor, initGlobals and the function table, which it hands the
// editor (Caprice.Rt's Ed carries it), so that a call through a pointer
// imports nothing; and the names the host calls back, as the hs-boot
// declares them.  It re-exports the rest.  GHC compiles the parts one at a
// time, or side by side (-j), each a fraction of the whole's memory.

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// hsPrelude are the Prelude's names a module may use: an identifier, or an
// operator (in parentheses).
var hsPrelude = []string{"IO", "Bool (..)", "Eq", "Num", "pure", "fromIntegral", "not", "quot", "rem", "negate",
	"($)", "(+)", "(-)", "(*)", "(==)", "(/=)", "(<)", "(<=)", "(>)", "(>=)", "(&&)", "(||)", "(<$>)", "(>>)", "(!!)", "error"}

// header is a module's head: its pragmas, its name and exports, and the
// imports its body uses, the Prelude's by name.
func (h *hgen) header(mod, exports, body string, extra ...string) string {
	used := map[string]bool{}
	for _, t := range hsTokens(body) {
		used[t] = true
	}
	for _, o := range hsOps(body) {
		used["("+o+")"] = true
	}
	var pre []string
	for _, n := range hsPrelude {
		switch {
		case n == "Bool (..)":
			if used["Bool"] || used["True"] || used["False"] {
				pre = append(pre, n)
			}
		case used[n]:
			pre = append(pre, n)
		}
	}
	var b strings.Builder
	b.WriteString("{-# LANGUAGE BangPatterns, MagicHash, PatternSynonyms #-}\n{-# OPTIONS_GHC -w #-}\n")
	if exports != "" {
		fmt.Fprintf(&b, "module %s\n  ( %s\n  ) where\n\n", mod, exports)
	} else {
		fmt.Fprintf(&b, "module %s where\n\n", mod)
	}
	fmt.Fprintf(&b, "import Prelude (%s)\n", strings.Join(pre, ", "))
	if used["listArray"] {
		b.WriteString("import Data.Array (Array, listArray)\n")
	}
	b.WriteString("import Caprice.Rt\n")
	if strings.Contains(body, h.host+".") {
		fmt.Fprintf(&b, "import qualified %s\n", h.host)
	}
	for _, e := range extra {
		b.WriteString(e + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

// hsOps are the operators in Haskell source, outside its strings, its
// character literals and its comments -- a negative literal's minus not one.
func hsOps(src string) []string {
	var out []string
	const sym = "!#$%&*+./<=>?@\\^|-~:"
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '"':
			i++
			for i < len(src) && src[i] != '"' {
				if src[i] == '\\' {
					i++
				}
				i++
			}
			i++
		case c == '\'' && (i == 0 || !hsIdentChar(src[i-1])):
			i++
			if i < len(src) && src[i] == '\\' {
				i++
			}
			i += 2
		case c == '{' && i+1 < len(src) && src[i+1] == '-':
			if j := strings.Index(src[i:], "-}"); j >= 0 {
				i += j + 2
			} else {
				i = len(src)
			}
		case c == '-' && i+1 < len(src) && src[i+1] == '-':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case hsIdentChar(c):
			for i < len(src) && hsIdentChar(src[i]) {
				i++
			}
		case strings.IndexByte(sym, c) >= 0:
			j := i
			for j < len(src) && strings.IndexByte(sym, src[j]) >= 0 {
				j++
			}
			op := src[i:j]
			if !(op == "-" && hsNegLit(src, i, j)) {
				out = append(out, op)
			}
			i = j
		default:
			i++
		}
	}
	return out
}

// split writes the functions as parts, the definitions as Defs and the top
// module at path (files: a path -> its source).
func (h *hgen) split(path string, fds []*cc.FunctionDefinition, funcs []string, init string, inits []string, table string, files map[string]string) {
	base := strings.TrimSuffix(path, ".hs")
	size := map[string]int{}
	text := map[string]string{}
	total := 0
	for i, fd := range fds {
		n := fd.Declarator.Name()
		text[n] = funcs[i]
		size[n] = strings.Count(funcs[i], "\n") + 1
		total += size[n]
	}
	// the parts: the components, callees first, cut at about equal sizes
	comps := h.components(fds)
	target := (total + h.g.p.HsParts - 1) / h.g.p.HsParts
	part := map[string]int{}
	var parts [][]string
	acc := 0
	for _, c := range comps {
		if len(parts) == 0 || acc >= target && len(parts) < h.g.p.HsParts {
			parts = append(parts, nil)
			acc = 0
		}
		k := len(parts) - 1
		for _, n := range c {
			part[n] = k
			parts[k] = append(parts[k], n)
			acc += size[n]
		}
	}
	// the functions in the unit's order in each part
	order := map[string]int{}
	for i, fd := range fds {
		order[fd.Declarator.Name()] = i
	}
	partMod := func(k int) string { return fmt.Sprintf("%s.Part%d", h.module, k+1) }
	partFile := func(k int) string { return filepath.Join(base, fmt.Sprintf("Part%d.hs", k+1)) }
	defsMod := h.module + ".Defs"
	var all strings.Builder
	all.WriteString(strings.Join(inits, "\n") + table)
	for _, f := range funcs {
		all.WriteString(f + "\n")
	}
	defs := h.definitions(all.String(), false)
	files[filepath.Join(base, "Defs.hs")] = h.header(defsMod, "", defs) + defs
	bootOf := map[int][]string{} // a part -> the names the host calls back from it
	for k, ns := range parts {
		sort.Slice(ns, func(i, j int) bool { return order[ns[i]] < order[ns[j]] })
		deps := map[int]bool{}
		var b strings.Builder
		for _, n := range ns {
			b.WriteString(text[n] + "\n")
			for _, c := range h.fx[n].calls {
				if pk, ok := part[c]; ok && pk != k {
					deps[pk] = true
				}
			}
			if _, ok := h.bootSigs[n]; ok {
				bootOf[k] = append(bootOf[k], h.names[n])
			}
		}
		imports := []string{"import " + defsMod}
		var ds []int
		for d := range deps {
			ds = append(ds, d)
		}
		sort.Ints(ds)
		for _, d := range ds {
			imports = append(imports, "import "+partMod(d))
		}
		files[partFile(k)] = h.header(partMod(k), "", b.String(), imports...) + b.String()
	}
	// the top: the editor made, the table, and what the host calls back
	var top strings.Builder
	top.WriteString(init)
	exps := []string{"module " + h.module, "module " + defsMod}
	var bootAddrs []string
	for _, name := range h.g.p.HsExports {
		if h.defined[name] == nil && h.objs["global:"+name].name == name {
			bootAddrs = append(bootAddrs, "addr'"+name) // Defs's own name is the host's
		}
	}
	imports := []string{hsImportHiding(defsMod, bootAddrs)}
	for _, name := range h.g.p.HsExports {
		if h.defined[name] == nil {
			imports = append(imports, "import qualified "+defsMod+" as Defs")
			break
		}
	}
	for k := range parts {
		exps = append(exps, "module "+partMod(k))
		imports = append(imports, hsImportHiding(partMod(k), bootOf[k]))
		if len(bootOf[k]) > 0 {
			imports = append(imports, fmt.Sprintf("import qualified %s as Part%d", partMod(k), k+1))
		}
	}
	top.WriteString("-- * What the host calls back, as the hs-boot declares it\n\n")
	for _, name := range h.g.p.HsExports {
		if sig, ok := h.bootSigs[name]; ok {
			fmt.Fprintf(&top, "%s\n%s = Part%d.%s\n\n", sig, h.names[name], part[name]+1, h.names[name])
		} else {
			fmt.Fprintf(&top, "addr'%s :: Ed -> Ptr a\naddr'%s = Defs.addr'%s\n\n", name, name, h.objs["global:"+name].name)
		}
	}
	files[path] = h.header(h.module, strings.Join(exps, "\n  , "), top.String(), imports...) + top.String()
}

// hsImportHiding is an import of mod, hiding names.
func hsImportHiding(mod string, names []string) string {
	if len(names) == 0 {
		return "import " + mod
	}
	return "import " + mod + " hiding (" + strings.Join(names, ", ") + ")"
}

// components are the call graph's strongly connected components, callees
// first (Tarjan's order), each in the unit's order.
func (h *hgen) components(fds []*cc.FunctionDefinition) [][]string {
	index := map[string]int{}
	low := map[string]int{}
	on := map[string]bool{}
	var stack []string
	var out [][]string
	next := 0
	var visit func(n string)
	visit = func(n string) {
		index[n], low[n] = next, next
		next++
		stack = append(stack, n)
		on[n] = true
		for _, c := range h.fx[n].calls {
			if h.fx[c] == nil {
				continue
			}
			if _, seen := index[c]; !seen {
				visit(c)
				low[n] = min(low[n], low[c])
			} else if on[c] {
				low[n] = min(low[n], index[c])
			}
		}
		if low[n] == index[n] {
			var comp []string
			for {
				m := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				on[m] = false
				comp = append(comp, m)
				if m == n {
					break
				}
			}
			out = append(out, comp)
		}
	}
	for _, fd := range fds {
		if _, seen := index[fd.Declarator.Name()]; !seen {
			visit(fd.Declarator.Name())
		}
	}
	return out
}

// hsNegLit says the minus at src[i:j] is a negative literal's: before a
// digit, after an opening parenthesis, an = or a comma.
func hsNegLit(src string, i, j int) bool {
	if j >= len(src) || src[j] < '0' || src[j] > '9' {
		return false
	}
	k := i - 1
	for k >= 0 && src[k] == ' ' {
		k--
	}
	return k < 0 || src[k] == '(' || src[k] == '=' || src[k] == ','
}
