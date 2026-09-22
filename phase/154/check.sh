#!/bin/sh
# Whim phase 154, the check -- the NULL write in free_one_termoption() is gone.
# See phase/154/edit.sh, and GOALS.md.
#
# Usage: phase/154/check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim154.go proves the call's one effect was the NULL write,
# requires it and the function gone, and probes the paths into ttest().

# THE BODY IS GO: internal/check/whim154.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/154/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/154/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim154 "$work" "$state"
