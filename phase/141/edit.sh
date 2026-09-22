#!/bin/sh
# Whim phase 141 -- regrepeat() does not jump into a case.  See GOAL.md.
#
# Usage: phase/141/edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# Seventeen character classes set their mask and jumped to do_class, a label
# inside the \s case (tx/FINDINGS.md, 11).  All eighteen now share one case
# that sets mask and testval in a switch on the same opcode, then runs the
# unchanged loop.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: phase/141/edit.sh <work-dir> <state-dir>}
state=${2:?usage: phase/141/edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim141 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' doclass "'$cflags'" "'$ldflags'"; exit 1; }
