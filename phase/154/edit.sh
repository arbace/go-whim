#!/bin/sh
# Whim phase 154 -- the NULL write in free_one_termoption() is gone.  See GOAL.md.
#
# Usage: phase/154/edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# ttest() called free_one_termoption() with t_Co's value where its address was
# meant; the call never cleared t_Co, and its one effect was a write through
# NULL when both were NULL (phase 153).  The call goes; the sweep takes the
# function.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: phase/154/edit.sh <work-dir> <state-dir>}
state=${2:?usage: phase/154/edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim154 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' nullwrite "'$cflags'" "'$ldflags'"; exit 1; }
