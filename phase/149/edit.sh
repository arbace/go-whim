#!/bin/sh
# Whim phase 149 -- the allocation-failure branches fold.  See GOAL.md.
#
# Usage: phase/149/edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# Every NULL test of a never-NULL allocation's result that follows it folds,
# the never-NULL functions found to a fixpoint from host_alloc(); labels only
# the folded branches jumped to go (tx/FINDINGS.md, 9).
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: phase/149/edit.sh <work-dir> <state-dir>}
state=${2:?usage: phase/149/edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim149 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' allocnull "'$cflags'" "'$ldflags'"; exit 1; }
