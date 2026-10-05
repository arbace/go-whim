// The Joker guest (doc/LISP-SANDBOX.md): Joker, a Clojure dialect in Go, on
// TamaGo on the Go guest's board -- a REPL in the box.  A module of its own,
// as guest/tamago is, built only by TamaGo's toolchain (go tool whim guest
// --joker); Joker is ./joker, a fork (joker/README.md).
module github.com/arbace/go-whim/guest/joker

go 1.27.1

require (
	github.com/arbace/go-whim v0.0.0
	github.com/arbace/go-whim/guest/tamago v0.0.0
	github.com/candid82/joker v0.0.0
	// GOOSPKG, the runtime's goos overlay: imported by no package
	github.com/usbarmory/tamago v1.27.1
)

replace (
	github.com/arbace/go-whim => ../..
	github.com/arbace/go-whim/crefactor => ../../crefactor
	github.com/arbace/go-whim/guest/tamago => ../tamago
	github.com/candid82/joker => ./joker
)
