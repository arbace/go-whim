#!/bin/sh
# Whim phase 158 -- a highlight's terminal font is read only from a colour entry.  See GOAL.md.
#
# Usage: phase/158/edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# screen_start_highlight() tested a colour entry's font before t_colors, so
# on a terminal without colours it read it out of a term entry's start
# pointer.  The test asks t_colors first.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: phase/158/edit.sh <work-dir> <state-dir>}
state=${2:?usage: phase/158/edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim158 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' font "'$cflags'" "'$ldflags'"; exit 1; }
