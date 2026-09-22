#!/bin/sh
# What whim-vim does from phase 83 on, differently from q82, as a check rather
# than a report.
#
# Usage: tools/coredelta.sh <binary> <source> --phase N
#        tools/coredelta.sh --declared N
#
# tools/whimdelta.sh's rule against a different instrument, and whimdelta.sh hands
# every phase from CORE_FROM on to it.  --phase N records the
# binary with tools/zrecord.sh and hands the recording, the baselines and
# the declarations from CORE_FROM on (tools/declared.sh) to zcompare, which
# requires **exactly** the declared difference: every record that moved is
# declared, every declaration moved something, and nothing else differs at all.  --declared N prints what phase N
# itself declares, for a phase program that wants to assert its own list.
#
# THE BASELINES ARE q82'S: .reference/core-baselines, recorded by phase 83 from the
# tree it is handed, built with the compile line that tree carries.  So the delta is
# the difference from q82, not from slim; it is CUMULATIVE, as phases 0-82's are
# against slim -- the declarations up to phase N are the whole difference from q82
# at N -- and it starts empty at 83.  phase/083/make.sh says why a recording of q82 is not the mistake
# CLAUDE.md warns about: nothing from 83 on can reach it.
#
# THE INSTRUMENT IS THE SCREEN (phase 86, GOALS.md II.2): keystrokes in on stdin,
# escape sequences out on stdout, and a screen per redraw rebuilt from them.  The
# file-based harnesses -- behaviour, exsweep -- are phases 0-82's and are untouched;
# they cannot measure these, because the editor they measure is on its way to having
# no file to write and no stream to print on.
#
# "Six commands differ" is a check.  "Some commands differ" is not.
set -eu

# The line between the two arcs, stated once in Go (internal/build.CoreFrom) and
# once here, because this script is shell and has nowhere else to read it from.
CORE_FROM=83

if [ "${1:-}" = "--declared" ]; then
    n=${2:?usage: coredelta.sh --declared N}
    d=$(mktemp)
    tools/declared.sh "$CORE_FROM" "$n" > "$d"
    tools/st.sh zcompare --declared "$d" "$n"
    rm -f "$d"
    exit 0
fi

bin=${1:?usage: coredelta.sh <binary> <source> --phase N}
src=${2:?usage: coredelta.sh <binary> <source> --phase N}
[ "${3:-}" = "--phase" ] || { echo "usage: coredelta.sh <binary> <source> --phase N" >&2; exit 2; }
n=${4:?usage: coredelta.sh <binary> <source> --phase N}

base=.reference/core-baselines
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
fail=0

# The same source check whimdelta.sh runs beside its harnesses: no option global
# left without the row that initialises it (orphanopts).
tools/st.sh orphanopts "$src" > "$tmp/orphanopts" 2>&1 &
pid_o=$!

orphans() {
    if wait $pid_o; then cat "$tmp/orphanopts"; else cat "$tmp/orphanopts"; fail=1; fi
}

# Unlike whimdelta.sh, an absent baseline is a failure and not a note: phase 83
# records them before anything is compared, so a missing set means the pipeline is
# being run out of order.
if [ ! -d "$base/screen" ] || [ ! -f "$base/ref-excmds.txt" ] \
        || [ ! -f "$base/ref-argv.txt" ] || [ ! -f "$base/ref-term.txt" ] \
        || [ ! -f "$base/ref-pty.txt" ]; then
    orphans
    echo "  delta        no core baselines at $base -- phase 83 records them"
    echo "               (a recording is screen/, ref-excmds.txt, ref-argv.txt,"
    echo "                ref-pty.txt and ref-term.txt: tools/zrecord.sh)"
    exit 1
fi

tools/zrecord.sh "$bin" "$src" "$tmp/now"
orphans

tools/declared.sh "$CORE_FROM" "$n" > "$tmp/declared"
tools/st.sh zcompare "$base" "$tmp/now" "$tmp/declared" "$n" || fail=1
exit $fail
