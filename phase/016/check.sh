#!/bin/sh
# Whim phase 16, the check -- six options that no longer decide anything.
# See phase/016/edit.sh, and GOALS.md.
#
# Usage: phase/016/check.sh <work-dir> <state-dir>      (run from the repository root)
#
# Runs after phase/016/edit.sh and the sweep tools/phaserun.sh runs between
# them, and reads nothing from the edit's shell -- only the work tree and the state
# directory, as tools/phaserun.sh describes.

# THE BODY IS GO: internal/check/whima.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasebuild.sh
#   tools/phasecheck.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/016/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/016/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim16 "$work" "$state"
