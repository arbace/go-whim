package suite

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

// runPipe is how Run fed the keys before: through a pipe.  The stress test runs
// it beside Run as its control -- under load it is the way that flakes.
func runPipe(bin string, keys []byte) []byte {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin)
	cmd.Stdin = bytes.NewReader(keys)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	cmd.Run()
	return out.Bytes()
}

// TestStress runs every case REPS times on each of BINS (space-separated, each
// staged as `vim`), fed from a file (Run) and through a pipe (the control), and
// counts the runs whose output is not the case's first.  Run it under load:
//
//	BINS="$PWD/c/vim $PWD/go/vim" REPS=40 go test -run TestStress ./internal/suite
//
// Measured on 2026-09-25 with 48 busy loops: from a file 0 of 1,755 differ for
// both editors; through a pipe the Go editor differed 18 times, the C 0.
func TestStress(t *testing.T) {
	bins := os.Getenv("BINS")
	if bins == "" {
		t.Skip()
	}
	reps, _ := strconv.Atoi(os.Getenv("REPS"))
	cases, err := Cases()
	if err != nil {
		t.Fatal(err)
	}
	for _, bin := range bytes.Fields([]byte(bins)) {
		b := string(bin)
		for _, mode := range []string{"file", "pipe"} {
			diff := 0
			for _, c := range cases {
				var first []byte
				for i := 0; i < reps; i++ {
					var out []byte
					if mode == "file" {
						out, _, _ = Run(b, c.Keys)
					} else {
						out = runPipe(b, c.Keys)
					}
					if i == 0 {
						first = out
					} else if !bytes.Equal(out, first) {
						diff++
					}
				}
			}
			t.Logf("%s %s: %d of %d runs differ from the case's first run", b, mode, diff, len(cases)*(reps-1))
		}
	}
}
