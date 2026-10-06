// The Joker guest (doc/LISP-SANDBOX.md): Joker, a Clojure dialect in Go, on
// TamaGo on the Go guest's board -- a REPL in the box.  A module of its own,
// as guest/tamago is, built only by TamaGo's toolchain (go tool whim guest
// --joker); Joker is ./joker, a fork (joker/README.md).
module github.com/arbace/go-whim/guest/joker

go 1.27.1

require (
	github.com/arbace/go-whim v0.0.0
	github.com/arbace/go-whim/crefactor v0.0.0
	github.com/arbace/go-whim/guest/tamago v0.0.0
	github.com/candid82/joker v0.0.0
	// GOOSPKG, the runtime's goos overlay: imported by no package
	github.com/usbarmory/tamago v1.27.1
)

require (
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/opt v0.2.0 // indirect
	modernc.org/sortutil v1.2.1 // indirect
	modernc.org/strutil v1.2.1 // indirect
	modernc.org/token v1.1.0 // indirect
)

replace (
	github.com/arbace/go-whim => ../..
	github.com/arbace/go-whim/crefactor => ../../crefactor
	github.com/arbace/go-whim/guest/tamago => ../tamago
	github.com/candid82/joker => ./joker
)
