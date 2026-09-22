#!/bin/sh
# Whim phase 147, the check -- deathtrap() runs at the host's next wait.
# See phase/147/edit.sh, and GOALS.md.
#
# Usage: phase/147/check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim147.go requires the handler and delivery, the libc surface
# grown by exactly pipe2, and probes SIGTERM and SIGHUP while waiting and
# SIGTERM while busy, on a real pty, against the input's binary.

# THE BODY IS GO: internal/check/whim147.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/147/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/147/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim147 "$work" "$state"
