#!/bin/sh
# Whim phase 146, the check -- a memline node names its block.
# See pipes/whim146-edit.sh, and WHIM-GOAL.md.
#
# Usage: pipes/whim146-check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim146.go requires the casts gone and every read of a node's
# block to be of a node known to be there, and probes lines made, deleted and
# restored across block splits.

# THE BODY IS GO: internal/check/whim146.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: whim146-check.sh <work-dir> <state-dir>}
state=${2:?usage: whim146-check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim146 "$work" "$state"
