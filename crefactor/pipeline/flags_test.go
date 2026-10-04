package pipeline

import (
	"fmt"
	"testing"
)

// TestFlags: each wanted name with its count, and a refusal for a name not
// wanted, a name with no count, and a count that is not one.
func TestFlags(t *testing.T) {
	f, err := Flags("t", []string{"--at-least", "3", "--calls", "0"}, "--at-least", "--calls")
	if err != nil || fmt.Sprint(f) != "map[--at-least:3 --calls:0]" {
		t.Errorf("%v %v", f, err)
	}
	if f, err := Flags("t", nil, "--at-least"); err != nil || len(f) != 0 {
		t.Errorf("none: %v %v", f, err)
	}
	for _, args := range [][]string{{"--most", "3"}, {"--at-least"}, {"--at-least", "x"}, {"--at-least", "-1"}} {
		if _, err := Flags("t", args, "--at-least"); err == nil {
			t.Errorf("%q accepted", args)
		}
	}
}
