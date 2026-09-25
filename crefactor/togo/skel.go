package togo

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/arbace/go-whim/crefactor/cc"
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

func Run(args []string, errw io.Writer, prof Profile) int {
	p := prof.sets()
	logw = errw
	osArgs := append([]string{"run"}, args...)
	if len(osArgs) < 3 {
		fmt.Fprintln(errw, "usage: skel <editor.c> <outdir> [-bodies | -editor <editor.go> | -java <Class.java> | -clj <editor.clj> | -lowerc <lowered.c>]")
		return 2
	}
	ast, err := parse(osArgs[1])
	if err != nil {
		fmt.Fprintln(errw, err)
		return 1
	}
	a := &an{u: newUF(), fnDecls: map[string]*cc.Declarator{}, decls: map[*cc.Declarator]string{}, addr: map[string]bool{}, frees: p.frees}
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		a.walk(tu.ExternalDeclaration)
	}
	g := newGen(ast, a, p)
	g.collect()
	if err := g.write(osArgs[2]); err != nil {
		fmt.Fprintln(errw, err)
		return 1
	}
	if len(osArgs) > 4 && osArgs[3] == "-editor" {
		if err := g.writeEditor(osArgs[4], p.runtime); err != nil {
			fmt.Fprintln(errw, "skel:", err)
			return 1
		}
	}
	if len(osArgs) > 4 && osArgs[3] == "-java" {
		if err := g.writeJava(osArgs[4]); err != nil {
			fmt.Fprintln(errw, "skel:", err)
			return 1
		}
	}
	if len(osArgs) > 4 && osArgs[3] == "-lowerc" {
		if err := g.writeLoweredC(osArgs[1], osArgs[4]); err != nil {
			fmt.Fprintln(errw, "skel:", err)
			return 1
		}
	}
	if len(osArgs) > 3 && osArgs[3] == "-bodies" {
		if err := g.writeBodies(osArgs[2], p.runtime); err != nil {
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
