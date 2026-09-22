#!/bin/sh
# Whim phase 137 -- the changedtick is a number.  See WHIM-GOAL.md.
#
# Usage: pipes/whim137-edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# b:changedtick was a dictionary item inside buf_T, read as
# ((buf)->b_ct_di.di_tv.vval); with no buffer variables left it is a number
# in a typval in a struct, and the one field keeping typval_T alive in buf_T.
# The field becomes a varnumber_T and its 18 uses name it.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: whim137-edit.sh <work-dir> <state-dir>}
state=${2:?usage: whim137-edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim137 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' tick "'$cflags'" "'$ldflags'"; exit 1; }
