package main

import (
	"encoding/json"
	"fmt"
	"os"

	"modernc.org/cc/v4"
)

func parse(path string) (*cc.AST, error) {
	cfg, err := cc.NewConfig("linux", "amd64")
	if err != nil {
		return nil, err
	}
	return cc.Translate(cfg, []cc.Source{
		{Name: "<predefined>", Value: cfg.Predefined},
		{Name: "<builtin>", Value: cc.Builtin},
		{Name: path},
	})
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: skel <editor.c> <outdir> [-bodies | -editor <editor.go>]")
		os.Exit(2)
	}
	ast, err := parse(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	a := &an{u: newUF(), fnDecls: map[string]*cc.Declarator{}, decls: map[*cc.Declarator]string{}, addr: map[string]bool{}}
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		a.walk(tu.ExternalDeclaration)
	}
	g := newGen(ast, a)
	g.collect()
	if err := g.write(os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(os.Args) > 4 && os.Args[3] == "-editor" {
		if err := g.writeEditor(os.Args[4], crtFuncs); err != nil {
			fmt.Fprintln(os.Stderr, "skel:", err)
			os.Exit(1)
		}
	}
	if len(os.Args) > 3 && os.Args[3] == "-bodies" {
		if err := g.writeBodies(os.Args[2], crtFuncs); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	// the facts, for review
	facts := map[string]string{}
	for k := range a.u.parent {
		if ok, why := a.u.cursor(k); ok {
			facts[k] = "cursor: " + why
		}
	}
	for k, why := range a.u.pun {
		facts["PUN "+k] = why
	}
	b, _ := json.MarshalIndent(facts, "", " ")
	os.WriteFile(os.Args[2]+"/facts.json", b, 0o644)
	fmt.Fprintf(os.Stderr, "skel: %d objects, %d cursors, %d statics, %d functions used as values\n",
		len(a.u.parent), len(facts), len(a.statics), len(a.addr))
}

// crtFuncs are the C functions editor/crt.go replaces: their calls are
// translated, their bodies are not (tx/CONVENTIONS.md).  ga_grow_inner()'s
// body is the one rule of its own: it grows the storage with GaGrowTo, in
// elements of the storage's type.
var crtFuncs = map[string]bool{"alloc": true, "alloc_clear": true, "lalloc": true, "lalloc_clear": true,
	"musl_memmove": true, "musl_memcpy": true, "musl_memset": true, "musl_memcmp": true,
	"ga_grow_inner": true}
