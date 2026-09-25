// Command pre checks editor.c for what an automatic transpilation to Go needs
// to be able to assume (internal/ccx), and reports every place it cannot.
//
//	pre casts <editor.c>     every pointer cast, by what it converts
//	pre order <editor.c>     unsequenced operands whose effects collide
//	pre garrays <editor.c>   every growarray, by its element type
//	pre unions <editor.c>    every union member access, by its discriminant
//	pre voids <editor.c>     every declaration that names a void *
//	pre gotos <editor.c>     every goto, by whether it jumps into a block
//	pre funcs <editor.c>     every comparison of function pointers
//	                         (CCX_GUARDS=1 prints what holds at each leftover)
package pre

import (
	"fmt"
	"io"
	"os"

	"github.com/arbace/go-whim/internal/ccx"
	"github.com/arbace/go-whim/internal/whim"
)

// Run is the program, called as `go tool whim <name> ARGS`: args are its
// arguments, and what it used to print on stderr goes to errw.  It returns
// the exit status.
func Run(args []string, errw io.Writer) int {
	osArgs := append([]string{"run"}, args...)
	if len(osArgs) != 3 {
		fmt.Fprintln(errw, "usage: pre casts|order|unions|garrays|voids|gotos|funcs <editor.c>")
		return 2
	}
	ast, err := ccx.Parse(osArgs[2])
	if err != nil {
		fmt.Fprintln(errw, err)
		return 1
	}
	var r ccx.Result
	switch osArgs[1] {
	case "casts":
		r = ccx.Casts(ast, whim.CCX)
	case "order":
		r = ccx.Order(ast, whim.CCX)
	case "funcs":
		r = ccx.FuncCompares(ast)
	case "gotos":
		r = ccx.Gotos(ast)
	case "voids":
		r = ccx.VoidPtrs(ast, whim.CCX)
	case "garrays":
		r = ccx.GrowArrays(ast, whim.CCX)
	case "unions":
		r = ccx.Unions(ast, whim.CCX)
	default:
		fmt.Fprintln(errw, "pre: no check", osArgs[1])
		return 2
	}
	if !r.Print(os.Stdout) {
		return 1
	}
	return 0
}
