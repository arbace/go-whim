package p072

// Whim phase 72 (formerly 146) -- a memline node names its block.  See GOAL.md.
//
// A tree node was a block beginning with a tagged header, and the tree cast
// the header to the block its tag named (internal/gen/FINDINGS.md, 4).  The header
// becomes the node, holding its tag and a typed pointer to its block, and each
// of the 19 casts reads that pointer.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim72", Edit) }

const (
	W72NewData = `    DATA_BL     *dp;
    bhdr_T      *hp;

    dp =  (DATA_BL *)alloc_clear(sizeof(DATA_BL)) ;
    if (dp == nullptr)
    {
        return nullptr;
    }
    hp =  (bhdr_T *)alloc_clear(sizeof(bhdr_T)) ;
    if (hp == nullptr)
    {
        return nullptr;
    }

    hp->bh_id = (('d' << 8) + 'a');
    hp->bh_data = dp;
    dp->db_line_count = 0;

    return hp;
`
	W72NewPtr = `    PTR_BL      *pp;
    bhdr_T      *hp;

    pp =  (PTR_BL *)alloc_clear(sizeof(PTR_BL)) ;
    if (pp == nullptr)
    {
        return nullptr;
    }
    hp =  (bhdr_T *)alloc_clear(sizeof(bhdr_T)) ;
    if (hp == nullptr)
    {
        return nullptr;
    }

    hp->bh_id = (('p' << 8) + 't');
    hp->bh_ptr = pp;
    pp->pb_count = 0;

    return hp;
`
)

// Edit makes a memline node name its block.
//
// A node of the text's tree was a PTR_BL or a DATA_BL whose first member was a
// bhdr_T holding a tag; the tree held bhdr_T pointers and cast them to the
// block the tag named -- struct prefix inheritance, which the Go transpilation
// kept a registry of blocks for (internal/gen/FINDINGS.md, 4).  The header becomes the
// node: its tag and a pointer to its block, of the block's own type, set when
// the two are allocated.  Every cast is then a field -- (DATA_BL *)(hp) is
// hp->bh_data -- and the blocks no longer begin with a header.  Every cast
// was of a node already known not to be NULL, so reading the field where the
// cast was changes nothing.  The headers the blocks began with, pb_hdr and
// db_hdr, are named by nothing then, and the collection takes them.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): the header's definition written
// anew by FRAG (a member is no FRAG spot), with the two bodies in one unit;
// the assertion's term and string by form; and each cast to a block, found by
// its form, made the field by FRAG -- the parentheses the text's cast stood in, where
// the printer writes them around a cast that is an operand of `->`, kept as
// the text kept them: dropped with the parenthesised PTR_BL cast it matched
// whole, kept around a DATA_BL field it matched inside them.  History keeps
// the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("memnode", e, w)
	hdr := v.One("(struct block_hdr (bh_id short_u))", "struct block_hdr")
	if v.Failed() {
		return v.Done()
	}
	if _, err := e.SpliceC(graph.Frag{At: e.SpotOf(hdr), Src: "struct block_hdr\n{\n    short_u     bh_id;\n    struct pointer_block *bh_ptr;\n    struct data_block *bh_data;\n};\n"},
		graph.Frag{At: e.SpotBody(e.Defn("ml_new_data")), Src: W72NewData},
		graph.Frag{At: e.SpotBody(e.Defn("ml_new_ptr")), Src: W72NewPtr}); err != nil {
		v.Die("a node is its tag and a pointer to its block -- %v", err)
		return v.Done()
	}
	v.Say("a node is its tag and a pointer to its block")
	v.Rewrite("(+ 16 (* DB_LINE_MAX (sizeof-type DATA_LN)))", "(+ 8 (* DB_LINE_MAX (sizeof-type DATA_LN)))", 1, "a leaf is its count and its records")
	v.RespellString(`"a leaf is its tag, its count and its records"`, `"a leaf is its count and its records"`, 1, "and says so")
	v.Say("ml_new_data() allocates the block and its node, and the node names the block")
	v.Say("and so does ml_new_ptr()")
	// field is each cast of a node to the block T, the field m: parens says
	// whether it stood in parentheses (its own, or the printer's around an
	// operand of ->), and dropParens whether the replacement drops them
	var frags []graph.Frag
	field := func(t, m string) (inParens, bare int) {
		if v.Failed() {
			return
		}
		p, _ := clisp.Pattern("(cast (ptr " + t + ") (paren ?x))")
		for _, c := range v.Find("(cast (ptr " + t + ") (paren ?x))") {
			b, _ := graph.Match(p, c)
			par := e.Parent(c)
			explicit := par.Is("paren")
			implicit := par.Is("->") && par.Kids[1] == c
			at, src := c, "$x->"+m
			switch {
			case t == "PTR_BL" && explicit:
				at = par
			case t == "DATA_BL" && implicit:
				src = "($x->" + m + ")"
			}
			// a field that is the operand of -> unparenthesised is one
			// selection chain with it, as cc reads the text
			if q := e.Parent(at); !strings.HasPrefix(src, "(") && q.Is("->") && q.Kids[1] == at {
				var ms []string
				for _, k := range q.Kids[2:] {
					ms = append(ms, k.Atom)
				}
				at, src = q, src+"->"+strings.Join(ms, "->")
			}
			if explicit || implicit {
				inParens++
			} else {
				bare++
			}
			frags = append(frags, graph.Frag{At: e.SpotOf(at), Src: src, Holes: graph.Bindings{"x": b["x"]}})
		}
		return
	}
	pp, pb := field("PTR_BL", "bh_ptr")
	v.Expect(pp == 1 && pb == 7, "%d parenthesised and %d other casts of a node to its pointer block, where this phase was written against 1 and 7", pp, pb)
	v.Say("a node cast to its pointer block reads bh_ptr, the parenthesised cast")
	v.Say("and the other seven")
	dp, db := field("DATA_BL", "bh_data")
	v.Expect(dp+db == 11, "%d casts of a node to its data block, where this phase was written against 11", dp+db)
	if !v.Failed() {
		if _, err := e.SpliceC(frags...); err != nil {
			v.Die("a node cast to its block -- %v", err)
			return v.Done()
		}
	}
	v.Say("a node cast to its data block reads bh_data, eleven times")
	k := v.Count("(cast (ptr PTR_BL) _)") + v.Count("(cast (ptr DATA_BL) _)") + v.Count("(cast (ptr bhdr_T) _)")
	v.Expect(k == 4, "%d casts to a block or a node remain, where the four allocations -- two blocks, two nodes -- are 4", k)
	return v.Done()
}
