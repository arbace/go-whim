#!/bin/sh
# Whim phase 136 -- the engine is called directly.  See WHIM-GOAL.md.
#
# Usage: pipes/whim136-edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# bt_regengine is the only regengine_T and every program's engine points at it,
# so the four calls through the table call known functions.  They name them,
# bt_regcomp() stops recording an engine, and the sweep takes the table, the
# field and regengine_T (tx/FINDINGS.md, 4).
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: whim136-edit.sh <work-dir> <state-dir>}
state=${2:?usage: whim136-edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim136 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' engine "'$cflags'" "'$ldflags'"; exit 1; }
