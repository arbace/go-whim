# Phase 140 — highlight groups are found in their array

Every highlight group's upper-cased name was kept twice: as `sg_name_u` in its
`hl_group_T`, and as the key of an entry in `highlight_ht`, allocated inside
an `hlname_T` beside the group's id. `syn_name2id_len()` found the key in the
table and got the id back by subtracting the key's offset in `hlname_T`. That
is the `container_of` the Go transpilation kept an owner registry for
(finding 3, its second half; phase 133 was the first).

A group is only added after the lookup of its name failed, so names are
unique. The group whose `sg_name_u` is the name is therefore the one the
table found, and its id is its index + 1, which is what `hn_id` held. So:
- the lookup scans the array;
- the name is saved and upper-cased on its own;
- `syn_unadd_group()` just decrements the length.

`highlight_ht` was the last hash table. The sweep takes `hashtab_T`,
`hashitem_T` and all of `hash_*`: the file loses 344 lines. The scalar
`hash_T` typedef stays, because the sweep doesn't follow scalar typedefs.

**Declared delta: nothing.** The check proves the premise from the input:
- `syn_add_group()` has one call, taken only when the lookup failed;
- the key is the group's `sg_name_u`;
- the id is `ga_len + 1` just before the length goes up;
- `highlight_ht` is the only table.

Its probes are a new group looked up in another case, a group a failed
`:hi` added and took back, and a link; each control moves.
