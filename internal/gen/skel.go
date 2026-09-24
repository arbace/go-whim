package gen

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/arbace/go-whim/internal/cc"
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

// Run is the program, called as `go tool whim <name> ARGS`: args are its
// arguments, and what it used to print on stderr goes to errw.  It returns
// the exit status.
// logw is where the generator's progress lines go: Run's errw.
var logw io.Writer = os.Stderr

func Run(args []string, errw io.Writer) int {
	logw = errw
	osArgs := append([]string{"run"}, args...)
	if len(osArgs) < 3 {
		fmt.Fprintln(errw, "usage: skel <editor.c> <outdir> [-bodies | -editor <editor.go>]")
		return 2
	}
	ast, err := parse(osArgs[1])
	if err != nil {
		fmt.Fprintln(errw, err)
		return 1
	}
	a := &an{u: newUF(), fnDecls: map[string]*cc.Declarator{}, decls: map[*cc.Declarator]string{}, addr: map[string]bool{}}
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		a.walk(tu.ExternalDeclaration)
	}
	g := newGen(ast, a)
	g.collect()
	if err := g.write(osArgs[2]); err != nil {
		fmt.Fprintln(errw, err)
		return 1
	}
	if len(osArgs) > 4 && osArgs[3] == "-editor" {
		if err := g.writeEditor(osArgs[4], crtFuncs); err != nil {
			fmt.Fprintln(errw, "skel:", err)
			return 1
		}
	}
	if len(osArgs) > 3 && osArgs[3] == "-bodies" {
		if err := g.writeBodies(osArgs[2], crtFuncs); err != nil {
			fmt.Fprintln(errw, err)
			return 1
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
	os.WriteFile(osArgs[2]+"/facts.json", b, 0o644)
	fmt.Fprintf(errw, "skel: %d objects, %d cursors, %d statics, %d functions used as values\n",
		len(a.u.parent), len(facts), len(a.statics), len(a.addr))
	return 0
}

// crtFuncs are the C functions editor/crt.go replaces: their calls are
// translated, their bodies are not (internal/gen/CONVENTIONS.md).  ga_grow_inner()'s
// body is the one rule of its own: it grows the storage with GaGrowTo, in
// elements of the storage's type.
var crtFuncs = map[string]bool{"alloc": true, "alloc_clear": true, "lalloc": true, "lalloc_clear": true,
	"musl_memmove": true, "musl_memcpy": true, "musl_memset": true, "musl_memcmp": true,
	"ga_grow_inner": true}
