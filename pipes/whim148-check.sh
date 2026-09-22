#!/bin/sh
# Whim phase 148, the check -- allocation cannot fail.
# See pipes/whim148-edit.sh, and WHIM-GOAL.md.
#
# Usage: pipes/whim148-check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim148.go proves host_alloc() never returns NULL and requires
# lalloc() to be the phase's body.

# THE BODY IS GO: internal/check/whim148.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: whim148-check.sh <work-dir> <state-dir>}
state=${2:?usage: whim148-check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim148 "$work" "$state"
