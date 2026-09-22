#!/bin/sh
# Whim phase 143, the check -- regatom() has no goto.
# See phase/143/edit.sh, and GOALS.md.
#
# Usage: phase/143/check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim143.go requires the helper to be the input's block, the
# line diff of regatom() to be exactly the accounted changes, and probes every
# path the rewrite touched.

# THE BODY IS GO: internal/check/whim143.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/143/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/143/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim143 "$work" "$state"
