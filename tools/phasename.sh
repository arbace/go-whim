#!/bin/sh
# What a phase is called, from the document that defines it.
#
# Usage: tools/phasename.sh <phase>
#
# A phase's GOAL.md opens with its heading, `# Phase N — what it does`, and that
# heading is the only place the phase is named, so the progress log reads it from
# there rather than keeping a second list that can disagree with the first.
set -eu

phase=${1:?usage: phasename.sh <phase>}
. tools/pipeline.sh
sed -En "1s/^# Phase $phase *[—-] *//p" "$(phasedir "$phase")/GOAL.md"
