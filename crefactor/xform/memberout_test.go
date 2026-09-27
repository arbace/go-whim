package xform

import (
	"bytes"
	"strings"
	"testing"
)

// MemberOut takes a member's address out of a call only where the copy is
// the call's: a callee that reads and writes through the pointer and
// nothing else; not one that names the member itself, nor one that keeps
// the pointer, nor a member whose address is kept elsewhere too.
func TestMemberOut(t *testing.T) {
	src := `struct s { int a; int b; int c; int d; };
static int *kept;
static void bump(int *p) { *p += 1; }
static void peek(struct s *o, int *p) { *p = o->b + 1; }
static void keep(int *p) { kept = p; }
static void twice(int *p) { bump(p); bump(p); }
static int *elsewhere;
void f(struct s *o)
{
    bump(&o->a);
    twice(&o->a);
    peek(o, &o->b);
    keep(&o->c);
    bump(&o->d);
    elsewhere = &o->d;
}
`
	var log bytes.Buffer
	out, err := MemberOut(nil)([]byte(src), nil, &log)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		"bump__a(o);", "twice__a(o);", // taken: every address of s.a goes
		"peek(o, &o->b);", // peek names b
		"keep(&o->c);",    // keep keeps the pointer
		"bump(&o->d);",    // d's address is kept elsewhere too
		"typeof(s0__->a) a0__ = s0__->a;",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in\n%s\n%s", want, got, log.String())
		}
	}
	if _, err := translate(out); err != nil {
		t.Errorf("the result does not type-check: %v\n%s", err, got)
	}
}
