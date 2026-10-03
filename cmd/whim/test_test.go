package main

import (
	"testing"
	"time"
)

// TestParseTest holds runTest's arguments to what they ask for: --limit a
// positive duration, 0 (the suite's default) when absent, and --haskell-bin
// a program.
func TestParseTest(t *testing.T) {
	o, err := parseTest(nil)
	if err != nil || o.rev != "HEAD" || o.file != "src/whim-vim.c" || o.wide || o.jvm.Limit != 0 || o.jvm.HaskellBin != "" {
		t.Fatalf("no arguments: %+v, %v", o, err)
	}
	o, err = parseTest([]string{"--wide", "--haskell-bin", "/x/caprice", "--limit", "2m", "--ref", "abc", "f.c"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.wide || o.jvm.HaskellBin != "/x/caprice" || o.jvm.Limit != 2*time.Minute || o.rev != "abc" || o.file != "f.c" {
		t.Fatalf("got %+v", o)
	}
	for _, bad := range [][]string{{"--limit", "ten"}, {"--limit", "0s"}, {"--limit", "-1s"}, {"--limit"}, {"--haskell-bin"}, {"--nope"}} {
		if _, err := parseTest(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}
