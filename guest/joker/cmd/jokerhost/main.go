// Command jokerhost is the Joker guest's language on the host: the fork's
// core and the standard namespaces the guest links (../../main.go), built
// by the ordinary go command -- no TamaGo, no board -- so that what runs in
// the box can be run and tested beside it (doc/LISP-SANDBOX.md, *The views
// in Joker, in the box*).
//
//	jokerhost [--gview] [--store DIR] [-e EXPR | FILE.joke]...
//	jokerhost view [view-clj's arguments]
//
// The first evaluates each expression and file in order, printing the last
// expression's value; with none, it is a REPL on stdin, as the guest's.
// --gview loads the views' namespaces (../../gview: gview.graph,
// gview.view, gview.main), and --store makes the namespace box (../../box)
// the store in DIR, as vmm.DirStore keeps it for the guest's monitor
// (WHIM_GUEST_STORE): box/load "graph" the EDN the guest reads.  The second is
// crefactor/graph/view/clj/view-clj in Joker: the views loaded and
// (gview.main/run ARGS) called, its value the exit status.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime/pprof"
	"strings"

	"github.com/arbace/go-whim/guest/joker/box"
	"github.com/arbace/go-whim/guest/joker/ed"
	"github.com/arbace/go-whim/guest/joker/gview"
	"github.com/arbace/go-whim/guest/joker/rt"
	"github.com/arbace/go-whim/vmm"
	. "github.com/candid82/joker/core"

	// the standard namespaces the guest links
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

const usage = "usage: jokerhost [--gview] [--store DIR] [-e EXPR | FILE.joke]...\n       jokerhost view [view-clj's arguments]"

func main() {
	if f := os.Getenv("JOKERHOST_CPUPROFILE"); f != "" {
		w, err := os.Create(f)
		if err == nil && pprof.StartCPUProfile(w) == nil {
			defer pprof.StopCPUProfile()
		}
	}
	args := os.Args[1:]
	view := len(args) > 0 && args[0] == "view"
	if view {
		args = args[1:]
	}
	GLOBAL_ENV.InitEnv(Stdin, Stdout, Stderr, args)
	RT.GIL.Lock()
	ProcessCoreData()
	GLOBAL_ENV.ReferCoreToUser()
	rt.Install()
	if view {
		load()
		v, err := eval("(gview.main/run (vec *command-line-args*))", "<view>")
		flush()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			exit(1)
		}
		code := 0
		if i, ok := v.(Int); ok {
			code = i.I
		}
		exit(code)
	}
	ran := false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--gview":
			load()
		case a == "--store" && i+1 < len(args):
			i++
			s, err := vmm.OpenDirStore(args[i])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				exit(1)
			}
			box.Install(s)
			ed.Install(s)
		case a == "-e" && i+1 < len(args):
			i++
			ran = true
			v, err := eval(args[i], "<expr>")
			flush()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				exit(1)
			}
			if _, ok := v.(Nil); !ok && i == len(args)-1 {
				PrintObject(v, Stdout)
				fmt.Println()
			}
		case strings.HasPrefix(a, "-"):
			fmt.Fprintln(os.Stderr, usage)
			exit(2)
		default:
			ran = true
			b, err := os.ReadFile(a)
			if err == nil {
				err = ProcessReader(NewReader(strings.NewReader(string(b)), a), a, EVAL)
			}
			flush()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				exit(1)
			}
		}
	}
	if !ran {
		repl()
	}
	flush()
}

func load() {
	if err := gview.Load(); err != nil {
		fmt.Fprintln(os.Stderr, "gview:", err)
		exit(1)
	}
}

// eval reads and evaluates every form of src, and returns the last value.
func eval(src, name string) (v Object, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	reader := NewReader(strings.NewReader(src), name)
	pc := &ParseContext{GlobalEnv: GLOBAL_ENV}
	v = NIL
	for {
		obj, rerr := TryRead(reader)
		if rerr == io.EOF {
			return v, nil
		}
		if rerr != nil {
			return nil, rerr
		}
		v = Evaluate(Parse(obj, pc))
	}
}

// flush flushes Joker's *out*, which buffers.
func flush() {
	_, out, _ := GLOBAL_ENV.StdIO()
	if f, ok := out.(interface{ Flush() error }); ok {
		f.Flush()
	}
}

// repl reads forms from stdin, each evaluated and its value printed.
func repl() {
	reader := NewReader(bufio.NewReader(os.Stdin), "<repl>")
	pc := &ParseContext{GlobalEnv: GLOBAL_ENV}
	for {
		done := func() (done bool) {
			defer func() {
				if r := recover(); r != nil {
					flush()
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
			PrintObject(res, Stdout)
			flush()
			fmt.Println()
			return false
		}()
		if done {
			fmt.Println()
			return
		}
	}
}

// exit stops the CPU profile JOKERHOST_CPUPROFILE asked for, and exits.
func exit(code int) {
	pprof.StopCPUProfile()
	os.Exit(code)
}
