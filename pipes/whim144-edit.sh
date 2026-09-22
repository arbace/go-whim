#!/bin/sh
# Whim phase 144 -- edit() has no goto.  See WHIM-GOAL.md.
#
# Usage: pipes/whim144-edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# Insert mode jumped to doESCkey, normalchar and do_intr from other cases and
# from before its switch (tx/FINDINGS.md, 11).  The two blocks become
# edit_esc() and edit_normalchar(), each jump a call whose continue or break
# is checked to go where the label's went, and do_intr's block is written out.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: whim144-edit.sh <work-dir> <state-dir>}
state=${2:?usage: whim144-edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim144 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' editgoto "'$cflags'" "'$ldflags'"; exit 1; }
