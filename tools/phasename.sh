#!/bin/sh
# What a phase is called, from the document that defines it.
#
# Usage: tools/phasename.sh <phase>
#
# The goal documents' headings are the only place the phases are named, so the
# progress log reads them from there rather than keeping a second list that can
# disagree with the first.  From phase 83 on a heading also gives the number the
# phase had in the zero pipeline: `## Phase 95 (zero 12) — ...`.
set -eu

phase=${1:?usage: phasename.sh <phase> [pipeline]}
. tools/pipeline.sh "${2:-whim}"
sed -En "s/^## Phase $phase( \([^)]*\))? *[—-] *//p" "$DOC" | head -1
