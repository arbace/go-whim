#!/bin/sh
# Whim phase 38, the check -- the argument list is walked by :next and :previous alone.
# See phase/038/edit.sh, and GOALS.md.
#
# Usage: phase/038/check.sh <work-dir> <state-dir>      (run from the repository root)
#
# Runs after phase/038/edit.sh and the sweep tools/phaserun.sh runs between
# them, and reads nothing from the edit's shell -- only the work tree and the state
# directory, as tools/phaserun.sh describes.

# THE BODY IS GO: internal/check/whimb.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasebuild.sh
#   tools/phasecheck.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/038/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/038/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim38 "$work" "$state"
