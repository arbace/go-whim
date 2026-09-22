#!/bin/sh
# Whim phase 139 -- the core sorts and searches typed arrays.  See WHIM-GOAL.md.
#
# Usage: pipes/whim139-edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# The four table searches call a typed copy of musl_bsearch() -- the same
# probes in the same order -- the comparators take the type they cast to, and
# :undolist's sort is an insertion sort; the sweep takes musl_qsort(),
# musl_bsearch() and sort_compare() (tx/FINDINGS.md, 7).
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: whim139-edit.sh <work-dir> <state-dir>}
state=${2:?usage: whim139-edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim139 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' typed "'$cflags'" "'$ldflags'"; exit 1; }
