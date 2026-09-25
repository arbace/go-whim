package p146

// Whim phase 146 -- a memline node names its block.  See GOAL.md.
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
	"fmt"
	"io"
	"regexp"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim146", Edit) }

// W146NewData and W146NewPtr are the two allocators' bodies after this phase,
// inside their braces: the block and its header, each allocated, the header
// naming the block.
const (
	W146NewData = `    DATA_BL     *dp;
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

    return hp;`
	W146NewPtr = `    PTR_BL      *pp;
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

    return hp;`
)

var (
	w146Paren = regexp.MustCompile(`\(\((PTR_BL|DATA_BL) \*\)\(([\w.>-]+)\)\)`)
	w146Cast  = regexp.MustCompile(`\((PTR_BL|DATA_BL) \*\)\(([\w.>-]+)\)`)
)

// Whim146 makes a memline node name its block.
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
// db_hdr, are named by nothing then, and the sweep takes them.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "memnode", W: w}
	var err error
	steps := []struct{ Old, New, What string }{
		{"struct block_hdr\n{\n    short_u     bh_id;\n};\n", "struct block_hdr\n{\n    short_u     bh_id;\n    struct pointer_block *bh_ptr;\n    struct data_block *bh_data;\n};\n", "a node is its tag and a pointer to its block"},
		{"static_assert(sizeof(DATA_BL) == 16 + DB_LINE_MAX * sizeof(DATA_LN), \"a leaf is its tag, its count and its records\");",
			"static_assert(sizeof(DATA_BL) == 8 + DB_LINE_MAX * sizeof(DATA_LN), \"a leaf is its count and its records\");", "a leaf is its count and its records"},
	}
	for _, s := range steps {
		if text, err = p.Literal(text, s.Old, s.New, s.What, 1); err != nil {
			return nil, err
		}
	}
	if text, _, err = cutil.ReplaceBody(text, "ml_new_data", W146NewData); err != nil {
		return nil, p.Die("ml_new_data: %v", err)
	}
	if text, _, err = cutil.ReplaceBody(text, "ml_new_ptr", W146NewPtr); err != nil {
		return nil, p.Die("ml_new_ptr: %v", err)
	}
	p.Say("ml_new_data() and ml_new_ptr() allocate the block and its node, and the node names the block")
	field := func(kind string) string {
		if kind == "PTR_BL" {
			return "bh_ptr"
		}
		return "bh_data"
	}
	n := 0
	text = w146Paren.ReplaceAllFunc(text, func(m []byte) []byte {
		s := w146Paren.FindSubmatch(m)
		n++
		return []byte(string(s[2]) + "->" + field(string(s[1])))
	})
	text = w146Cast.ReplaceAllFunc(text, func(m []byte) []byte {
		s := w146Cast.FindSubmatch(m)
		n++
		return []byte(string(s[2]) + "->" + field(string(s[1])))
	})
	if n != 19 {
		return nil, p.Die("%d casts from a node to its block, and this phase was written against 19", n)
	}
	p.Say(fmt.Sprintf("the %d casts from a node to its block read the node's pointer", n))
	if k := len(regexp.MustCompile(`\((PTR_BL|DATA_BL|bhdr_T) \*\)`).FindAll(text, -1)); k != 4 {
		return nil, p.Die("%d casts to a block or a node remain, where the four allocations -- two blocks, two nodes -- are 4", k)
	}
	return text, nil
}
