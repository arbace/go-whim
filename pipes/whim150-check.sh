#!/bin/sh
# Whim phase 150, the check -- the regexp stack is three typed stacks.
# See pipes/whim150-edit.sh, and WHIM-GOAL.md.
#
# Usage: pipes/whim150-check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim150.go requires the same byte adjustments, nothing reading
# the stack as bytes, regexp probes against controls, and E363 at the exact
# 'maxmempattern' the input's binary reaches it.

# THE BODY IS GO: internal/check/whim150.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: whim150-check.sh <work-dir> <state-dir>}
state=${2:?usage: whim150-check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim150 "$work" "$state"
