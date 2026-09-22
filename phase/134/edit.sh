#!/bin/sh
# Whim phase 134 -- the empty blocks fold.  See GOAL.md.
#
# Usage: phase/134/edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# Phase 132 left 33 blocks empty, beside those earlier phases left: 49 fold.
# An empty block guarded by a condition that only reads goes, as do an empty
# else and an empty else-if ending its chain; the sweep takes what the
# conditions computed and nothing reads any more.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: phase/134/edit.sh <work-dir> <state-dir>}
state=${2:?usage: phase/134/edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim134 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' fold "'$cflags'" "'$ldflags'"; exit 1; }
