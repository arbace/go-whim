#!/bin/sh
# Whim phase 131, the check -- the saved input buffer is a garray_T *.
# See pipes/whim131-edit.sh, and WHIM-GOAL.md.
#
# Usage: pipes/whim131-check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim131.go: every line naming the saved input buffer is one of
# seven, each saying garray_T where the input said char_u; the two casts are
# gone and no other appeared; the compile is silent.

# THE BODY IS GO: internal/check/whim131.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/st.sh
set -eu

work=${1:?usage: whim131-check.sh <work-dir> <state-dir>}
state=${2:?usage: whim131-check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim131 "$work" "$state"
