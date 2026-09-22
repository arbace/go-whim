#!/bin/sh
# Whim phase 160, the check -- no line getter takes a cookie.
# See pipes/whim160-edit.sh, and WHIM-GOAL.md.
#
# Usage: pipes/whim160-check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim160.go requires every cookie passed to have been nullptr,
# every void * left memory (internal/ccx), and probes each getter.

# THE BODY IS GO: internal/check/whim160.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: whim160-check.sh <work-dir> <state-dir>}
state=${2:?usage: whim160-check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim160 "$work" "$state"
