#!/bin/sh
# Whim phase 146 -- a memline node names its block.  See WHIM-GOAL.md.
#
# Usage: pipes/whim146-edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# A tree node was a block beginning with a tagged header, and the tree cast
# the header to the block its tag named (tx/FINDINGS.md, 4).  The header
# becomes the node, holding its tag and a typed pointer to its block, and each
# of the 19 casts reads that pointer.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: whim146-edit.sh <work-dir> <state-dir>}
state=${2:?usage: whim146-edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim146 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' memnode "'$cflags'" "'$ldflags'"; exit 1; }
