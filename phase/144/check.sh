#!/bin/sh
# Whim phase 144, the check -- edit() has no goto.
# See phase/144/edit.sh, and GOALS.md.
#
# Usage: phase/144/check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim144.go requires the helpers to be the input's blocks, every
# continue and break at a former jump to bind as the label's did, the line diff
# of edit() to be accounted for, and probes every key whose case jumped.

# THE BODY IS GO: internal/check/whim144.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/144/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/144/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim144 "$work" "$state"
