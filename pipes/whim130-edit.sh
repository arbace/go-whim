#!/bin/sh
# Whim phase 130 -- the (pos_T *)-1 tests go.  See WHIM-GOAL.md.
#
# Usage: pipes/whim130-edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# get_address(), nv_gomark() and nv_pcmark() each compared a mark lookup with
# (pos_T *)-1, vim's old "mark in another file" -- and nothing in this tree
# returns it.  Each test is an if never taken, and cutil.FoldNever folds the
# three away keeping the branch that runs (tx/FINDINGS.md, 10).
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: whim130-edit.sh <work-dir> <state-dir>}
state=${2:?usage: whim130-edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim130 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { echo "  sentinel      the input binary did not build with '$cflags' '$ldflags'"; exit 1; }
