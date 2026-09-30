package main

// cdiff is the first difference between two C files, entity by entity:
// every top-level function, object, type, tag and assertion keyed by what
// it declares, its text compared -- the tool the pipeline's reform measures
// a candidate product against the committed one with
// (doc/PIPELINE-REFORM.md, §7).
//
//	whim cdiff [-n N] A.c B.c
//
// It prints the entities that differ (the first lines that differ in
// each, up to N of them), the ones only in A or only in B, and whether the
// ones in both come in the same order; and exits 0 only when the two files
// are the same entity for entity.

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/crefactor/cc"
)

// centity is one top-level entity: its key and its text.
type centity struct {
	key, text string
}

func runCdiff(args []string) int {
	n := 10
	if len(args) >= 2 && args[0] == "-n" {
		v, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, "usage: whim cdiff [-n N] A.c B.c")
			return 2
		}
		n, args = v, args[2:]
	}
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: whim cdiff [-n N] A.c B.c")
		return 2
	}
	a, err := centities(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim cdiff: %v\n", err)
		return 1
	}
	b, err := centities(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "whim cdiff: %v\n", err)
		return 1
	}
	byKey := func(es []centity) map[string]string {
		m := map[string]string{}
		for _, e := range es {
			m[e.key] = e.text
		}
		return m
	}
	am, bm := byKey(a), byKey(b)
	var differ, onlyA, onlyB []string
	var orderA, orderB []string
	for _, e := range a {
		t, ok := bm[e.key]
		switch {
		case !ok:
			onlyA = append(onlyA, e.key)
		case t != e.text:
			differ = append(differ, e.key)
		}
		if ok {
			orderA = append(orderA, e.key)
		}
	}
	for _, e := range b {
		if _, ok := am[e.key]; !ok {
			onlyB = append(onlyB, e.key)
		} else {
			orderB = append(orderB, e.key)
		}
	}
	same := len(a) - len(onlyA) - len(differ)
	fmt.Printf("  %s: %d entities, %s: %d; %d the same, %d differ, %d only in the first, %d only in the second\n",
		args[0], len(a), args[1], len(b), same, len(differ), len(onlyA), len(onlyB))
	reordered := strings.Join(orderA, "\x00") != strings.Join(orderB, "\x00")
	if reordered {
		for i := range orderA {
			if orderA[i] != orderB[i] {
				fmt.Printf("  order differs first at %s (the second has %s there)\n", orderA[i], orderB[i])
				break
			}
		}
	}
	show := func(title string, keys []string) {
		if len(keys) == 0 {
			return
		}
		fmt.Printf("  %s:\n", title)
		for i, k := range keys {
			if i == n {
				fmt.Printf("    ... and %d more\n", len(keys)-n)
				break
			}
			fmt.Printf("    %s\n", k)
		}
	}
	show("only in the first", onlyA)
	show("only in the second", onlyB)
	for i, k := range differ {
		if i == n {
			fmt.Printf("  ... and %d more that differ\n", len(differ)-n)
			break
		}
		al, bl := strings.Split(am[k], "\n"), strings.Split(bm[k], "\n")
		j := 0
		for j < len(al) && j < len(bl) && al[j] == bl[j] {
			j++
		}
		fmt.Printf("  %s differs at its line %d:\n", k, j+1)
		for d := j; d < j+3; d++ {
			if d < len(al) {
				fmt.Printf("    - %s\n", al[d])
			}
		}
		for d := j; d < j+3; d++ {
			if d < len(bl) {
				fmt.Printf("    + %s\n", bl[d])
			}
		}
	}
	if len(differ) > 0 || len(onlyA) > 0 || len(onlyB) > 0 || reordered {
		return 1
	}
	return 0
}

// centities is a file's top-level entities in order, each keyed by what it
// declares, its text running to the next one's start.
func centities(path string) ([]centity, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg, err := cc.NewConfig("linux", "amd64")
	if err != nil {
		return nil, err
	}
	ast, err := cc.Parse(cfg, []cc.Source{
		{Name: "<predefined>", Value: cfg.Predefined},
		{Name: "<builtin>", Value: cc.Builtin},
		{Name: path, Value: src},
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	type at struct {
		key string
		off int
	}
	var starts []at
	seen := map[string]int{}
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.Position().Filename != path {
			continue
		}
		off := lineStart(src, ed.Position().Offset)
		key := entityKey(ed)
		if key == "" {
			// a tag, an enum or a static_assert: named by its text, up to
			// its body
			line := string(src[off:])
			if i := strings.IndexAny(line, "{;"); i >= 0 {
				line = line[:i]
			}
			key = "decl " + strings.Join(strings.Fields(line), " ")
		}
		seen[key]++
		if seen[key] > 1 {
			key += "#" + strconv.Itoa(seen[key]) // a prototype, then its definition
		}
		starts = append(starts, at{key, off})
	}
	var out []centity
	for i, s := range starts {
		end := len(src)
		if i+1 < len(starts) {
			end = starts[i+1].off
		}
		out = append(out, centity{s.key, strings.TrimRight(string(src[s.off:end]), "\n ")})
	}
	return out, nil
}

// lineStart is the start of the line holding off: a definition's return
// type is on the line before its name, which the canonical print indents.
func lineStart(src []byte, off int) int {
	for off > 0 && src[off-1] != '\n' {
		off--
	}
	return off
}

// entityKey names what a top-level entity declares.
func entityKey(ed *cc.ExternalDeclaration) string {
	if fd := ed.FunctionDefinition; fd != nil {
		return "func " + fd.Declarator.Name()
	}
	d := ed.Declaration
	if d == nil {
		return fmt.Sprintf("entity@%d", ed.Position().Line)
	}
	var names []string
	for l := d.InitDeclaratorList; l != nil; l = l.InitDeclaratorList {
		if l.InitDeclarator != nil && l.InitDeclarator.Declarator != nil {
			names = append(names, l.InitDeclarator.Declarator.Name())
		}
	}
	if len(names) > 0 {
		return "decl " + strings.Join(names, ",")
	}
	return ""
}
