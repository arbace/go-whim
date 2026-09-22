#!/bin/sh
# Whim phase 129, the check -- p_emoji is an int.
# See phase/129/edit.sh, and GOALS.md.
#
# Usage: phase/129/check.sh <work-dir> <state-dir>    (run from the repository root)
#
# Four things, in internal/check/whim129.go: every P_BOOL row of options[] names
# an int variable, where the input had exactly one exception, 'emoji'; p_emoji
# is its declaration, its row and its one reader and nothing else; the compile
# is silent and the libc surface is the one the stage was handed; and a line
# holding an emoji is written in the same bytes by both binaries while the
# CONTROL, `+set noemoji`, writes different ones.  The recording sees none of
# it -- no case types an emoji -- and moves nothing (phase/129/delta declares
# nothing for this phase).

# THE BODY IS GO: internal/check/whim129.go and internal/check/core.go, run through tools/st.sh.
# The tools it runs, named as PATHS so tools/implhash.sh hashes them into
# this phase's key -- a path the program does not name is a dependency no key
# sees.  Do not delete these lines.
#   tools/phasecheck.sh
#   tools/phasebuild.sh
#   tools/st.sh
set -eu

work=${1:?usage: phase/129/check.sh <work-dir> <state-dir>}
state=${2:?usage: phase/129/check.sh <work-dir> <state-dir>}
exec tools/st.sh check whim129 "$work" "$state"
