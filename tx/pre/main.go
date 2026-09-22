// Command pre checks editor.c for what an automatic transpilation to Go needs
// to be able to assume (internal/ccx), and reports every place it cannot.
//
//	pre casts <editor.c>     every pointer cast, by what it converts
//	pre order <editor.c>     unsequenced operands whose effects collide
package main

import (
	"fmt"
	"os"

	"github.com/arbace/go-whim/internal/ccx"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: pre casts|order <editor.c>")
		os.Exit(2)
	}
	ast, err := ccx.Parse(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var r ccx.Result
	switch os.Args[1] {
	case "casts":
		r = ccx.Casts(ast)
	case "order":
		r = ccx.Order(ast)
	default:
		fmt.Fprintln(os.Stderr, "pre: no check", os.Args[1])
		os.Exit(2)
	}
	if !r.Print(os.Stdout) {
		os.Exit(1)
	}
}
