package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The work tree's makefile IS the compile line, and the compile line is the
// boundary's: phase 0 starts from tools/templates/whim.mk, phase 83 writes
// tools/templates/core.mk over it and phase 84 adds -fno-stack-protector.  A
// binary built with any other line is not the binary a check or a delta means.

// Flags reads CFLAGS and LDFLAGS out of the work tree's makefile, which is
// where the compile line lives: it is the boundary's, and it moves at 83 and 84.
func Flags(mk string) (c, l []string, err error) {
	b, err := os.ReadFile(mk)
	if err != nil {
		return nil, nil, err
	}
	for _, ln := range strings.Split(string(b), "\n") {
		if v, ok := after(ln, "CFLAGS"); ok {
			c = strings.Fields(v)
		} else if v, ok := after(ln, "LDFLAGS"); ok {
			l = strings.Fields(v)
		}
	}
	if len(c) == 0 {
		return nil, nil, fmt.Errorf("no CFLAGS in %s", mk)
	}
	return c, l, nil
}
func after(line, name string) (string, bool) {
	if !strings.HasPrefix(line, name) {
		return "", false
	}
	rest := strings.TrimLeft(line[len(name):], " \t")
	if !strings.HasPrefix(rest, "=") {
		return "", false
	}
	return strings.TrimSpace(rest[1:]), true
}

// ApplyMakefile is what a phase does to the work tree's makefile.
func ApplyMakefile(what, mk string) error {
	switch {
	case what == "whim" || what == "core":
		b, err := os.ReadFile(filepath.Join("tools/templates", what+".mk"))
		if err != nil {
			return err
		}
		return os.WriteFile(mk, b, 0o644)
	case strings.HasPrefix(what, "+"):
		flag := what[1:]
		b, err := os.ReadFile(mk)
		if err != nil {
			return err
		}
		var out []string
		done := false
		for _, ln := range strings.Split(string(b), "\n") {
			if v, ok := after(ln, "CFLAGS"); ok && !done {
				if strings.Contains(" "+v+" ", " "+flag+" ") {
					return fmt.Errorf("CFLAGS already carries %s", flag)
				}
				ln = "CFLAGS  = " + v + " " + flag
				done = true
			}
			out = append(out, ln)
		}
		if !done {
			return fmt.Errorf("no CFLAGS line to add %s to", flag)
		}
		return os.WriteFile(mk, []byte(strings.Join(out, "\n")), 0o644)
	}
	return fmt.Errorf("verify: no makefile action %q", what)
}

// MakefileFor is the compile line a phase's tree carries, for a run that starts
// inside the pipeline.
func MakefileFor(n int, mk string) error {
	what := ""
	var adds []string
	for _, p := range Plan {
		if p.N > n {
			break
		}
		switch {
		case p.Makefile == "":
		case strings.HasPrefix(p.Makefile, "+"):
			adds = append(adds, p.Makefile)
		default:
			what, adds = p.Makefile, nil
		}
	}
	if what == "" {
		return fmt.Errorf("verify: no makefile is defined at phase %d", n)
	}
	if err := ApplyMakefile(what, mk); err != nil {
		return err
	}
	for _, a := range adds {
		if err := ApplyMakefile(a, mk); err != nil {
			return err
		}
	}
	return nil
}
