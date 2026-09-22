// Command pre checks editor.c for what an automatic transpilation to Go needs
// to be able to assume (internal/ccx), and reports every place it cannot.
//
//	pre casts <editor.c>     every pointer cast, by what it converts
//	pre order <editor.c>     unsequenced operands whose effects collide
//	pre garrays <editor.c>   every growarray, by its element type
//	pre unions <editor.c>    every union member access, by its discriminant
//	pre voids <editor.c>     every declaration that names a void *
//	                         (CCX_GUARDS=1 prints what holds at each leftover)
package main

import (
	"fmt"
	"os"

	"github.com/arbace/go-whim/internal/ccx"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: pre casts|order|unions|garrays|voids <editor.c>")
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
	case "voids":
		r = ccx.VoidPtrs(ast)
	case "garrays":
		r = ccx.GrowArrays(ast)
	case "unions":
		r = ccx.Unions(ast)
	default:
		fmt.Fprintln(os.Stderr, "pre: no check", os.Args[1])
		os.Exit(2)
	}
	if !r.Print(os.Stdout) {
		os.Exit(1)
	}
}
