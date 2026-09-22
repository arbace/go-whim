#!/bin/sh
# Whim phase 134, the check -- the empty blocks fold.
# See phase/134/edit.sh, and GOALS.md.
#
# Usage: phase/134/check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim134.go COMPUTES the whole output: edit.W134Fold applied to
# the input's core, run through tools/sweep.sh, is the output byte for byte; and
# every empty block left is one the rule must keep.

# THE BODY IS GO: internal/check/whim134.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/134/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/134/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim134 "$work" "$state"
