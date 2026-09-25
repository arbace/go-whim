module github.com/arbace/go-whim

go 1.27.1

require (
	// The generic C refactoring library, crefactor/, is a module of its own:
	// its boundary is one the compiler keeps (it cannot import this module), and
	// it is built from the tree beside this file, never fetched.
	github.com/arbace/go-whim/crefactor v0.0.0
	modernc.org/token v1.1.0
)

require (
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/opt v0.2.0 // indirect
	modernc.org/sortutil v1.2.1 // indirect
	modernc.org/strutil v1.2.1 // indirect
)

replace github.com/arbace/go-whim/crefactor => ./crefactor

tool github.com/arbace/go-whim/cmd/whim
