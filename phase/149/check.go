package p149

// Whim phase 149, the check -- the allocation-failure branches fold.
// See phase/149/edit.go, and GOALS.md.
//
// phase/149/check.go computes the whole output with the same rule and
// the real sweep, and checks the never-NULL set again on the output.

import (
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/cutil"
)

func init() { check.Register("whim149", Check) }

// Whim149 is phase 149's check: the allocation-failure branches fold.
//
//  1. THE WHOLE OUTPUT IS COMPUTED: the input's core with W149Rule
//     applied -- the same function -- and the host unchanged, run through the
//     real sweep, is the output byte for byte.
//  2. THE NEVER-NULL SET, CHECKED AGAIN ON THE OUTPUT, and by a second test
//     that does not share the rule's parse: each function in it returns only
//     calls and plain names, never nullptr or a literal, and host_alloc() --
//     where the set starts -- is still one return of a pointer into the arena.
//  3. NO FAILURE TEST IS LEFT that the rule could fold: applying it to the
//     output changes nothing.
//  4. THE GATE, the libc surface unchanged; the recording is the stage's delta
//     check -- nothing it does can fail to allocate, before or after.
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim149", "allocnull")
	if err != nil {
		return err
	}
	r := c.R
	i := strings.Index(c.Old, "\n#include")
	core, nn, n, _ := W149Rule([]byte(c.Old[:i+1]))
	pre := string(core) + c.Old[i+1:]
	comp, err := check.Swept(pre)
	if err != nil {
		r.Bad("%v", err)
		return r.Done()
	}
	if comp != c.New {
		l, a, b := check.FirstDiff(comp, c.New)
		r.Bad("the output is not the input with the rule applied and swept: they part at line %d\n    computed: %q\n    output:   %q", l, a, b)
	}
	if err := r.Done(); err != nil {
		return err
	}
	took := check.SweptLines(pre, c.New)
	r.Say("the output IS the input with %d allocation-failure tests folded by the rule, then swept: computed, byte for byte", n)
	r.Say("the sweep took, beyond the branches, %d lines", len(took))

	j := strings.Index(c.New, "\n#include")
	outCore := []byte(c.New[:j+1])
	nnOut := W149NN(outCore)
	var names []string
	for k := range nn {
		names = append(names, k)
		if !nnOut[k] {
			r.Bad("%s is never-NULL on the edit's last round and not on the output", k)
		}
	}
	sort.Strings(names)
	bad := regexp.MustCompile(`\breturn\s+(nullptr|NULL|0|"[^"]*"|\([^)]*\)\s*0)\s*;`)
	okRet := regexp.MustCompile(`^return\s+((\([^()]*\)\s*)?[A-Za-z_]\w*(\(.*\))?)\s*;$`)
	blank := cutil.Blank(outCore)
	for _, k := range names {
		if k == "host_alloc" {
			continue
		}
		a, z, ok := cutil.FindDefinition(outCore, blank, k)
		if !ok {
			continue // swept: nothing calls it
		}
		fn := string(outCore[a:z])
		if bad.MatchString(fn) {
			r.Bad("%s is in the never-NULL set and returns NULL, 0 or a literal", k)
		}
		for _, l := range strings.Split(fn, "\n") {
			t := strings.TrimSpace(l)
			if strings.HasPrefix(t, "return") && !okRet.MatchString(t) {
				r.Bad("%s is in the never-NULL set and returns %q", k, t)
			}
		}
	}
	ha, hz, hok := cutil.FindDefinition([]byte(c.New), cutil.Blank([]byte(c.New)), "host_alloc")
	if !hok || len(regexp.MustCompile(`\breturn\b`).FindAllString(c.New[ha:hz], -1)) != 1 || !strings.Contains(c.New[ha:hz], "return p;") {
		r.Bad("host_alloc() is not one return of a pointer into the arena")
	}
	again, _, m, _ := W149Rule(outCore)
	if m != 0 || string(again) != string(outCore) {
		r.Bad("the rule still folds %d tests on the output", m)
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("%d functions never return NULL -- %s -- each returning only calls and names, and the rule folds nothing more",
		len(names), strings.Join(names, " "))
	return c.Gate(true)
}
