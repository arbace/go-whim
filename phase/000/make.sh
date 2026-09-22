#!/bin/sh
# Whim phase 0 -- seed, and prove the copy is a copy.  See GOAL.md.
#
# Usage: phase/000/make.sh <work-dir>       (run from the repository root)
#
# whim-vim.c begins as slim-vim.c and this phase's only job is to establish
# that.  It matters because every phase after it is measured as a delta: if the
# seed is not identical, every later report is against the wrong thing, and the
# error would look like whatever that phase happened to do.
#
# There is nothing here to make faster.  It is a `cmp` and a build, and it
# exists so that the first boundary means something.
set -eu

work=${1:?usage: phase/000/make.sh <work-dir>}
f="$work/whim-vim.c"

if ! cmp -s "$f" slim-vim.c; then
    echo "  seed         DIFFERS from slim-vim.c -- the pipeline's input is not"
    echo "               what it claims, and every later delta would be measured"
    echo "               against the wrong file."
    exit 1
fi
echo "  seed         identical to slim-vim.c, $(grep -c '' "$f") lines"

make -C "$work" clean >/dev/null 2>&1 || true
if make -C "$work" >/dev/null 2>&1; then
    echo "  build        ok, $(stat -c%s "$work/whim-vim") bytes"
else
    echo "  build        FAILED -- rerun by hand: make -C $work"
    exit 1
fi

# --- the baselines every whim delta is measured against -----------------------
# In arbace/slim-vim these were recorded by the slim pipeline's Phase 1, the last
# phase there that changes behaviour, and every later slim phase was held to them
# -- so they describe slim-vim.c exactly.  This repository has no slim pipeline:
# slim-vim.c is its input, fetched and immutable.  So they are recorded HERE, from
# the seed, the way phase 83 records the second set from q82: three runs that
# must be identical, and an existing set compared and never overwritten.  That is
# not re-recording from this pipeline's own output, which would agree by
# construction; nothing whim produces is on the recording side.
#
# MEASURED before this was written: the Go harnesses below, run on slim-vim.c's
# binary, reproduce arbace/slim-vim's Python-recorded baselines byte for byte --
# all 67 behaviour cases, the 19-row terminal table and all 600 Ex-command rows.
base=.reference/baselines
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
for r in 1 2 3; do
    mkdir -p "$tmp/run$r"
    tools/st.sh behaviour "$work/whim-vim" "$tmp/run$r/behaviour" >/dev/null &
    p1=$!
    tools/st.sh termcheck "$work/whim-vim" "$tmp/run$r/ref-term.txt" >/dev/null &
    p2=$!
    tools/st.sh exsweep "$work/whim-vim" "$f" "$tmp/run$r/ref-exsweep.txt" >/dev/null &
    p3=$!
    wait $p1 && wait $p2 && wait $p3 || { echo "  baselines    a harness failed on run $r"; exit 1; }
    if [ "$r" != 1 ] && ! diff -r "$tmp/run1" "$tmp/run$r" >/dev/null; then
        echo "  baselines    run $r differs from run 1 -- not deterministic, not a baseline:"
        diff -rq "$tmp/run1" "$tmp/run$r" | head -10 | sed 's/^/                 /'
        exit 1
    fi
done
cases=$(ls "$tmp/run1/behaviour" | grep -c '')
rows=$(grep -c '' "$tmp/run1/ref-exsweep.txt")
terms=$(grep -c '' "$tmp/run1/ref-term.txt")
if [ -d "$base" ]; then
    if ! diff -r "$base" "$tmp/run1" >/dev/null; then
        echo "  baselines    DIFFER from the recorded $base:"
        diff -rq "$base" "$tmp/run1" | head -10 | sed 's/^/                 /'
        echo "               slim-vim.c is this pipeline's immutable input, so the same"
        echo "               input recorded differently: a harness changed, or the input"
        echo "               did (make fetches it).  Name which before removing $base."
        exit 1
    fi
    echo "  baselines    match $base: $cases behaviour cases, $rows Ex commands, $terms terminals, 3 identical runs"
else
    mkdir -p .reference
    rm -rf "$base.part"
    cp -r "$tmp/run1" "$base.part"
    mv "$base.part" "$base"
    echo "  baselines    recorded $base from slim-vim.c: $cases behaviour cases, $rows Ex commands, $terms terminals, 3 identical runs"
fi
