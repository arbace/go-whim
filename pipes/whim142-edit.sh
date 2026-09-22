#!/bin/sh
# Whim phase 142 -- the version names no build date or time.  See WHIM-GOAL.md.
#
# Usage: pipes/whim142-edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# init_longVersion() put __DATE__ " " __TIME__ into the version line a
# command-line error is headed with, so the binary depended on when it was
# built (tx/FINDINGS.md, 13).  The version is its name and release date.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: whim142-edit.sh <work-dir> <state-dir>}
state=${2:?usage: whim142-edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim142 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' datetime "'$cflags'" "'$ldflags'"; exit 1; }
