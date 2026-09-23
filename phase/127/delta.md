127 DECLARES NOTHING AT ALL, AND IT IS THE WEAKEST KIND ON THIS LIST -- phases 97 and
98's, and no other.  The code changes, the binary moves, and the claim is that a
replacement does what the thing it replaces did.  There is no `cmp` to be had: a data
block stops being a header, an index of byte offsets and a text arena and becomes an
array of `{char_u *dl_text; colnr_T dl_len; char dl_marked;}`, so every line of every
buffer is stored somewhere else and every address in the file moves.

SO THE RECORDINGS ARE THE FLOOR AND NOT THE EVIDENCE.  Two full tools/zrecord.sh
recordings are byte-identical to the input's across all 118 cases and four tables,
and what carries the phase is eleven builds of its own output with one thing changed:
eight that MUST move a recording and do -- the text not copied 2/118, the stored
length dropped 36, a mark never set 4, the delete shifting one record too few 6, the
insert opening its gap the wrong way 8, the split moving one record too few 4, every
read taking the block's first record 52, every length short by one 38 -- and three
that must not, each a measured statement about what the corpus cannot see rather than
a control that failed:

  poison  the text a record stops owning, overwritten the moment it is replaced.
          0 of 118 IS the lifetime rule measured: nothing reads a replaced line
          through a pointer it kept.
  cap     the capacity bound off by one.  A leaf is 1,040 bytes of a 4,096-byte page,
          so the 65th record lands in the page's spare room and nothing notices --
          the bound is soft until a block is allocated at its own size, which is a
          memfile phase's to do.
  dbmax   DB_LINE_MAX = 1, a leaf per line.  The fanout is invisible to the corpus in
          both directions -- 32, 64, 128 and 1 all record identically -- which is why
          the value is chosen by REACHABILITY and not by a recording.

ONE THING THE CORPUS REACHES LESS, AND IT IS BECAUSE THE CODE IS GONE.  Phase 123's
MLBIGLINE marker -- a data block of more than one page, for a line longer than a page
-- fires in 3 of the 16 memline cases on the input and has NO ANCHOR in the output at
all: a record is a pointer, so there is no such thing to measure.  Every other marker
the corpus reaches exactly as often as before, measured in the same run with the same
instrument: MLSPLITDATA 16, MLSPLITPTR 1, MLSPLITROOT 1, MLIDXNZ 16 and MLDEEP 1,
each of them the input's own number, with 0 of the 102 screen cases reaching any of
them.

AND THE MARGIN BEHIND THAT `1` IS ONE CASE, which belongs here because it is the next
phase's constraint and not this one's.  DB_LINE_MAX = 64 makes mem_deep_jumps build
391 data blocks against the input's 321, and pb_count_max is 255 -- so one case of
sixteen splits the root, on both sides, and no other case comes near.  pb_count_max
has gone 127 -> 170 -> 255 across phases 125 and 126 while the corpus's sizes have not
moved: the same table taken on q123 had 128 reaching 1 and it reaches 0 here.  A PTR_EN
of 8 bytes would put pb_count_max at 511 and take even 64 to zero.

AND ONE COST THAT IS NOT A DIFFERENCE, measured because a per-line allocation is the
one thing about this phase a byte-identical recording says nothing about.  The
heaviest memline session asks the host for 201,927,792 bytes where the input asks
200,438,864, +0.7%, mem_deep_jumps either side.  With nothing freed (phase 124) that
is a session's TRAFFIC and not its live data.  The counter is checked against a known
answer before it is believed -- on the 233 non-memline sessions it reproduces phase
124's own published high-water to the byte, 1,734,544 -- and the bound it enforces is
proven able to fail: ml_alloc_line() over-allocating by one page a line asks
304,354,000, 1.52 times the input, against the real output's 1.007.  A quarter as
much again is the bound the check enforces.
