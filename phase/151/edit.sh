#!/bin/sh
# Whim phase 151 -- the option table's defaults are typed.  See GOAL.md.
#
# Usage: phase/151/edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# def_val[2] held a string option's defaults and, cast to char_u *, a number's
# or a boolean's (tx/FINDINGS.md, 2).  A row now has def_str[2] and def_num[2],
# the one its kind uses filled and the other empty, and every read names one.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: phase/151/edit.sh <work-dir> <state-dir>}
state=${2:?usage: phase/151/edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim151 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' defaults "'$cflags'" "'$ldflags'"; exit 1; }
