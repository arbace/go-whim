#!/bin/sh
# Whim phase 149, the check -- the allocation-failure branches fold.
# See phase/149/edit.sh, and GOALS.md.
#
# Usage: phase/149/check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim149.go computes the whole output with the same rule and
# the real sweep, and checks the never-NULL set again on the output.

# THE BODY IS GO: internal/check/whim149.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/149/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/149/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim149 "$work" "$state"
