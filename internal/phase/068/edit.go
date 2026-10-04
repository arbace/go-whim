package p068

// Whim phase 68 (formerly 140) -- highlight groups are found in their array.  See GOAL.md.
//
// syn_name2id_len() found a group's id by subtracting a key's offset in
// hlname_T (internal/gen/FINDINGS.md, 3).  Names are unique and the id is the index + 1,
// so the lookup scans highlight_ga; highlight_ht was the last hash table, and
// the sweep takes the hash table code.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim68", Edit) }

// W68LookupBody is syn_name2id_len()'s Body after this phase, inside its
// braces, exported so the check requires the identical text.
const W68LookupBody = `    char_u      name_u[MAX_SYN_NAME + 1];
    int         i;

    if (len > MAX_SYN_NAME)
    {
        len = MAX_SYN_NAME;
    }
     musl_memmove((char *)(name_u), (char *)(name), len) ;
    name_u[len] = NUL;
    vim_strup(name_u);
    for (i = 0; i < highlight_ga.ga_len; ++i)
    {
        if (musl_strcmp((char *)(((hl_group_T *)(highlight_ga.ga_data))[i].sg_name_u), (char *)(name_u)) == 0)
        {
            return i + 1;
        }
    }
    return 0;
`

// Edit finds a highlight group by name in the array that holds it.
//
// Every group's upper-cased name was kept twice: as sg_name_u in its
// hl_group_T, and as the key of an entry in highlight_ht, allocated inside an
// hlname_T beside the group's id.  syn_name2id_len() found the key in the table
// and recovered the id by subtracting the key's offset in hlname_T -- the
// container_of the Go transpilation kept an owner registry for (internal/gen/FINDINGS.md,
// 3).  A group is only added after the lookup of its name failed, so the names
// are unique, and the group whose sg_name_u is the name is the one the table
// found: its id is its index + 1, which is what hn_id held.  So the lookup
// scans the array; the name is saved and upper-cased on its own; and
// syn_unadd_group() is the length going down.  highlight_ht was the last hash
// table, and the collection takes the hash table code with it.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): the bodies and the block by
// FRAG, in one unit (Together), the table's statements cut by their forms;
// history keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("hlname", e, w)
	v.Together(func(v *graph.Verbs) {
		v.BodyC("syn_name2id_len", W68LookupBody,
			"syn_name2id_len() finds the group whose upper-cased name is the name, and gives its index + 1")
		v.BodyC("syn_unadd_group", "    --highlight_ga.ga_len;\n", "syn_unadd_group() drops the last group")
		v.InFunction("syn_add_group", func(v *graph.Verbs) {
			v.Cut("(call hash_init (addr highlight_ht))", 1, "syn_add_group() starts no table")
			v.Cut("(= highlight_ht_inited true)", 1, "and marks none started")
			v.LiteralC(`    {
        hlname_T *hn;
        int len = (int)musl_strlen((char *)(name));
        hn = alloc(__builtin_offsetof(hlname_T, hn_key) + len + 1);
        if (hn == nullptr)
        {
            return 0;
        }
        vim_strncpy(hn->hn_key, name, len);
        vim_strup(hn->hn_key);
        hn->hn_id = highlight_ga.ga_len + 1;
        name_up = hn->hn_key;
    }
`, `    name_up = vim_strsave(name);
    if (name_up == nullptr)
    {
        return 0;
    }
    vim_strup(name_up);
`, 1, "saves the upper-cased name on its own, with no id beside it")
			v.Cut(`(call hash_add (addr highlight_ht) name_up "highlight")`, 1, "and adds it to no table")
		})
	})
	return v.Done()
}
