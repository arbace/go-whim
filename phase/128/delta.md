128 DECLARES NOTHING AT ALL, AND IT IS 127'S KIND -- phases 97 and 98's, the weakest on
this list.  The code changes, the binary moves, and the claim is that a replacement
does what the thing it replaces did.  There is no `cmp` to be had: a node stops being
a `bhdr_T` of four members with a 4,096-byte page hanging off a `char_u *bh_data` and
becomes ONE allocation at its own size -- 1,040 bytes for a leaf, 4,088 for a branch --
with the header as its first member and its tag; `memfile_T` has nothing left to hold
and is gone; and `ml_root` answers "does this buffer have a memline" where `ml_mfp`
did, in all fifteen functions that asked and in ml_delete_int, which asked it through
a local copy.  Thirty identifiers leave the file and five arrive.

SO THE RECORDINGS ARE THE FLOOR AND NOT THE EVIDENCE.  Two full tools/zrecord.sh
recordings are byte-identical to the input's across all 118 cases and four tables, and
what carries the phase is twelve builds of its own output with one thing changed: nine
that MUST move a recording and do -- the leaf tagged wrong 118/118, the branch tagged
wrong 118, the leaf test inverted 118, the root split forgetting its count 1, the root
split copying no entries 1, the branch capacity bound off by one 1, a branch allocated
at a leaf's size 6, a leaf allocated at half its size 16, the leaf capacity bound off
by one 4 -- and three that must not, each a measured statement about what the corpus
cannot see rather than a control that failed:

  fanout   PB_COUNT_MAX = 511, which is what an 8-byte PTR_EN would give.  0 of 118,
           AND THAT IS THE POINT: the same binary takes MLSPLITPTR, MLSPLITROOT and
           MLDEEP from 1 to 0.  391 data blocks is more than 255 and less than 511, so
           narrowing the entry would take the root split out of the corpus WITHOUT
           MOVING ONE RECORD.  The control is run under phase 123's instrument as well
           as under the recording, because the recording is exactly what cannot see it.
  noclear  alloc() for alloc_clear() in ml_new_data.  The host's arena is a bump
           pointer over fresh pages, so the memory is already zero -- a fact about
           this host, not a promise the core may rest on, and ml_open's error path
           does rest on it: it walks a root whose one entry has not been filled in.
  nofree   ml_free_tree() walking the tree and freeing nothing, so a closed buffer
           keeps every node it had.  0 of 118 because GOALS.md's charter says
           host_free() returns without doing anything: what a core gives back is
           unobservable by construction (phase 124).

THE CORPUS REACHES EXACTLY WHAT IT REACHED, case by case and not only in total:
MLSPLITDATA 16, MLSPLITPTR 1, MLSPLITROOT 1, MLIDXNZ 16 and MLDEEP 1, every marker in
the same cases on both sides, with 0 of the 102 screen cases reaching any of them.
`mem_deep_jumps` is the one case of sixteen that splits the root, on both sides.

AND SIZEOF(PTR_EN) IS 16 EITHER SIDE, COMPILED AND NOT READ.  `pb_count_max` was
computed per block as (4096 - 8) / sizeof(PTR_EN) and is the enumerator PB_COUNT_MAX
now; the two agree at 255 only because the entry is still `{bhdr_T *pe_block;
linenr_T pe_line_count;}`.  A static_assert is appended to the `make editor.c` cut of
the OUTPUT and of the INPUT and both must compile, with `== 8` required to fail
against both so that the assertion is an assertion.  The new struct has the same
offset by construction -- a two-byte tag and a two-byte count where three shorts were
-- so 255 is the number the input computes and not a number chosen.

PHASE 127'S PREDICTION, MEASURED IN THE SAME RUN.  That phase wrote that
allocating a block at its own size "would make an off-by-one in the capacity bound
VISIBLE, which today it is not", and recorded its own `cap` control at 0 of 118.  The
identical edit is made here to the input and to the output and both are built and
recorded: 0 of 118 on the input, 4 of 118 here.  A leaf is 1,040 bytes of its own
allocation now and was 1,040 bytes of a 4,096-byte page, so the 65th record used to
land in the page's spare room and now lands past the end.

AND THE COST, which is the one thing about this phase a byte-identical recording says
nothing about.  Every one of the sixteen memline sessions asks the host for LESS than
the input: the heaviest, mem_deep_jumps, asks 200,720,256 bytes where it asked
201,927,792, -0.6%, and the biggest fall is mem_join_split at -9.2%.  The difference
is the node overhead and it is arithmetic: 391 data blocks x (4,128 - 1,040) is
1,207,408 of the 1,207,536 bytes that go.  Per line a leaf costs 16.25 bytes where it
cost 64.5.  With nothing freed (phase 124) that is a session's TRAFFIC and not its live
data.

WHAT THE PHASE DOES NOT BUY.  The tree is a counted tree of nodes now and there is no
memfile, no page and no block header -- but a leaf is still a fixed array of
DB_LINE_MAX records and a branch a fixed array of PB_COUNT_MAX entries, so a node is
sized for its worst case and not for what it holds.  That is a representation
question and not a paging one, and it is nobody's phase yet.
