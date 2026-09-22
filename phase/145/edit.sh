#!/bin/sh
# Whim phase 145 -- check_termcode() has no goto.  See GOAL.md.
#
# Usage: phase/145/edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# While an OSC response arrived over several reads, the loop jumped into the
# OSC branch of a later if-chain (tx/FINDINGS.md, 11).  The jump's if handles
# the response itself and everything the jump skipped becomes its else.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: phase/145/edit.sh <work-dir> <state-dir>}
state=${2:?usage: phase/145/edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim145 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' oscgoto "'$cflags'" "'$ldflags'"; exit 1; }
