#!/bin/sh
# What a phase is called, from the document that defines it.
#
# Usage: tools/phasename.sh <phase>
#
# The goal documents' headings are the only place the phases are named, so the
# progress log reads them from there rather than keeping a second list that can
# disagree with the first.  Phases before ZERO_FROM are WHIM-GOAL.md's; the rest
# are ZERO-GOAL.md's, whose headings say `## Phase 95 (zero 12) — ...`.
set -eu

phase=${1:?usage: phasename.sh <phase> [pipeline]}
. tools/pipeline.sh "${2:-whim}"
doc=$DOC
[ "$phase" -lt "$ZERO_FROM" ] || doc=ZERO-GOAL.md
sed -En "s/^## Phase $phase( \([^)]*\))? *[—-] *//p" "$doc" | head -1
