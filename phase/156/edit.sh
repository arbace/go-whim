#!/bin/sh
# Whim phase 156 -- the regex size pass's node is a static byte, not (char_u *) -1.  See GOAL.md.
#
# Usage: phase/156/edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# The regex compiler's size pass marks its node pointer with (char_u *) -1,
# an integer made a pointer that is only ever compared.  It becomes the
# address of a static byte, reg_calc_size_node: fourteen uses.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: phase/156/edit.sh <work-dir> <state-dir>}
state=${2:?usage: phase/156/edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim156 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' calcsize "'$cflags'" "'$ldflags'"; exit 1; }
