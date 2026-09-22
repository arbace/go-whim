#!/bin/sh
# Whim phase 145, the check -- check_termcode() has no goto.
# See pipes/whim145-edit.sh, and WHIM-GOAL.md.
#
# Usage: pipes/whim145-check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim145.go proves from the input that the jump's path is the
# if/else, requires the else to be the skipped code byte for byte, and probes an
# OSC response split across two writes.

# THE BODY IS GO: internal/check/whim145.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: whim145-check.sh <work-dir> <state-dir>}
state=${2:?usage: whim145-check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim145 "$work" "$state"
