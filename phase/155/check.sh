#!/bin/sh
# Whim phase 155, the check -- call arguments with effects are evaluated in gcc's order.
# See phase/155/edit.sh, and GOALS.md.
#
# Usage: phase/155/check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim155.go measures gcc's order on the input's code, requires
# no argument pair left and the binary pairs left to right in the output's
# code, and probes the rewritten calls.

# THE BODY IS GO: internal/check/whim155.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/155/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/155/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim155 "$work" "$state"
