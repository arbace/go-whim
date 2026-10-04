# Phase 72 — a memline node names its block

*Formerly phase 146. The other phase numbers in this file are the old numbering,
as it was written: `doc/PHASES.md` maps them.*

A node of the text's tree was either a `PTR_BL` or a `DATA_BL`, and each began
with a `bhdr_T` holding a tag. The tree held `bhdr_T` pointers and cast each
to the block its tag named: struct prefix inheritance. Phase 135 took the
regexp half of this out; the Go transpilation kept a registry of blocks for
this half (finding 4).

Now the header is the node. It holds its tag and a pointer to its block, of
the block's own type, set when the two are allocated. Each of the 19 casts
becomes a field read, so `(DATA_BL *)(hp)` is `hp->bh_data`, and the blocks no
longer begin with a header. The `static_assert` on a leaf's size loses the
header's 8 bytes: a leaf is its count and its records.

Casting a NULL gave NULL, but reading a field of one crashes, so this is only
safe because every cast was of a node known to exist. The check proves it: it
classifies every read of `bh_ptr` or `bh_data` by what shows its node is
there. That is a NULL test or a dereference of its tag earlier in the
function, the tree's stack, or an assignment from such a node.

**Declared delta: nothing.** The memline corpus in the recording exercises the
tree. The check's probe makes 300 lines, deletes 100 and undoes the deletion,
splitting leaves and making pointer blocks; one line fewer moves the output.

Since step 5 (`doc/GRAPH-MIGRATION.md`, B3f) it runs on the graph: the header's definition, the two bodies and the nineteen fields are written by FRAG, the assertion's term and string by form, on `crefactor/graph`'s editor and verbs, its report the text version's, and `whim-build-check` holds it to the bytes the text version made (which is in history, `16717ab` and before).
