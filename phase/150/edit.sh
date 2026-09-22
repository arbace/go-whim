#!/bin/sh
# Whim phase 150 -- the regexp stack is three typed stacks.  See GOAL.md.
#
# Usage: phase/150/edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# regmatch()'s stack held regitem_T records and, below a star's or a
# look-behind's record, its regstar_T or regbehind_T, in one byte array
# (tx/FINDINGS.md, 5 and 8).  They are three stacks of their own types, kept in
# step, and regstack_bytes keeps the byte count 'maxmempattern' is measured by.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: phase/150/edit.sh <work-dir> <state-dir>}
state=${2:?usage: phase/150/edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim150 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' regstack "'$cflags'" "'$ldflags'"; exit 1; }
