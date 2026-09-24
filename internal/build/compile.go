package build

// THE COMPILE LINE IS ONE LINE, for every boundary and for the input and the
// product alike: -O0 -fno-stack-protector -static -no-pie -s -- an ordinary
// static executable (EXEC, no dynamic section, no relocation), no stack
// protector, no -g.  It used to move twice, at phase 83 (-no-pie) and 84
// (-fno-stack-protector), which is why those two phases exist; they change
// nothing now.
var (
	cflags  = []string{"-O0", "-fno-stack-protector"}
	ldflags = []string{"-static", "-no-pie", "-s"}
)

// FlagsFor is the compile line the boundary after phase n carries, which is the
// one line; n is kept so a caller says which boundary it means.
func FlagsFor(n int) ([]string, []string, error) {
	return append([]string{}, cflags...), append([]string{}, ldflags...), nil
}
