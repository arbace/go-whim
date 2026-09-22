#!/bin/sh
# Whim phase 152, the check -- the option variables are typed.
# See phase/152/edit.sh, and GOALS.md.
#
# Usage: phase/152/check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim152.go computes every row's typed variable from the
# input's, requires no punning left, and probes options of every kind and scope.

# THE BODY IS GO: internal/check/whim152.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/152/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/152/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim152 "$work" "$state"
