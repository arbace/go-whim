#!/bin/sh
# Whim phase 151, the check -- the option table's defaults are typed.
# See phase/151/edit.sh, and GOALS.md.
#
# Usage: phase/151/check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim151.go computes every row's two pairs from the input's,
# requires no def_val field left, and probes every option's default shown.

# THE BODY IS GO: internal/check/whim151.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/151/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/151/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim151 "$work" "$state"
