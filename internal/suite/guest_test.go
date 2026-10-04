package suite

import (
	"bytes"
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
// suite's cases (its Ex commands when WHIM_SUITE_SRC names the whim-vim.c
// the C was built from); WHIM_SUITE_ONLY=GROUP runs one group alone;
// WHIM_SUITE_LIMIT is a run's limit (default 10s).  The launcher is the C
// guest's or the Go guest's (whim guest --go), told apart by the control's
// literal (controlLiteral).  WHIM_SUITE_GUEST_IMAGE names the
// image when the launcher carries none -- the Mac's, signed, which cannot --
// and the control is then that image changed, each run by a script that
// hands the monitor its image (WHIM_GUEST_IMAGE).  WHIM_SUITE_HEAVY=1 runs
// the heavy case after the cases (heavy.go: 5,000 lines, three :s and a :g),
// the C and then the guest, one at a time, the answers compared and the
// times reported -- a native speed figure where the suite has no toolset,
// the Mac's; WHIM_SUITE_HEAVY=only runs it alone.  The test
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
		if p := os.Getenv("WHIM_SUITE_SRC"); p != "" {
			src, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			ex, err := exCases(src)
			if err != nil {
				t.Fatal(err)
			}
			w = append(w, ex...)
		}
		for i := range w {
			w[i].Group = "wide-" + w[i].Group
		}
		all = append(all, w...)
		groups = append(groups, "wide-keys", "wide-ex", "wide-argv", "wide-pty")
	}
	if only := os.Getenv("WHIM_SUITE_ONLY"); only != "" {
		var some []WideCase
		for _, c := range all {
			if c.Group == only {
				some = append(some, c)
			}
		}
		all, groups = some, []string{only}
	}
	limit := DefaultLimit
	if l := os.Getenv("WHIM_SUITE_LIMIT"); l != "" {
		var err error
		if limit, err = time.ParseDuration(l); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	abs := func(p string) string { a, _ := filepath.Abs(p); return a }
	var ctl string
	img := os.Getenv("WHIM_SUITE_GUEST_IMAGE")
	prog := g
	if img != "" {
		prog = img
	}
	from, to := controlLiteral(prog)
	if img != "" {
		ci, err := patchControl(img, dir, "control.elf", from, to)
		if err != nil {
			t.Fatal(err)
		}
		mon := abs(g)
		if g, err = imageScript(dir, "whim-guest", mon, abs(img)); err != nil {
			t.Fatal(err)
		}
		if ctl, err = imageScript(dir, "whim-guest-control", mon, ci); err != nil {
			t.Fatal(err)
		}
	} else if ctl, err = patchControl(g, dir, "whim-guest-control", from, to); err != nil {
		t.Fatal(err)
	}
	e := &jvmEditor{name: "guest", where: "guest/, vmm/", file: "the image", launcher: "whim-guest",
		bin: abs(g), ctl: ctl, limit: limit, report: guestExits}
	if from[len(from)-1] != 0 {
		e.name, e.where, e.launcher = "Go guest", "guest/tamago/, vmm/", "whim-guest-go"
	}
	b := &builds{cand: abs(c)}
	var out strings.Builder
	heavy := os.Getenv("WHIM_SUITE_HEAVY")
	if heavy != "only" {
		err = checkJVM(&out, "guest", groups, all, b, e)
	}
	if err == nil && heavy != "" {
		// the C is the reference and the timed candidate both: one run
		// untimed, then each editor timed alone
		b.ref, b.jvm = b.cand, []*jvmEditor{e}
		err = checkHeavy(&out, "the C", b)
	}
	t.Log("\n" + out.String())
	if err != nil {
		t.Fatal(err)
	}
}

// imageScript writes dir/name, a script running the monitor with the image
// img, and returns its path.  monitor is a program, or another such script.
func imageScript(dir, name, monitor, img string) (string, error) {
	q := func(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
	p := filepath.Join(dir, name)
	return p, os.WriteFile(p, []byte("#!/bin/sh\nWHIM_GUEST_IMAGE="+q(img)+" exec "+q(monitor)+` "$@"`+"\n"), 0o755)
}

// controlLiteral is what the control changes in the program at p: the C
// guest's " INSERT" with its terminating NUL, or, where the image holds
// none -- the Go guest's, whose strings have no NUL -- its one " INSERT".
func controlLiteral(p string) (from, to []byte) {
	if b, err := os.ReadFile(p); err == nil && bytes.Count(b, []byte(" INSERT\x00")) != 1 {
		return []byte(" INSERT"), []byte(" INSERX")
	}
	return []byte(" INSERT\x00"), []byte(" INSERX\x00")
}
