#!/bin/sh
# Whim phase 159 -- a struct's text is a pointer to an allocation of its own.  See WHIM-GOAL.md.
#
# Usage: pipes/whim159-edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# buffblock_T, msgchunk_T and regprog_T ended in a one-element array sized
# at allocation.  Each is now a char_u * to an allocation of its own.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: whim159-edit.sh <work-dir> <state-dir>}
state=${2:?usage: whim159-edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim159 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' structhack "'$cflags'" "'$ldflags'"; exit 1; }
