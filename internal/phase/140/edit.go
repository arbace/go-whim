package p140

// Whim phase 140 -- highlight groups are found in their array.  See GOAL.md.
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

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim140", Edit) }

// W140LookupBody is syn_name2id_len()'s Body after this phase, inside its
// braces, exported so the check requires the identical text.
const W140LookupBody = `    char_u      name_u[MAX_SYN_NAME + 1];
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
    return 0;`

// Whim140 finds a highlight group by name in the array that holds it.
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
// table, and the sweep takes the hash table code with it.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "hlname", W: w}
	Out, _, err := cutil.ReplaceBody(text, "syn_name2id_len", W140LookupBody)
	if err != nil {
		return nil, p.Die("syn_name2id_len: %v", err)
	}
	p.Say("syn_name2id_len() finds the group whose upper-cased name is the name, and gives its index + 1")
	Out, _, err = cutil.ReplaceBody(Out, "syn_unadd_group", "    --highlight_ga.ga_len;")
	if err != nil {
		return nil, p.Die("syn_unadd_group: %v", err)
	}
	p.Say("syn_unadd_group() drops the last group")
	steps := []struct {
		Old, New, What string
		n              int
	}{
		{"        hash_init(&highlight_ht);\n        highlight_ht_inited = true;\n", "", "syn_add_group() starts no table", 1},
		{`    {
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
`, "saves the upper-cased name on its own, with no id beside it", 1},
		{"    hash_add(&highlight_ht, name_up, \"highlight\");\n", "", "and adds it to no table", 1},
	}
	for _, s := range steps {
		if Out, err = p.Literal(Out, s.Old, s.New, s.What, s.n); err != nil {
			return nil, err
		}
	}
	return Out, nil
}
