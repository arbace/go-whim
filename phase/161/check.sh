#!/bin/sh
# Whim phase 161, the check -- no goto jumps into a block.
# See phase/161/edit.sh, and GOALS.md.
#
# Usage: phase/161/check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim161.go requires internal/ccx's Gotos to leave nothing,
# the new function to be the old tail, and probes ml_get_buf().

# THE BODY IS GO: internal/check/whim161.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/161/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/161/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim161 "$work" "$state"
