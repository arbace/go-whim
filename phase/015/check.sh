#!/bin/sh
# Whim phase 15, the check -- the last two encoding options.
# See phase/015/edit.sh, and GOALS.md.
#
# Usage: phase/015/check.sh <work-dir> <state-dir>      (run from the repository root)
#
# Runs after phase/015/edit.sh and the sweep tools/phaserun.sh runs between
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

work=${1:?usage: phase/015/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/015/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim15 "$work" "$state"
