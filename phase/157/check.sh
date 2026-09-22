#!/bin/sh
# Whim phase 157, the check -- get_register() and put_register() carry a yankreg_T *, not a void *.
# See phase/157/edit.sh, and GOALS.md.
#
# Usage: phase/157/check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim157.go requires the typed prototypes, every pointer cast
# in a class, a byte-identical binary, and probes a Visual-mode put.

# THE BODY IS GO: internal/check/whim157.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/157/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/157/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim157 "$work" "$state"
