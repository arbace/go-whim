#!/bin/sh
# Whim phase 159, the check -- a struct's text is a pointer to an allocation of its own.
# See phase/159/edit.sh, and GOALS.md.
#
# Usage: phase/159/check.sh <work-dir> <state-dir>    (run from the repository root)
#
# internal/check/whim159.go partitions every mention of the three types,
# before and after, and probes the redo, record and message buffers and a pattern.

# THE BODY IS GO: internal/check/whim159.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/sweep.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/159/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/159/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim159 "$work" "$state"
