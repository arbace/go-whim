package suite

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRunWideLimit holds a run to the limit it is given, on a file and on a
// terminal: a program that outlives it is killed and the run is a hang, and
// one that ends within it is not.
func TestRunWideLimit(t *testing.T) {
	dir := t.TempDir()
	prog := filepath.Join(dir, "slow")
	if err := os.WriteFile(prog, []byte("#!/bin/sh\nsleep ${1:-0}\necho done\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range []WideCase{
		{Group: "keys", Name: "file"},
		{Group: "pty", Name: "pty", pty: &ptySpec{24, 80, "xterm"}},
	} {
		c.Args = []string{"1"}
		start := time.Now()
		if _, _, err := runWide(prog, c, 200*time.Millisecond); err == nil || !strings.Contains(err.Error(), "no exit within 200ms") {
			t.Errorf("%s: a 1 s run under a 200ms limit: %v", c.Name, err)
		}
		if d := time.Since(start); d > 900*time.Millisecond {
			t.Errorf("%s: killed after %s, not 200ms", c.Name, d)
		}
		out, code, err := runWide(prog, c, 5*time.Second)
		if err != nil || code != 0 || !bytes.Contains(out, []byte("done")) {
			t.Errorf("%s: a 1 s run under a 5s limit: %q, %d, %v", c.Name, out, code, err)
		}
	}
}

// TestPatchControl holds the control of a prebuilt program to its one copy
// of the literal: changed in an executable copy, the original untouched, and
// refused when the literal is not there exactly once.
func TestPatchControl(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "prog")
	orig := append(append([]byte("\x7fELF..."), hsBinControlOld...), "...end"...)
	if err := os.WriteFile(bin, orig, 0o755); err != nil {
		t.Fatal(err)
	}
	ctl, err := patchControl(bin, dir, "prog-control", hsBinControlOld, hsBinControlNew)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(ctl)
	if err != nil {
		t.Fatal(err)
	}
	if want := bytes.Replace(orig, hsBinControlOld, hsBinControlNew, 1); !bytes.Equal(got, want) {
		t.Errorf("the control is %q, not %q", got, want)
	}
	if st, err := os.Stat(ctl); err != nil || st.Mode().Perm()&0o100 == 0 {
		t.Errorf("the control is not executable: %v", err)
	}
	if have, _ := os.ReadFile(bin); !bytes.Equal(have, orig) {
		t.Error("the program itself was changed")
	}
	for _, n := range []int{0, 2} {
		b := append([]byte("x"), bytes.Repeat(hsBinControlOld, n)...)
		if err := os.WriteFile(bin, b, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := patchControl(bin, dir, "c", hsBinControlOld, hsBinControlNew); err == nil {
			t.Errorf("the literal %d times: not refused", n)
		}
	}
}
