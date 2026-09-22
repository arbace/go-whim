#!/bin/sh
# Whim phase 148 -- allocation cannot fail.  See WHIM-GOAL.md.
#
# Usage: pipes/whim148-edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# host_alloc() never returns NULL, and lalloc()'s one other NULL -- a request
# for zero bytes, an internal error -- now reports the error and returns
# host_alloc(0).  So no allocation in the core can fail (tx/FINDINGS.md, 9).
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: whim148-edit.sh <work-dir> <state-dir>}
state=${2:?usage: whim148-edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim148 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' nofail "'$cflags'" "'$ldflags'"; exit 1; }
