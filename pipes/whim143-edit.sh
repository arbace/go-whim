#!/bin/sh
# Whim phase 143 -- regatom() has no goto.  See WHIM-GOAL.md.
#
# Usage: pipes/whim143-edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# regatom() jumped into other cases three ways (tx/FINDINGS.md, 11).  The
# delimiter atom becomes regatom_delim(), the multibyte node is written where
# its jump was, and the switch dispatches on sw in a loop that runs once, so
# the collection is reached by dispatching again.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: whim143-edit.sh <work-dir> <state-dir>}
state=${2:?usage: whim143-edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim143 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' regatom "'$cflags'" "'$ldflags'"; exit 1; }
