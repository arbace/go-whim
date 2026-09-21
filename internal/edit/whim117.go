package edit

import (
	"bytes"
	"io"
	"regexp"
	"strings"
)

func init() { register("whim117", Zero34) }

var (
	whim117Directive = regexp.MustCompile(`^ *#`)
	whim117Inc       = regexp.MustCompile(`^ *# *include <([A-Za-z0-9_/.]+)>$`)
	// `\brealloc\b` does NOT match inside `realloc_cmdbuff` -- `_` is a word
	// character -- so this counts the libc name alone, which is what the
	// phase is about.
	whim117Realloc = regexp.MustCompile(`\brealloc\b`)
)

// ga_grow_inner: the old allocation is `ga_itemsize * ga_maxlen`, which the
// input computes to zero the tail.  It is hoisted ABOVE the allocation so the
// copy can use it, and the zeroing statement is left exactly as it was, BELOW
// the copy.
//
// THE GUARD IS TRAP 1.  On a first grow `ga_data` is null and `ga_maxlen` is
// 0; `realloc(nullptr, n)` is `malloc(n)` and frees nothing, so the rewrite
// must neither copy from null nor free null.
//
// THE `return FAIL` IS TRAP 2, and its position is the whole of it: nothing is
// freed above it, so a failed allocation leaves `gap->ga_data` pointing at a
// block that is still valid and still allocated, which is what `realloc`
// guaranteed and what the caller relies on.
const whim117OldGa = `    new_len = (usize)gap->ga_itemsize * (gap->ga_len + n);
    pp =  realloc((gap->ga_data), (new_len)) ;
    if (pp == nullptr)
    {
        return FAIL;
    }
    old_len = (usize)gap->ga_itemsize * gap->ga_maxlen;
`

const whim117NewGa = `    new_len = (usize)gap->ga_itemsize * (gap->ga_len + n);
    old_len = (usize)gap->ga_itemsize * gap->ga_maxlen;
    pp = malloc(new_len);
    if (pp == nullptr)
    {
        return FAIL;
    }
    if (gap->ga_data != nullptr)
    {
        musl_memcpy(pp, gap->ga_data, old_len);
        free(gap->ga_data);
    }
`

const whim117Zero = "     musl_memset((pp + old_len), (0), (new_len - old_len)) ;\n"

// get_keystroke: `t_buflen` is declared beside the `t_buf` the input already
// keeps, so the two halves of what `realloc`'s interface does not carry -- the
// old pointer and the old size -- are saved together and on adjacent lines.
// `buflen - 100` would be the same number and a worse text.
//
// TRAP 2 AGAIN, and the other shape of it: here the caller frees the old block
// ITSELF on failure, so `vim_free(t_buf)` stays exactly where it is and the
// success-path free goes in an `else`.  It is `free()` and not `vim_free()`
// because `vim_free` declines while `really_exiting` and `realloc` freed
// regardless.
const whim117OldKs = `            char_u  *t_buf = buf;

            buflen += 100;
            buf =  realloc((buf), (buflen)) ;
            if (buf == nullptr)
            {
                vim_free(t_buf);
            }
`

const whim117NewKs = `            char_u  *t_buf = buf;
            int     t_buflen = buflen;

            buflen += 100;
            buf = malloc(buflen);
            if (buf == nullptr)
            {
                vim_free(t_buf);
            }
            else
            {
                musl_memcpy(buf, t_buf, t_buflen);
                free(t_buf);
            }
`

const whim117Proto = "void *realloc(void *p, usize n);\n"

// Zero34 stops the core reallocating: `realloc` is rewritten at its two core
// sites as an allocation, a copy of the OLD size and a free.
//
// It is the one libc function that cannot be vendored at all -- to move the
// old contents it needs a length its interface does not carry -- so the route
// is each call site supplying the length it already knows.
func Zero34(text []byte, w io.Writer) ([]byte, error) {
	p := ph{"realloc", w}
	lines := bytes.Split(text, []byte{'\n'})
	linesBefore := len(lines)
	runsBefore := p.blankRuns(text)

	// ---- 0. the file this edit was written against -----------------------
	// The boundary is the first `#include` and nothing else marks it.  The
	// core is everything above; the host is everything from there down, and
	// the third `realloc` call is the host's and is not this phase's.
	var d []int
	for i, l := range lines {
		if whim117Directive.Match(l) {
			d = append(d, i)
		}
	}
	if len(d) == 0 {
		return nil, p.die("the file has no preprocessor directive, so there is no boundary and no way to " +
			"tell a core call from a host one")
	}
	for k, i := range d {
		if i != d[0]+k {
			return nil, p.die("the %d directives are not on consecutive lines, so the first `#include` is not "+
				"a boundary", len(d))
		}
	}
	for _, i := range d {
		if !whim117Inc.Match(lines[i]) {
			return nil, p.die("a directive is not an `#include <...>` of a system header")
		}
	}
	bound := d[0]
	core := bytes.Join(lines[:bound], []byte{'\n'})
	host := bytes.Join(lines[bound:], []byte{'\n'})
	p.sayf("%d directives on consecutive lines from %d, every one an `#include <...>`; the "+
		"core is the %d lines above the first of them", len(d), bound+1, bound)

	// ---- 1. the inventory, as a partition and not a list -----------------
	nc := len(whim117Realloc.FindAll(core, -1))
	nh := len(whim117Realloc.FindAll(host, -1))
	if nc != 3 || nh != 1 {
		return nil, p.die("`realloc` occurs %d times in the core and %d in the host, where this phase was "+
			"written against 3 and 1: the prototype and two calls above the boundary, and "+
			"adjust_types() below it", nc, nh)
	}
	p.say("`realloc` is 3 in the core -- the prototype, ga_grow_inner's call and " +
		"get_keystroke's -- and 1 in the host, adjust_types(), which is NOT this phase's " +
		"and is why the symbol does not leave")

	// NO LITERAL MAY HOLD THE NAME.  Phase 23 was caught out by three string
	// literals holding `NULL`; the lesson is applied rather than assumed.
	spans, err := literalSpansShort(p, text)
	if err != nil {
		return nil, err
	}
	var bad []string
	for _, s := range spans {
		if whim117Realloc.Match(text[s[0]:s[1]]) {
			bad = append(bad, string(text[s[0]:s[1]]))
		}
	}
	if len(bad) > 0 {
		return nil, p.die("a literal holds the name `realloc`: %s", strings.Join(bad, " / "))
	}
	p.sayf("%d string and character literals, none holding `realloc`, so both substitutions "+
		"below are over code", len(spans))

	// ---- 2. ga_grow_inner -- the size is already on the next line --------
	if bytes.Count(text, []byte(whim117OldGa)) != 1 {
		return nil, p.die("ga_grow_inner's realloc and the two lines either side are not in the file " +
			"exactly once, so this phase cannot tell what the old size is")
	}
	if bytes.Count(text, []byte(whim117Zero)) != 1 {
		return nil, p.die("ga_grow_inner's tail-zeroing statement is not in the file exactly once, and " +
			"trap 3 is that it must survive the rewrite unmoved and BELOW the copy")
	}
	text = bytes.Replace(text, []byte(whim117OldGa), []byte(whim117NewGa), 1)
	if bytes.Index(text, []byte(whim117NewGa)) >= bytes.Index(text, []byte(whim117Zero)) {
		return nil, p.die("the copy did not land above the tail-zeroing statement")
	}
	p.say("ga_grow_inner: `realloc(ga_data, new_len)` -> `malloc(new_len)` with the copy and " +
		"the free GUARDED by `ga_data != nullptr`, and `old_len` hoisted above the " +
		"allocation -- it is the old size and the function already computed it.  The " +
		"`return FAIL` still comes before anything is freed, so a failed grow leaves the " +
		"old block valid and ga_data untouched; the tail-zeroing statement is unmoved and " +
		"below the copy")

	// ---- 3. get_keystroke -- the size is `buflen` before the `+= 100` ----
	if bytes.Count(text, []byte(whim117OldKs)) != 1 {
		return nil, p.die("get_keystroke's realloc, the `buflen += 100` above it and the `vim_free` " +
			"below it are not in the file exactly once")
	}
	text = bytes.Replace(text, []byte(whim117OldKs), []byte(whim117NewKs), 1)
	p.say("get_keystroke: `realloc(buf, buflen)` -> `malloc(buflen)` with the old size saved " +
		"as `t_buflen` beside the old pointer `t_buf`, one line above the `buflen += 100`.  " +
		"The failure path is untouched -- `vim_free(t_buf)`, which is what this site always " +
		"did for itself -- and the success path frees with `free()`, not `vim_free()`, " +
		"because vim_free declines while really_exiting and realloc did not")

	// ---- 4. the prototype, which now declares nothing the core uses ------
	if bytes.Count(text, []byte(whim117Proto)) != 1 {
		return nil, p.die("`%s` is not in the file exactly once -- it is one of the plain libc prototypes "+
			"phase 26 wrote and phase 27 carried above the includes", strings.TrimSpace(whim117Proto))
	}
	text = bytes.Replace(text, []byte(whim117Proto), nil, 1)
	p.say("`void *realloc(void *p, usize n);` is gone from the core's libc prototype block.  " +
		"adjust_types() takes its declaration from <stdlib.h>, which is above it")

	// ---- 5. what the file is now -----------------------------------------
	L := bytes.Split(text, []byte{'\n'})
	const added = 5 + 6 - 1
	if len(L) != linesBefore+added {
		return nil, p.die("the file is %d lines and the input was %d -- this phase adds exactly %d: 5 at "+
			"ga_grow_inner, 6 at get_keystroke and -1 for the prototype",
			len(L)-1, linesBefore-1, added)
	}
	var nd []int
	for i, l := range L {
		if whim117Directive.Match(l) {
			nd = append(nd, i)
		}
	}
	if len(nd) != len(d) {
		return nil, p.die("the file has %d directives and had %d: this phase adds none and removes none",
			len(nd), len(d))
	}
	if nd[0]-d[0] != len(L)-linesBefore {
		return nil, p.die("the boundary moved by %d lines and the file by %d: every line this phase "+
			"touches is above the first `#include`", nd[0]-d[0], len(L)-linesBefore)
	}
	ncore := bytes.Join(L[:nd[0]], []byte{'\n'})
	nhost := bytes.Join(L[nd[0]:], []byte{'\n'})
	if whim117Realloc.Match(ncore) {
		return nil, p.die("`realloc` still occurs %d times in the core, and the phase's whole product is "+
			"that it is 0", len(whim117Realloc.FindAll(ncore, -1)))
	}
	if len(whim117Realloc.FindAll(nhost, -1)) != nh {
		return nil, p.die("the host's %d `realloc` did not survive: adjust_types() is the host's and no "+
			"phase of this pipeline has taken it", nh)
	}
	if r := p.blankRuns(text); r != runsBefore {
		return nil, p.die("the edit left %d runs of two blank lines where there were %d", r, runsBefore)
	}
	p.sayf("the core does not name `realloc` at all: 3 -> 0 above the boundary, 1 -> 1 below "+
		"it, %d directives unmoved relative to the text, and the blank-line runs unchanged "+
		"at %d", len(nd), p.blankRuns(text))
	return text, nil
}
