// Command faulty stands in for the Go editor on the board, to see what the
// guest does where the editor never goes (guest's TestGoGuestFault): a null
// pointer written ("null"), a spin with no call ("spin"), a panic ("panic"),
// and anything else a line on the console and the exit code 7.
package main

import "github.com/arbace/go-whim/guest/tamago/board"

// nowhere is a null pointer the compiler cannot see is one.
var nowhere *int64

func main() {
	args := board.Args()
	what := ""
	if len(args) > 1 {
		what = args[1]
	}
	switch what {
	case "null":
		*nowhere = 1
	case "spin":
		//lint:ignore SA5002 the spin is the point: the watchdog ends it
		for {
		}
	case "panic":
		panic("faulty: " + what)
	}
	println("faulty: hello")
	board.Exit(7)
}
