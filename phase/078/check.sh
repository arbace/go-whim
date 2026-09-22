#!/bin/sh
# Whim phase 78, the check -- empty functions, write-only counters, and the window id.
# See phase/078/edit.sh, and GOALS.md.
#
# Usage: phase/078/check.sh <work-dir> <state-dir>      (run from the repository root)
#
# Runs after phase/078/edit.sh and the sweep tools/phaserun.sh runs between
# them, and reads nothing from the edit's shell -- only the work tree and the state
# directory, as tools/phaserun.sh describes.

# THE BODY IS GO: internal/check/whime.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasebuild.sh
#   tools/phasecheck.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/078/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/078/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim78 "$work" "$state"
