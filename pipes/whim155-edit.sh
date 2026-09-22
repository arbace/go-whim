#!/bin/sh
# Whim phase 155 -- call arguments with effects are evaluated in gcc's order.  See WHIM-GOAL.md.
#
# Usage: pipes/whim155-edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# gcc evaluates call arguments right to left; Go, left to right.  Where two
# arguments both have effects (internal/ccx), the one gcc evaluates first
# becomes a local computed before the call -- eleven calls.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: whim155-edit.sh <work-dir> <state-dir>}
state=${2:?usage: whim155-edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim155 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' argorder "'$cflags'" "'$ldflags'"; exit 1; }
