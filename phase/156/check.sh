#!/bin/sh
# Whim phase 156, the check -- the regex size pass's node is a static byte, not (char_u *) -1.
# See phase/156/edit.sh, and GOALS.md.
#
# Usage: phase/156/check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim156.go requires every use of the sentinel to be a
# comparison or the one assignment, before and after, no integer cast to a
# pointer left, and probes every node-making path of the regex compiler.

# THE BODY IS GO: internal/check/whim156.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/156/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/156/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim156 "$work" "$state"
