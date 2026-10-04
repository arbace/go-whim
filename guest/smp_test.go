package guest

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/arbace/go-whim/editor"
	"github.com/arbace/go-whim/vmm"
)

// smpKeys builds 4,000 lines and runs a :%s and a :g across them, each
// matched in parallel chunks (editor.Chunks), printing a screen of the
// result before :q!.
var smpKeys = []string{"iabab abc bab cab, abba acdc cabbage\rthe quick brown fox\033",
	"ggVGy1999P", ":%s/\\v(a|b)+c/X/g\r", ":g/fox/s/q/Q/\r", "gg:q!\r"}

// TestGoGuestSMP (doc/GUEST.md, *SMP*): the Go guest on 1, 2 and 4 vCPUs
// answers a session whose :%s and :g run in parallel chunks exactly as the
// Go editor does natively, on the same scripted host; on more than one, the
// runtime started every AP and the guest parked and was woken through the
// monitor.
func TestGoGuestSMP(t *testing.T) {
	gocmd := needTamaGo(t)
	img, err := GoImage(gocmd, Native(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	nat := newScripted(smpKeys...)
	want := editor.Main(nat, []string{"whim"})
	for _, n := range []int{1, 2, 4} {
		g := newScripted(smpKeys...)
		var stats bytes.Buffer
		code, err := vmm.Run(vmm.Config{Image: img, Host: g, Args: []string{"whim"}, Stats: &stats, CPUs: n})
		if err != nil {
			t.Fatalf("%d vCPUs: %v", n, err)
		}
		if code != want || g.out.String() != nat.out.String() {
			t.Fatalf("%d vCPUs: exit %d, %d bytes; the Go editor: exit %d, %d bytes", n, code, g.out.Len(), want, nat.out.Len())
		}
		st := stats.String()
		if n > 1 && (!strings.Contains(st, fmt.Sprintf("call cpu_start %d\n", n-1)) || !strings.Contains(st, "call cpu_park ")) {
			t.Fatalf("%d vCPUs: not all started, or none parked:\n%s", n, st)
		}
		if n == 1 && strings.Contains(st, "cpu_") {
			t.Fatalf("one vCPU made a vCPU's call:\n%s", st)
		}
		t.Logf("%d vCPUs: exit %d, %d bytes; %s", n, code, g.out.Len(), strings.Join(strings.Fields(st), " "))
	}
}
