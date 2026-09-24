package sweep

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/arbace/go-whim/internal/ccx"
	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/dead"
	"github.com/arbace/go-whim/internal/reach"
)

// THE CLOSURE SWITCH, and the one place it is read.  With WHIM_CLOSURE=1,
// typereach, deadfields and deadenums take WHAT TO DELETE from
// internal/reach's closure instead of their own textual analysis, and keep
// their own DELETION code: dead.DeleteDefs, dead.DeleteFields, and
// dead.AnalyseEnumsWith's pinning.  deadsweep, deadprotos, funcreach and canon
// are unchanged.  Off, which is the default, the sweep is what it was.
//
// This is doc/surveys/REACHABILITY.md's Option 3 as a measurement: whether the
// product moves, and whether any phase refuses, is the question it answers.
//
// The guards are REFUSALS, each counted in the tool's line, never filters:
//
//   - a member the closure classes as initialised by position (Moves > 0) is
//     never deleted, nor a type holding one;
//   - a member of a struct or union punned through a pointer cast is held by
//     the closure itself (reach.ClassCast), and counted in deadfields' line;
//   - no member is deleted while ml_recover is defined, deadfields' rule;
//   - a name the instrument could not place (a designator, an unjoined member
//     access, a reference charged to no entity) is never deleted;
//   - an entity something still in the text refers to waits: it is deleted
//     only with every referrer, or after the tool that deletes the referrer
//     has run -- deleting a type an unreachable function still names breaks
//     the text until funcreach reaches it, and may break it for ever when
//     nothing does;
//   - when the text does not parse, the closure DECLINES for the rest of the
//     round and the tools' own analysis runs, as it did before: funcreach is
//     what repairs such a text (doc/surveys/REACHABILITY.md §6).
var closureOn = os.Getenv("WHIM_CLOSURE") == "1"

// closureRounds is the ceiling with the switch on.  A deferred entity costs a
// round, so the fixpoint is further away; exceeding it is still a failure.
const closureRounds = 40

// closureRound is one round's closure: parsed once per text, declined for the
// rest of the round once a text does not parse.
type closureRound struct {
	path     string
	declined string
	sha      string
	c        *reach.Closure
	tally    *closureTally
}

// closureTally is a whole sweep's account, printed once at its end.
type closureTally struct {
	declined int
	deleted  map[string]int // by entity kind
	beyond   [3]int         // typereach, deadfields, deadenums: deleted, the old analysis would not have
	short    [3]int         // the old analysis would have deleted, the closure did not
}

func newTally() *closureTally { return &closureTally{deleted: map[string]int{}} }

func (t *closureTally) line() string {
	return fmt.Sprintf("  closure      declined %d rounds; deleted %s; beyond the old analysis "+
		"typereach %d deadfields %d deadenums %d; the old analysis alone typereach %d deadfields %d deadenums %d",
		t.declined, kinds(t.deleted), t.beyond[0], t.beyond[1], t.beyond[2], t.short[0], t.short[1], t.short[2])
}

func kinds(m map[string]int) string {
	var parts []string
	for _, k := range []string{"T", "S", "E", "M", "N"} {
		parts = append(parts, fmt.Sprintf("%s %d", k, m[k]))
	}
	return strings.Join(parts, ", ")
}

// get is the closure of cur, or nil once the round has declined.
func (r *closureRound) get(cur []byte) *reach.Closure {
	if r.declined != "" {
		return nil
	}
	sha := digest(cur)
	if sha == r.sha && r.c != nil {
		return r.c
	}
	if err := os.WriteFile(r.path, cur, 0o644); err != nil {
		r.declined = err.Error()
		r.tally.declined++
		return nil
	}
	ast, err := ccx.Parse(r.path)
	if err != nil {
		why := err.Error()
		if i := strings.IndexByte(why, '\n'); i >= 0 {
			why = why[:i]
		}
		r.declined = why
		r.tally.declined++
		return nil
	}
	r.sha, r.c = sha, reach.Analyze(ast, r.path, cur)
	return r.c
}

func (r *closureRound) declinedSay() string {
	return fmt.Sprintf("  closure declined, the text does not parse (%s), so the old analysis ran: ", r.declined)
}

// refusals counts entities by the guard that refused them.
type refusals map[string]int

func (f refusals) String() string {
	if len(f) == 0 {
		return "none"
	}
	var ks []string
	for k := range f {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	var parts []string
	for _, k := range ks {
		parts = append(parts, fmt.Sprintf("%d %s", f[k], k))
	}
	return strings.Join(parts, ", ")
}

// Why the guards refuse, as the lines say it.
const (
	refUnresolved = "named where the closure cannot place it"
	refPositional = "initialised by position"
	refRecover    = "members while ml_recover is defined"
	refNoEntity   = "definitions the closure has no entity in"
	refNested     = "member declarations holding a definition"
	refLine       = "member declarations sharing a line"
	refSharedDecl = "sharing a declaration with a kept member"
	refEmpty      = "would empty their struct"
	refDeferred   = "deferred: something still in the text refers to them"
	refCast       = "members held: their struct is punned through a pointer cast"
)

func byStart(c *reach.Closure) []*reach.Entity {
	var ents []*reach.Entity
	for _, e := range c.Entities {
		if _, en := e.Span(); en > 0 {
			ents = append(ents, e)
		}
	}
	sort.Slice(ents, func(i, j int) bool {
		a, _ := ents[i].Span()
		b, _ := ents[j].Span()
		return a < b
	})
	return ents
}

// within is every entity whose span starts in [lo, hi).
func within(ents []*reach.Entity, lo, hi int) []*reach.Entity {
	i := sort.Search(len(ents), func(i int) bool { s, _ := ents[i].Span(); return s >= lo })
	var out []*reach.Entity
	for ; i < len(ents); i++ {
		if s, _ := ents[i].Span(); s >= hi {
			break
		}
		out = append(out, ents[i])
	}
	return out
}

var forwardRe = regexp.MustCompile(`^\s*(?:struct|union|enum)\s+(\w+)\s*;\s*$`)

// typereach: every top-level type definition typereach can delete whose
// entities the closure all left unreachable, deleted by dead.DeleteDefs.
func (r *closureRound) typereach(cur []byte) ([]byte, string, error) {
	c := r.get(cur)
	if c == nil {
		out, line, err := runTypereach(cur)
		return out, r.declinedSay() + line, err
	}
	b := cutil.Blank(cur)
	defs := dead.Definitions(cur, b)
	_, oldDead := dead.TypeReach(cur)
	ents := byStart(c)
	unres := c.Unresolved()
	ref := refusals{}

	groups := map[int][]*reach.Entity{}
	forward := map[int]*reach.Entity{}
	considered := 0
	for i, d := range defs {
		if !d.Deletable {
			continue
		}
		g := within(ents, d.Start, d.End)
		if len(g) == 0 {
			if m := forwardRe.FindSubmatch(cur[d.Start:d.End]); m != nil {
				if t := c.Tag(string(m[1])); t != nil && !t.Reachable() {
					forward[i] = t
				}
				continue
			}
			ref[refNoEntity]++
			continue
		}
		live := false
		for _, e := range g {
			live = live || e.Reachable()
		}
		if live {
			continue
		}
		considered++
		why := ""
		for _, e := range g {
			switch {
			case unres[e.Name] || unres[e.ID]:
				why = refUnresolved
			case e.Kind == "M" && c.Moves(e) > 0:
				why = refPositional
			}
		}
		if why != "" {
			ref[why]++
			continue
		}
		groups[i] = g
	}
	// Deferral, to a fixpoint: a definition goes only with every referrer
	// of every entity in it.
	in := map[string]bool{}
	for _, g := range groups {
		for _, e := range g {
			in[e.Key()] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for i, g := range groups {
			ok := true
			for _, e := range g {
				for _, rr := range c.Referrers(e) {
					if !in[rr.Key()] {
						ok = false
					}
				}
			}
			if !ok {
				for _, e := range g {
					delete(in, e.Key())
				}
				delete(groups, i)
				ref[refDeferred]++
				changed = true
			}
		}
	}
	var idx []int
	for i, g := range groups {
		idx = append(idx, i)
		for _, e := range g {
			r.tally.deleted[e.Kind]++
		}
	}
	for i, t := range forward {
		if in[t.Key()] {
			idx = append(idx, i)
		}
	}
	sort.Ints(idx)
	old := map[int]bool{}
	for _, i := range oldDead {
		old[i] = true
	}
	now := map[int]bool{}
	beyond, short := 0, 0
	for _, i := range idx {
		now[i] = true
		if !old[i] {
			beyond++
		}
	}
	for _, i := range oldDead {
		if !now[i] {
			short++
		}
	}
	r.tally.beyond[0] += beyond
	r.tally.short[0] += short
	line := fmt.Sprintf("  typereach    closure: %d of %d type definitions deleted (%d beyond typereach's own, %d it alone would); refused %s",
		len(idx), len(defs), beyond, short, ref)
	if len(idx) == 0 {
		return cur, line, nil
	}
	return dead.DeleteDefs(cur, defs, idx), line, nil
}

// deadfields: every member the closure left unreachable, its declaration's
// lines deleted by dead.DeleteFields.
func (r *closureRound) deadfields(cur []byte) ([]byte, string, error) {
	c := r.get(cur)
	if c == nil {
		out, line, err := runDeadfields(cur)
		return out, r.declinedSay() + line, err
	}
	var dead0 []*reach.Entity
	for _, e := range c.Unreachable() {
		if e.Kind == "M" {
			dead0 = append(dead0, e)
		}
	}
	oldCands, recoverable := dead.DeadFields(cur)
	if recoverable || c.ReadsSwap() {
		return cur, fmt.Sprintf("  deadfields   closure: not while ml_recover() can read a swap file: "+
			"a struct layout is still a disk format; refused %d %s", len(dead0), refRecover), nil
	}
	ents := byStart(c)
	unres := c.Unresolved()
	ref := refusals{}
	// The closure holds them (reach.ClassCast), so they are reached and never
	// candidates; counted here so that the line says so.
	if n := len(c.Pun.Held); n > 0 {
		ref[refCast] = n
	}
	type decl struct{ a, b, la, lb int }
	cand := map[string]bool{}
	declOf := map[string]decl{}
	for _, e := range dead0 {
		a, b, _ := c.MemberDecl(e)
		switch {
		case unres[e.Name] || unres[e.ID]:
			ref[refUnresolved]++
			continue
		case c.Moves(e) > 0:
			ref[refPositional]++
			continue
		case b <= a:
			ref[refNoEntity]++
			continue
		}
		nested := false
		for _, o := range within(ents, a, b) {
			if s, _ := o.Span(); !(o.Kind == "M" && s == a) {
				nested = true
			}
		}
		if nested {
			ref[refNested]++
			continue
		}
		la := bytes.LastIndexByte(cur[:a], '\n') + 1
		lb := len(cur)
		if k := bytes.IndexByte(cur[b:], '\n'); k >= 0 {
			lb = b + k + 1
		}
		if len(bytes.TrimSpace(cur[la:a])) > 0 || len(bytes.TrimSpace(cur[b:lb])) > 0 {
			ref[refLine]++
			continue
		}
		cand[e.Key()] = true
		declOf[e.Key()] = decl{a, b, la, lb}
	}
	// Three rules to a fixpoint: a declaration goes whole or not at all, a
	// struct is never emptied, and a member goes only with every referrer.
	byDecl := map[int][]*reach.Entity{}
	for _, e := range dead0 {
		a, _, _ := c.MemberDecl(e)
		byDecl[a] = append(byDecl[a], e)
	}
	drop := func(e *reach.Entity, why string) bool {
		if !cand[e.Key()] {
			return false
		}
		delete(cand, e.Key())
		ref[why]++
		return true
	}
	for changed := true; changed; {
		changed = false
		for _, e := range dead0 {
			if !cand[e.Key()] {
				continue
			}
			_, _, n := c.MemberDecl(e)
			a, _, _ := c.MemberDecl(e)
			whole := len(byDecl[a]) == n
			for _, o := range byDecl[a] {
				whole = whole && cand[o.Key()]
			}
			if !whole {
				changed = drop(e, refSharedDecl) || changed
				continue
			}
			empty := true
			for _, s := range c.Siblings(e) {
				empty = empty && cand[s.Key()]
			}
			if empty {
				// deadfields' rule: every candidate of the struct stays.
				for _, s := range c.Siblings(e) {
					changed = drop(s, refEmpty) || changed
				}
				continue
			}
			for _, rr := range c.Referrers(e) {
				if !cand[rr.Key()] {
					changed = drop(e, refDeferred) || changed
					break
				}
			}
		}
	}
	var cands []dead.FieldCand
	seen := map[int]bool{}
	now := map[int]bool{}
	for _, e := range dead0 {
		if !cand[e.Key()] {
			continue
		}
		r.tally.deleted["M"]++
		d := declOf[e.Key()]
		if seen[d.la] {
			continue
		}
		seen[d.la] = true
		now[d.la] = true
		cands = append(cands, dead.FieldCand{Start: d.la, End: d.lb, Name: e.Name})
	}
	beyond, short := 0, 0
	old := map[int]bool{}
	for _, o := range oldCands {
		old[o.Start] = true
		if !now[o.Start] {
			short++
		}
	}
	for s := range now {
		if !old[s] {
			beyond++
		}
	}
	r.tally.beyond[1] += beyond
	r.tally.short[1] += short
	line := fmt.Sprintf("  deadfields   closure: %d of %d unreachable members deleted (%d beyond deadfields' own, %d it alone would); refused %s",
		len(cands), len(dead0), beyond, short, ref)
	if len(cands) == 0 {
		return cur, line, nil
	}
	return dead.DeleteFields(cur, cands), line, nil
}

// deadenums: every enumerator the closure left unreachable, deleted by
// dead.AnalyseEnumsWith -- the same edits, the same pinning.
func (r *closureRound) deadenums(cur []byte, vals string) ([]byte, string, error) {
	c := r.get(cur)
	if c == nil {
		out, line, err := runDeadenums(r.path, cur, vals, dead.AnalyseEnums)
		return out, r.declinedSay() + line, err
	}
	unres := c.Unresolved()
	ref := refusals{}
	cand := map[string]bool{}
	var dead0 []*reach.Entity
	for _, e := range c.Unreachable() {
		if e.Kind != "N" {
			continue
		}
		if unres[e.Name] || unres[e.ID] {
			ref[refUnresolved]++
			continue
		}
		dead0 = append(dead0, e)
		cand[e.Key()] = true
	}
	for changed := true; changed; {
		changed = false
		for _, e := range dead0 {
			if !cand[e.Key()] {
				continue
			}
			for _, rr := range c.Referrers(e) {
				if !cand[rr.Key()] {
					delete(cand, e.Key())
					ref[refDeferred]++
					changed = true
					break
				}
			}
		}
	}
	names := map[string]bool{}
	for _, e := range dead0 {
		if cand[e.Key()] {
			names[e.Name] = true
		}
	}
	// What the old rule would call dead, for the comparison only.
	counts := map[string]int{}
	for _, m := range identWord.FindAll(cutil.Blank(cur), -1) {
		counts[string(m)]++
	}
	beyond, short := 0, 0
	for n := range names {
		if counts[n] != 1 {
			beyond++
		}
	}
	for _, e := range c.Entities {
		if e.Kind == "N" && counts[e.Name] == 1 && !names[e.Name] {
			short++
		}
	}
	analyse := func(text []byte, v map[string]string) ([]dead.Edit, dead.EnumStats) {
		return dead.AnalyseEnumsWith(text, v, func(n string) bool { return names[n] })
	}
	out, line, err := runDeadenums(r.path, cur, vals, analyse)
	if err == nil && !bytes.Equal(out, cur) {
		// The enumerators the edits actually removed, which is fewer than
		// names when a run was kept or an enum was stuck.
		after := map[string]int{}
		for _, m := range identWord.FindAll(cutil.Blank(out), -1) {
			after[string(m)]++
		}
		for n := range names {
			if after[n] == 0 {
				r.tally.deleted["N"]++
			}
		}
		r.tally.beyond[2] += beyond
		r.tally.short[2] += short
	} else if err == nil {
		r.tally.short[2] += short
	}
	line = strings.Replace(line, "  deadenums    ", fmt.Sprintf(
		"  deadenums    closure (%d beyond the mention count, %d it alone would; refused %s): ", beyond, short, ref), 1)
	return out, line, err
}

var identWord = regexp.MustCompile(`\b[A-Za-z_]\w*\b`)
