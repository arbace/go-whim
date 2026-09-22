#!/bin/sh
# Whim phase 137, the check -- the changedtick is a number.
# See pipes/whim137-edit.sh, and WHIM-GOAL.md.
#
# Usage: pipes/whim137-check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim137.go proves nothing but the number was ever read, and
# that each use of the tick is the input's, rewritten in place.

# THE BODY IS GO: internal/check/whim137.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: whim137-check.sh <work-dir> <state-dir>}
state=${2:?usage: whim137-check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim137 "$work" "$state"
