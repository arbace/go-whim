// The Go guest (doc/GUEST.md, *A second guest*): the Go editor built with
// TamaGo (GOOS=tamago) on a board for whim's monitor.  A module of its own,
// so that the main module stays standard Go: `go build ./...` there does
// not see it, and only TamaGo's toolchain builds it (go tool whim guest
// --go, with TAMAGO_ROOT).
module github.com/arbace/go-whim/guest/tamago

go 1.27.1

require (
	github.com/arbace/go-whim v0.0.0
	// GOOSPKG, the runtime's goos overlay: imported by no package, so
	// that `go mod tidy` would drop it
	github.com/usbarmory/tamago v1.27.1
)

replace (
	github.com/arbace/go-whim => ../..
	github.com/arbace/go-whim/crefactor => ../../crefactor
)
