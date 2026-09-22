#!/bin/sh
# Whim phase 135, the check -- one regexp program type.
# See phase/135/edit.sh, and GOALS.md.
#
# Usage: phase/135/check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim135.go proves bt_regengine the only engine, requires the
# one definition and no cast left, and requires the input and the output to
# build to the same bytes.

# THE BODY IS GO: internal/check/whim135.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/135/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/135/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim135 "$work" "$state"
