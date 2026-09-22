#!/bin/sh
# Whim phase 7, the check -- the editor stops looking for files it was not given.
# See phase/007/edit.sh, and GOALS.md.
#
# Usage: phase/007/check.sh <work-dir> <state-dir>      (run from the repository root)
#
# Runs after phase/007/edit.sh and the sweep tools/phaserun.sh runs between
# them, and reads nothing from the edit's shell -- only the work tree and the state
# directory, as tools/phaserun.sh describes.

# THE BODY IS GO: internal/check/whimstd.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasebuild.sh
#   tools/phasecheck.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/007/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/007/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim7 "$work" "$state"
