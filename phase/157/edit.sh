#!/bin/sh
# Whim phase 157 -- get_register() and put_register() carry a yankreg_T *, not a void *.  See GOAL.md.
#
# Usage: phase/157/edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# get_register() returns a register as void * and put_register() casts it
# back: the register is typed yankreg_T * throughout, and the core's last
# pointer cast outside the classes internal/ccx names goes.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: phase/157/edit.sh <work-dir> <state-dir>}
state=${2:?usage: phase/157/edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim157 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' register "'$cflags'" "'$ldflags'"; exit 1; }
