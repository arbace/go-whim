module github.com/arbace/go-whim/crefactor

go 1.27.1

// The C front end is a FORK, cc/: modernc.org/cc/v4 v4.29.7 with two C23
// productions added (cc/README.md).  What is required here is what that fork
// needs and nothing else -- its test-only dependencies, ccorpus2 among them,
// are not carried.

require (
	modernc.org/mathutil v1.7.1
	modernc.org/opt v0.2.0
	modernc.org/sortutil v1.2.1
	modernc.org/strutil v1.2.1
	modernc.org/token v1.1.0
)

require github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
