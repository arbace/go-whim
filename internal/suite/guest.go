package suite

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/arbace/go-whim/guest"
)

// THE GUEST EDITOR (doc/GUEST.md), on demand: `whim test --guest`.  The
// candidate's core compiled freestanding with the guest runtime into an
// image, appended to the monitor (vmm/), which runs it as a virtual machine
// on this machine's KVM -- held to what every other editor is held to:
// every case answered as the C candidate answers it, and a control of its
// own, the launcher with the one " INSERT" in its image changed.
//
// The image is kept in .cache/guest-suite/ under the digest of what it is
// compiled from: a run on an unchanged core pays for no compile.

var guestCache = filepath.Join(".cache", "guest-suite")

// buildGuest builds the guest editor from candSrc for this machine, and
// its control, in dir.
func buildGuest(candSrc, dir string) (*jvmEditor, error) {
	a := guest.Native()
	e := &jvmEditor{name: "guest", where: "guest/, vmm/", file: "the image", launcher: "whim-guest",
		frame: regexp.MustCompile(`$^`),
		note:  "its control is the launcher with the image's one \" INSERT\" changed in its bytes"}
	c, err := os.ReadFile(candSrc)
	if err != nil {
		return nil, err
	}
	tu, err := guest.Source(c)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(append(append([]byte(a.Name+"\x00"), tu...), guest.Runtime()...))
	img := filepath.Join(guestCache, hex.EncodeToString(sum[:12])+".elf")
	b, err := os.ReadFile(img)
	if err != nil {
		if b, err = guest.Image(tu, a, filepath.Join(dir, "guest-image")); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(guestCache, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(img, b, 0o644); err != nil {
			return nil, err
		}
	}
	mon, err := guest.Monitor(a, dir)
	if err != nil {
		return nil, err
	}
	e.bin = filepath.Join(dir, "whim-guest")
	if err := guest.Launcher(mon, b, e.bin); err != nil {
		return nil, err
	}
	if e.ctl, err = patchControl(e.bin, dir, "whim-guest-control", []byte(" INSERT\x00"), []byte(" INSERX\x00")); err != nil {
		return nil, err
	}
	e.report = guestExits
	return e, nil
}

// guestExits runs every case once more with the monitor counting, and
// reports the exits a key costs: every VCPURun's return over every byte of
// input the cases hand the editor.
func guestExits(w io.Writer, label string, e *jvmEditor, cases []WideCase) error {
	var exits, keys, calls, reads, waitReads uint64
	// and without the cases that print thousands of lines (par_*): typing
	var tExits, tKeys uint64
	dir, err := os.MkdirTemp("", "guest-stats.")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	for i, c := range cases {
		if c.pty != nil {
			continue
		}
		p := filepath.Join(dir, strconv.Itoa(i))
		if _, _, err := runEnv(e.bin, c.Args, c.Keys, e.limit, []string{"WHIM_GUEST_STATS=" + p}); err != nil {
			return fmt.Errorf("%s %s with the monitor counting: %w", c.Group, c.Name, err)
		}
		s, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("%s %s wrote no counts: %w", c.Group, c.Name, err)
		}
		st := parseStats(s)
		exits += st["exits"]
		calls += st["calls"]
		reads += st["call read_input"]
		waitReads += st["call wait_read"]
		keys += uint64(len(c.Keys))
		if !strings.HasPrefix(c.Name, "par_") {
			tExits += st["exits"]
			tKeys += uint64(len(c.Keys))
		}
	}
	if keys == 0 {
		return nil
	}
	fmt.Fprintf(w, "  %-12s %d exits, %d of them calls (%d waits that read, %d reads of their own), for %d bytes of keys: %.2f exits a key", label, exits, calls, waitReads, reads, keys, float64(exits)/float64(keys))
	if tKeys > 0 && tKeys < keys {
		fmt.Fprintf(w, "; %.2f in the cases not par_*", float64(tExits)/float64(tKeys))
	}
	fmt.Fprintln(w)
	return nil
}

// parseStats reads the monitor's counts: "name value" per line, the first
// line's pairs too.
func parseStats(b []byte) map[string]uint64 {
	m := map[string]uint64{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) > 0 && f[0] == "call" && len(f) == 3 {
			n, _ := strconv.ParseUint(f[2], 10, 64)
			m["call "+f[1]] = n
			continue
		}
		for i := 0; i+1 < len(f); i += 2 {
			n, _ := strconv.ParseUint(strings.TrimSuffix(f[i+1], "us"), 10, 64)
			m[f[i]] = n
		}
	}
	return m
}

// THE GO GUEST (doc/GUEST.md, *A second guest*), on demand: `whim test
// --guest-go`.  The Go editor, editor/ as it stands, built with TamaGo for
// the monitor and appended to it -- held to the same: every case answered
// as the C candidate answers it, and a control of its own, the launcher
// with the one " INSERT" in its image changed.  TamaGo is $TAMAGO_ROOT's,
// or root when given (--tamago).
func buildGuestGo(root, dir string) (*jvmEditor, error) {
	a := guest.Native()
	e := &jvmEditor{name: "Go guest", where: "guest/tamago/, vmm/", file: "the image", launcher: "whim-guest-go",
		frame: regexp.MustCompile(`$^`),
		note:  "its control is the launcher with the image's one \" INSERT\" changed in its bytes"}
	gocmd, err := guest.TamaGo(root)
	if err != nil {
		return nil, err
	}
	img, err := guest.GoImage(gocmd, a, filepath.Join(dir, "guest-go-image"))
	if err != nil {
		return nil, err
	}
	mon, err := guest.Monitor(a, filepath.Join(dir, "guest-go"))
	if err != nil {
		return nil, err
	}
	e.bin = filepath.Join(dir, "whim-guest-go")
	if err := guest.Launcher(mon, img, e.bin); err != nil {
		return nil, err
	}
	if e.ctl, err = patchControl(e.bin, dir, "whim-guest-go-control", []byte(" INSERT"), []byte(" INSERX")); err != nil {
		return nil, err
	}
	e.report = guestExits
	return e, nil
}
