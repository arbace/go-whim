#!/bin/sh
# Whim phase 135 -- one regexp program type.  See GOAL.md.
#
# Usage: phase/135/edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# With one engine every regprog_T is a bt_regprog_T, and the casts between the
# two are casts to itself (tx/FINDINGS.md, 4).  regprog_T takes the
# backtracking fields, the casts go, and bt_regprog_T is not a name any more.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: phase/135/edit.sh <work-dir> <state-dir>}
state=${2:?usage: phase/135/edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim135 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' regprog "'$cflags'" "'$ldflags'"; exit 1; }
