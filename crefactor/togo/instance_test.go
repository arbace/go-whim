package togo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A toy program in the shape the generator writes: state in package
// variables (one with its storage declared), a table of function pointers
// filled in init, a local that shadows a global, a helper that touches no
// state, and a hand-written file with state of its own (embedded) and a
// method the generated code calls.
const instGen = `package toy

type handler = func(int32) int32

var count int32
var buf []byte = make([]byte, 8) // storage the C declares with the object
var table [2]handler

func init() {
	table[0] = bump
	table[1] = twice
}

func twice(n int32) int32 { return n * 2 }

func bump(n int32) int32 {
	count += n
	return count
}

func run(n int32) int32 {
	count := int32(100) // a local: not the state
	_ = count
	out(n)
	buf[0] = byte(n)
	return table[0](n) + table[1](n)
}
`

const instHand = `package toy

type hostState struct{ written int }

func (m *Machine) out(n int32) { m.written++ }

func New() *Machine {
	m := &Machine{}
	m.initGlobals()
	return m
}
`

const instMain = `package toy

import "testing"

func TestInstances(t *testing.T) {
	a, b := New(), New()
	if got := a.run(3); got != 3+6 {
		t.Fatalf("a.run(3) = %d", got)
	}
	a.run(4)
	if got := b.run(5); got != 5+10 {
		t.Fatalf("b.run(5) = %d, want 15: b saw a's state", got)
	}
	if a.count != 7 || b.count != 5 || a.written != 2 || b.written != 1 || a.buf[0] != 4 {
		t.Fatalf("a %d/%d/%d, b %d/%d", a.count, a.written, a.buf[0], b.count, b.written)
	}
	if twice(2) != 4 {
		t.Fatal("twice")
	}
}
`

func TestInstanceRewrite(t *testing.T) {
	methods, fields, err := HandNames(map[string][]byte{"hand.go": []byte(instHand)}, "Machine", "hostState")
	if err != nil {
		t.Fatal(err)
	}
	o := Instance{Type: "Machine", Receiver: "m", Init: "initGlobals", Embed: "hostState",
		Own: []string{"gen.go"}, Fields: fields, Methods: methods}
	out, err := o.Rewrite(map[string][]byte{"gen.go": []byte(instGen)})
	if err != nil {
		t.Fatal(err)
	}
	gen := string(out["gen.go"])
	for _, want := range []string{
		"type Machine struct {\n\thostState\n\tcount int32\n\tbuf   []byte",
		"func (m *Machine) initGlobals() {",
		"m.buf = make([]byte, 8)",
		"m.table[0] = m.bump", // a method as a value: bound to the instance
		"m.table[1] = twice",  // a helper that touches no state stays a function
		"func twice(n int32) int32",
		"count := int32(100)", // the local is left alone
		"m.out(n)",
		"return m.table[0](n) + m.table[1](n)",
	} {
		if !strings.Contains(gen, want) {
			t.Errorf("rewritten file lacks %q:\n%s", want, gen)
		}
	}
	// And it is a program: two instances, each with its own state.
	dir := t.TempDir()
	for n, s := range map[string]string{"gen.go": gen, "hand.go": instHand, "toy_test.go": instMain,
		"go.mod": "module toy\n\ngo 1.22\n"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off")
	if o, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the rewritten toy does not test:\n%s", o)
	}
}

// The receiver's name must be free: a function that already has a local of
// that name is refused, not silently shadowed.
func TestInstanceReceiverTaken(t *testing.T) {
	src := "package toy\n\nvar n int\n\nfunc f() int {\n\tm := 1\n\treturn n + m\n}\n\nfunc init() {}\n"
	o := Instance{Type: "Machine", Receiver: "m", Init: "initGlobals", Own: []string{"gen.go"}}
	if _, err := o.Rewrite(map[string][]byte{"gen.go": []byte(src)}); err == nil || !strings.Contains(err.Error(), "receiver name") {
		t.Fatalf("err = %v, want the receiver name refused", err)
	}
}
