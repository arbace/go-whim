#!/bin/sh
# Whim phase 147 -- deathtrap() runs at the host's next wait.  See GOAL.md.
#
# Usage: phase/147/edit.sh <work-dir> <state-dir>     (run from the repository root)
#
# SIGHUP and SIGTERM ran deathtrap() as their handler (tx/FINDINGS.md, 12).
# The handler records the signal and writes a byte to a pipe; the host's wait
# selects on the pipe beside the input, and the wait and the read run
# deathtrap() first, inside the wait where the core unblocks deadly signals.
# The libc surface grows by pipe2.
#
# THE INPUT BINARY IS BUILT HERE, before the edit, from the boundary's own
# makefile flags, as $state/old beside $state/old.c, for the check.
set -eu

work=${1:?usage: phase/147/edit.sh <work-dir> <state-dir>}
state=${2:?usage: phase/147/edit.sh <work-dir> <state-dir>}
f="$work/whim-vim.c"

cflags=$(sed -n 's/^CFLAGS  *= *//p' "$work/Makefile")
ldflags=$(sed -n 's/^LDFLAGS  *= *//p' "$work/Makefile")
cp "$f" "$state/old.c"
# shellcheck disable=SC2086
( cd "$state" && SOURCE_DATE_EPOCH=0 gcc $cflags $ldflags -o old old.c ) &
pid_old=$!

tools/st.sh edit whim147 "$f"

# An edit that starts a background job waits for it before it exits (tools/phaserun.sh).
wait $pid_old || { printf '  %-13s the input binary did not build with %s %s\n' selfpipe "'$cflags'" "'$ldflags'"; exit 1; }
