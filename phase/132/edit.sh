#!/bin/sh
# Whim phase 132 -- nothing frees.  See GOAL.md.
#
# Usage: phase/132/edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# host_free() has had an empty body since phase 124, so vim_free() -- a NULL
# test around it -- does nothing observable.  All 273 calls to either in the
# core go; the two whose argument decrements a counter keep the decrement.
# vim_free() is then called by nothing and the sweep takes it
# (tx/FINDINGS.md, 9).
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: phase/132/edit.sh <work-dir> <state-dir>}
state=${2:?usage: phase/132/edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim132 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { echo "  free          the input binary did not build with '$cflags' '$ldflags'"; exit 1; }
