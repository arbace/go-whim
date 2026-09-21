#!/bin/sh
# Whim phase 83 (zero phase 0) -- where the core begins: take zero's compile line,
# prove the binary is absolutely static, and record what the editor does.
# See ZERO-GOAL.md, whose phase N is phase 83+N here.
#
# Usage: pipes/whim83.sh <work-dir>       (run from the repository root)
#
# This was the seed of a pipeline of its own, handed the committed whim-vim.c.  It
# is now the phase after 82, handed q82's tree -- whim-vim.c and whim's makefile --
# and every phase after it is measured as a delta from that tree.  So it
# establishes four things, in the order each depends on the one before:
#
#   1. the tree it is handed builds with whim's compile line (its own makefile), and
#      that binary is the one the zero baselines are recorded from;
#   2. the makefile becomes tools/templates/zero.mk -- gcc -O0 -static -no-pie -s --
#      and whim-vim.c, unchanged, builds with it into a binary that is absolutely
#      static: readelf -h says EXEC, there is no INTERP, no dynamic section and not
#      one relocation;
#   3. the zero baselines, .reference/zero-baselines, are what the q82 binary does:
#      one tools/zrecord.sh, three times, identical each time.  An existing set is
#      compared, never overwritten;
#   4. the -no-pie binary shows NO difference from those baselines
#      (tools/zerodelta.sh --phase 83, pipes/zero.delta declaring nothing), and
#      whim's own cumulative delta still holds of it against slim-vim's baselines
#      (tools/whimdelta.sh --phase 82), unchanged.
#
# THESE BASELINES ARE RECORDED FROM THE PIPELINE'S OWN OUTPUT AT q82, and that is
# the one place this pipeline does so.  It is legitimate for the reason recording
# from the pipeline's input is: nothing from 83 on can reach q82, so a baseline
# taken from it cannot agree with a later phase by construction; and q82 itself is
# held by whim's own deltas against slim-vim's baselines, which step 4 rechecks.
# What it costs is that a change to what phases 0-82 produce moves this recording,
# and this phase then REFUSES rather than overwrite it -- see step 3.
#
# A tier 3 hit on this phase records nothing, because the phase does not run.  A
# checkout that has the cache and not .reference/zero-baselines gets them back with
# `rm -rf .cache/q83 && make whim-phase-83`.
#
# tools/templates/zero.mk is named here as a PATH so that tools/implhash.sh hashes it.
set -eu

work=${1:?usage: whim83.sh <work-dir>}
f="$work/whim-vim.c"
base=.reference/zero-baselines

# --- 1. the input's own binary, with the input's own compile line ----------
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/whim"
cp "$f" "$tmp/whim/whim-vim.c"
cp "$work/Makefile" "$tmp/whim/Makefile"
if ! make -C "$tmp/whim" >/dev/null 2>&1; then
    echo "  whim-vim     FAILED to build with the makefile q82 carries"
    exit 1
fi
wbin="$tmp/whim/whim-vim"
echo "  whim-vim     $(stat -c%s "$wbin") bytes, $(readelf -h "$wbin" | awk -F: '$1 ~ /^ *Type$/ { split($2, t, " "); print t[1] }'), q82's binary, $(grep -c '' "$f") lines"

# --- 2. the build, and what kind of file it is ------------------------------
cp tools/templates/zero.mk "$work/Makefile"
make -C "$work" clean >/dev/null 2>&1 || true
if ! make -C "$work" >/dev/null 2>&1; then
    echo "  build        FAILED -- rerun by hand: make -C $work"
    exit 1
fi
bin="$work/whim-vim"
type=$(readelf -h "$bin" | awk -F: '$1 ~ /^ *Type$/ { sub(/^ +/, "", $2); split($2, t, " "); print t[1] }')
interp=$(readelf -l "$bin" | grep -c 'INTERP' || true)
dynamic=$(readelf -d "$bin" | grep -c '^There is no dynamic section in this file\.$' || true)
relocs=$(readelf -r "$bin" | grep -c '^There are no relocations in this file\.$' || true)
if [ "$type" != EXEC ] || [ "$interp" != 0 ] || [ "$dynamic" != 1 ] || [ "$relocs" != 1 ]; then
    echo "  static       NOT absolutely static: type $type, INTERP $interp, no-dynamic $dynamic, no-relocations $relocs"
    echo "               zero's compile line is gcc -O0 -static -no-pie -s (tools/templates/zero.mk)"
    exit 1
fi
echo "  build        ok, $(stat -c%s "$bin") bytes: EXEC, no INTERP, no dynamic section, 0 relocations"

# --- 3. the baselines, from q82's binary ----------------------------------
# Three runs, and every run must be the same bytes -- a nondeterministic baseline
# is worse than none (SLIM-GOAL.md).  A recording is tools/zrecord.sh's five parts:
# the 102 keystroke cases, every Ex command typed at `:`, every command line the
# parser may see, the pty scenarios and the terminal table.  ZERO PHASE 3 is where
# the instrument became this; before it, the recording was the file-based
# behaviour.py and exsweep.py, which an editor with no file to write cannot use.
for r in 1 2 3; do
    tools/zrecord.sh "$wbin" "$tmp/whim/whim-vim.c" "$tmp/run$r"
    if [ "$r" != 1 ] && ! diff -r "$tmp/run1" "$tmp/run$r" >/dev/null; then
        echo "  baselines    run $r differs from run 1 -- not deterministic, not a baseline:"
        diff -rq "$tmp/run1" "$tmp/run$r" | head -10 | sed 's/^/                 /'
        exit 1
    fi
done
cases=$(ls "$tmp/run1/screen" | grep -c '')
cmds=$(grep -c '^=== ' "$tmp/run1/ref-excmds.txt")
argvs=$(grep -c '^=== ' "$tmp/run1/ref-argv.txt")
ptys=$(grep -c '^=== ' "$tmp/run1/ref-pty.txt")
terms=$(grep -c '' "$tmp/run1/ref-term.txt")

# An existing recording of the OLD shape is named rather than diffed: the two have
# no file in common, so a diff would print every line of both and say nothing.
if [ -d "$base" ] && [ ! -d "$base/screen" ]; then
    echo "  baselines    $base is the old file-based recording (behaviour/, ref-exsweep.txt)."
    echo "               Zero phase 3 replaced the instrument: a recording is now"
    echo "               screen/, ref-excmds.txt, ref-argv.txt, ref-pty.txt and"
    echo "               ref-term.txt (tools/zrecord.sh).  Remove it once, by hand,"
    echo "               and this phase records the new one from whim-vim:"
    echo "                 rm -rf $base && rm -rf .cache/q83 && make whim-phase-83"
    exit 1
fi

if [ -d "$base" ]; then
    if ! diff -r "$base" "$tmp/run1" >/dev/null; then
        echo "  baselines    DIFFER from the recorded $base:"
        diff -rq "$base" "$tmp/run1" | head -10 | sed 's/^/                 /'
        echo "               The same recording of a different q82, or a different recording"
        echo "               of the same one: a phase before 83 changed what it produces, or"
        echo "               a harness changed.  Name which before removing $base."
        exit 1
    fi
    echo "  baselines    match $base: $cases cases, $cmds commands, $argvs command lines, $ptys pty scenarios, $terms terminals, 3 identical runs"
else
    mkdir -p .reference
    rm -rf "$base.part"
    cp -r "$tmp/run1" "$base.part"
    mv "$base.part" "$base"
    echo "  baselines    recorded $base from whim-vim: $cases cases, $cmds commands, $argvs command lines, $ptys pty scenarios, $terms terminals, 3 identical runs"
fi

# --- 4. whim-vim against them, and whim's delta against slim ----------------
tools/zerodelta.sh "$bin" "$f" --phase 83

whim_last=$(. tools/pipeline.sh whim && echo "$((ZERO_FROM - 1))")
if [ -d .reference/baselines/behaviour ]; then
    # Its report names every one of whim's ~490 declared commands on one line, so
    # the line is counted here rather than printed; a failure is printed whole.
    if ! tools/whimdelta.sh "$bin" "$f" --phase "$whim_last" > "$tmp/whimdelta" 2>&1; then
        cat "$tmp/whimdelta"
        echo "  whim delta   whim-vim does NOT show whim's declared delta to phase $whim_last against slim-vim's baselines"
        exit 1
    fi
    held=$(sed -n 's/^ *delta  *exactly as declared: //p' "$tmp/whimdelta")
    printf '  %-12s %s commands and %s cases moved against slim-vim'"'"'s baselines, exactly whim'"'"'s declared delta to phase %s\n' \
        "whim delta" "$(printf '%s\n' "${held%%;*}" | wc -w)" "$(printf '%s\n' "${held#*cases:}" | wc -w)" "$whim_last"
else
    echo "  whim delta   no slim baselines at .reference/baselines -- whim's delta not rechecked"
fi
