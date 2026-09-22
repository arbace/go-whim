#!/bin/sh
# Whim phase 153 -- free_one_termoption() compares without a cast.  See WHIM-GOAL.md.
#
# Usage: pipes/whim153-edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# It compared an option variable's address, cast to char_u *, with a string
# value; the two are equal exactly when both are NULL, and the comparison
# now says so, with no pointer cast to another type (tx/FINDINGS.md).
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: whim153-edit.sh <work-dir> <state-dir>}
state=${2:?usage: whim153-edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim153 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' termopt "'$cflags'" "'$ldflags'"; exit 1; }
