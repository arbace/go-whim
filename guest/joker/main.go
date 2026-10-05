// The Joker guest: a REPL on the console.  Forms are read from the
// monitor's input -- the wait-and-read hypercall, as the Go guest reads its
// keys -- each evaluated and its value printed, an error printed and the
// REPL going on; the input's end ends the guest.  With an argument, the
// guest evaluates it and exits instead.
//
// The graph views in Joker are loaded first (./gview: gview.graph,
// gview.view, gview.main), and the namespace box is the monitor's store
// (./box; WHIM_GUEST_STORE=DIR, written by `go tool whim store`): so
// (def st (gview.main/open-string (box/load "graph"))) reads and indexes
// the graph whose EDN the ref graph names, in the box, and
// (gview.main/show st "callers ml_find_line") prints what `go tool whim view callers ml_find_line` prints.
package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/arbace/go-whim/guest/abi"
	"github.com/arbace/go-whim/guest/joker/box"
	"github.com/arbace/go-whim/guest/joker/gview"
	"github.com/arbace/go-whim/guest/tamago/board"
	. "github.com/candid82/joker/core"

	// the standard namespaces that need no system
	_ "github.com/candid82/joker/std/base64"
	_ "github.com/candid82/joker/std/crypto"
	_ "github.com/candid82/joker/std/csv"
	_ "github.com/candid82/joker/std/hex"
	_ "github.com/candid82/joker/std/hiccup"
	_ "github.com/candid82/joker/std/html"
	_ "github.com/candid82/joker/std/json"
	_ "github.com/candid82/joker/std/math"
	_ "github.com/candid82/joker/std/mime"
	_ "github.com/candid82/joker/std/strconv"
	_ "github.com/candid82/joker/std/string"
	_ "github.com/candid82/joker/std/time"
	_ "github.com/candid82/joker/std/url"
	_ "github.com/candid82/joker/std/uuid"
)

// console is the monitor's input: WaitRead, waiting for ever.
type console struct{ buf [256]byte }

func (c *console) Read(p []byte) (int, error) {
	b := board.Call(abi.WaitRead, -1, board.Addr(c.buf[:]), int64(len(c.buf)), 0, 0)
	n := int(b.A[0])
	if b.Ret == 0 || n <= 0 {
		return 0, io.EOF
	}
	return copy(p, c.buf[:n]), nil
}

func main() {
	args := board.Args()
	GLOBAL_ENV.InitEnv(Stdin, Stdout, Stderr, nil)
	RT.GIL.Lock()
	ProcessCoreData()
	GLOBAL_ENV.ReferCoreToUser()
	if err := gview.Load(); err != nil {
		fmt.Println("gview:", err)
	}
	box.Install(callStore{})
	if len(args) > 1 {
		code := int32(0)
		if err := ProcessReader(NewReader(strings.NewReader(args[1]), "<expr>"), "", EVAL); err != nil {
			code = 1
		}
		board.Exit(code)
	}
	reader := NewReader(bufio.NewReader(&console{}), "<repl>")
	pc := &ParseContext{GlobalEnv: GLOBAL_ENV}
	rc := newHistory() // *1, *2, *3 and *e
	fmt.Println("joker in the box")
	for !form(reader, pc, rc) {
	}
	board.Exit(0)
}

// form reads, parses, evaluates and prints one form; true at the input's
// end.
func form(reader *Reader, pc *ParseContext, rc *history) (done bool) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(Object); ok {
				rc.exc.Value = e
			}
			fmt.Println("error:", r)
		}
	}()
	fmt.Print("user=> ")
	obj, err := TryRead(reader)
	if err == io.EOF {
		return true
	}
	if err != nil {
		fmt.Println("error:", err)
		return false
	}
	res := Evaluate(Parse(obj, pc))
	rc.push(res)
	PrintObject(res, Stdout)
	fmt.Println()
	return false
}

// history is the REPL's *1, *2, *3 and *e, joker.core's vars, as Joker's own
// REPL keeps them (its main package's ReplContext).
type history struct{ first, second, third, exc *Var }

func newHistory() *history {
	v := func(name string) *Var {
		x, _ := GLOBAL_ENV.Resolve(MakeSymbol("joker.core/" + name))
		x.Value = NIL
		return x
	}
	return &history{v("*1"), v("*2"), v("*3"), v("*e")}
}

func (h *history) push(o Object) {
	h.third.Value, h.second.Value, h.first.Value = h.second.Value, h.first.Value, o
}
