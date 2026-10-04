package suite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestGuestPrebuilt holds a guest launcher built already to a C editor built
// already, on a machine with neither the toolset nor the sources: the arm64
// gate of doc/GUEST.md, run in an arm64 Linux VM.  WHIM_SUITE_C is the C
// editor, WHIM_SUITE_GUEST the launcher; WHIM_SUITE_WIDE=1 adds the wide
// suite's cases; WHIM_SUITE_LIMIT a run's limit (default 10s).  The test
// binary is built here, `GOARCH=arm64 go test -c ./internal/suite`, and run
// there: the cases are compiled into it.
func TestGuestPrebuilt(t *testing.T) {
	c, g := os.Getenv("WHIM_SUITE_C"), os.Getenv("WHIM_SUITE_GUEST")
	if c == "" || g == "" {
		t.Skip("WHIM_SUITE_C and WHIM_SUITE_GUEST name the two programs")
	}
	cases, err := Cases()
	if err != nil {
		t.Fatal(err)
	}
	var all []WideCase
	for _, k := range cases {
		all = append(all, WideCase{Group: "keys", Name: k.Name, Keys: k.Keys})
	}
	groups := []string{"keys"}
	if os.Getenv("WHIM_SUITE_WIDE") == "1" {
		w, err := WideCases()
		if err != nil {
			t.Fatal(err)
		}
		for i := range w {
			w[i].Group = "wide-" + w[i].Group
		}
		all = append(all, w...)
		groups = append(groups, "wide-keys", "wide-ex", "wide-argv", "wide-pty")
	}
	limit := DefaultLimit
	if l := os.Getenv("WHIM_SUITE_LIMIT"); l != "" {
		var err error
		if limit, err = time.ParseDuration(l); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	ctl, err := patchControl(g, dir, "whim-guest-control", []byte(" INSERT\x00"), []byte(" INSERX\x00"))
	if err != nil {
		t.Fatal(err)
	}
	abs := func(p string) string { a, _ := filepath.Abs(p); return a }
	e := &jvmEditor{name: "guest", where: "guest/, vmm/", file: "the image", launcher: "whim-guest",
		bin: abs(g), ctl: ctl, limit: limit, report: guestExits}
	b := &builds{cand: abs(c)}
	var out strings.Builder
	err = checkJVM(&out, "guest", groups, all, b, e)
	t.Log("\n" + out.String())
	if err != nil {
		t.Fatal(err)
	}
}
