package check

import (
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { register("whim149", Whim149) }

// Whim149 is phase 149's check: the allocation-failure branches fold.
//
//  1. THE WHOLE OUTPUT IS COMPUTED: the input's core with edit.W149Rule
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
func Whim149(w io.Writer, args []string) error {
	c, err := newCore(w, args, "whim149", "allocnull")
	if err != nil {
		return err
	}
	r := c.r
	i := strings.Index(c.old, "\n#include")
	core, nn, n, _ := edit.W149Rule([]byte(c.old[:i+1]))
	pre := string(core) + c.old[i+1:]
	comp, err := swept(pre)
	if err != nil {
		r.bad("%v", err)
		return r.done()
	}
	if comp != c.new {
		l, a, b := firstDiff(comp, c.new)
		r.bad("the output is not the input with the rule applied and swept: they part at line %d\n    computed: %q\n    output:   %q", l, a, b)
	}
	if err := r.done(); err != nil {
		return err
	}
	took := sweptLines(pre, c.new)
	r.say("the output IS the input with %d allocation-failure tests folded by the rule, then swept: computed, byte for byte", n)
	r.say("the sweep took, beyond the branches, %d lines", len(took))

	j := strings.Index(c.new, "\n#include")
	outCore := []byte(c.new[:j+1])
	nnOut := edit.W149NN(outCore)
	var names []string
	for k := range nn {
		names = append(names, k)
		if !nnOut[k] {
			r.bad("%s is never-NULL on the edit's last round and not on the output", k)
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
			r.bad("%s is in the never-NULL set and returns NULL, 0 or a literal", k)
		}
		for _, l := range strings.Split(fn, "\n") {
			t := strings.TrimSpace(l)
			if strings.HasPrefix(t, "return") && !okRet.MatchString(t) {
				r.bad("%s is in the never-NULL set and returns %q", k, t)
			}
		}
	}
	ha, hz, hok := cutil.FindDefinition([]byte(c.new), cutil.Blank([]byte(c.new)), "host_alloc")
	if !hok || len(regexp.MustCompile(`\breturn\b`).FindAllString(c.new[ha:hz], -1)) != 1 || !strings.Contains(c.new[ha:hz], "return p;") {
		r.bad("host_alloc() is not one return of a pointer into the arena")
	}
	again, _, m, _ := edit.W149Rule(outCore)
	if m != 0 || string(again) != string(outCore) {
		r.bad("the rule still folds %d tests on the output", m)
	}
	if err := r.done(); err != nil {
		return err
	}
	r.say("%d functions never return NULL -- %s -- each returning only calls and names, and the rule folds nothing more",
		len(names), strings.Join(names, " "))
	return c.gate(true)
}
