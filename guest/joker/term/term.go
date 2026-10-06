// Package term is the console in Joker, the namespace term: the keys and
// the screen a modal editor needs (../gview/vi.joke), on the same input the
// REPL reads -- the guest's console, the host runner's stdin.
//
//	(term/key)    the next key: a character as a string, "<esc>", "<up>",
//	              "<down>", "<left>", "<right>"; nil at the input's end
//	(term/size)   [rows cols]
//	(term/start)  the terminal raw (the host's TermStart), and a newline
//	              left after the form that called it skipped
//	(term/stop)   the terminal as it was
//	(term/write s)  s, as it is
package term

import (
	"bufio"
	"fmt"
	"io"

	. "github.com/candid82/joker/core"
)

// A Terminal is the terminal's host: its size and its raw mode.
type Terminal interface {
	Size() (rows, cols int, ok bool)
	Start()
	Stop()
}

// Install interns the namespace term's functions: keys from in, which the
// REPL reads forms from too, and t the terminal.  Joker's environment must
// be initialised and its lock held.
func Install(in *bufio.Reader, t Terminal) {
	ns := GLOBAL_ENV.EnsureSymbolIsNamespace(MakeSymbol("term"))
	def := func(name string, fn func(args []Object) Object) {
		ns.Intern(MakeSymbol(name)).Value = &Proc{Fn: fn, Name: "term/" + name}
	}
	def("key", func(args []Object) Object {
		CheckArity(args, 0, 0)
		r, _, err := in.ReadRune()
		if err != nil {
			return NIL
		}
		if r != 0x1b {
			return MakeString(string(r))
		}
		// an arrow is ESC [ A-D, and arrives in one read; ESC alone is Esc
		if in.Buffered() >= 2 {
			if b, _ := in.Peek(2); b[0] == '[' && b[1] >= 'A' && b[1] <= 'D' {
				in.Discard(2)
				return MakeString([]string{"<up>", "<down>", "<right>", "<left>"}[b[1]-'A'])
			}
		}
		return MakeString("<esc>")
	})
	def("size", func(args []Object) Object {
		CheckArity(args, 0, 0)
		rows, cols, ok := t.Size()
		if !ok || rows < 2 || cols < 10 {
			rows, cols = 24, 80
		}
		return NewVectorFrom(MakeInt(rows), MakeInt(cols))
	})
	def("start", func(args []Object) Object {
		CheckArity(args, 0, 0)
		if b, err := in.Peek(1); err == nil && in.Buffered() > 0 && b[0] == '\n' {
			in.Discard(1)
		}
		t.Start()
		return NIL
	})
	def("stop", func(args []Object) Object {
		CheckArity(args, 0, 0)
		t.Stop()
		return NIL
	})
	def("write", func(args []Object) Object {
		CheckArity(args, 1, 1)
		_, out, _ := GLOBAL_ENV.StdIO()
		w, ok := out.(io.Writer)
		if !ok {
			w = Stdout
		}
		fmt.Fprint(w, EnsureArgIsString(args, 0).S)
		if f, ok := out.(interface{ Flush() error }); ok {
			f.Flush()
		}
		return NIL
	})
}
