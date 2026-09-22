# The pipeline, and the only place that states its parameters.
#
# Sourced, not run:  . tools/pipeline.sh
#
#   whim-vim.c = G(slim-vim.c)       GOALS.md, work in .tmp/whim-stage/
#
# ONE PIPELINE, and the tools take no pipeline argument: this file is where its
# parameters are stated, and the only place.  They took one while a second
# pipeline existed, whose phases are 83 onwards here.
#
# The boundary tag is q (arbace/slim-vim's own pipeline, which produces this one's
# input, tags its boundaries p).
#
# A PHASE IS A DIRECTORY, phase/NNN/, numbered with three digits so that they
# sort: its program (make.sh, or edit.sh and check.sh), its GOAL.md and its
# declared delta.  phasedir N names it.  N is a plain integer everywhere else: a
# padded name in shell arithmetic is octal, and 010 is 8.
#
# PSOURCE is the one file a sweep runs on, which is what lets tools/phaserun.sh
# run a split phase's sweep itself.
#
# PDELTA is the checker a stage's declared delta goes through: tools/phaserun.sh
# runs it and tools/implhash.sh hashes it.  There are TWO SETS OF BASELINES, and
# CORE_FROM is the line between them.  Phases before it are measured against
# .reference/baselines, recorded by phase 0 from slim-vim.c; phases from it on
# against .reference/core-baselines, recorded by phase CORE_FROM from the tree it
# is handed, with an instrument (tools/zrecord.sh) that an editor with no file to
# write can still be measured by.  tools/whimdelta.sh hands a phase from
# CORE_FROM on to tools/coredelta.sh.  Every phase declares its delta the same
# way, in its own directory; tools/declared.sh reads a run of them.
#
# THE PHASE LIST IS NOT WRITTEN HERE, and that is measured rather than tidy.  This
# file is hashed into every stage's key and every edit's (it is named by
# tools/phaserun.sh, which every split phase's key reads), so a byte changed here
# re-keys the whole pipeline.  A list written here would do that every time a
# phase is added.  It is the `phases` line of phase/stages instead, which no key
# reads.

phasedir() { printf 'phase/%03d' "$1"; }

TAG=q
PWORK=.tmp/whim-stage
PBUILD=.build
PORACLE=.reference/whim-phases
PSOURCE=whim-vim.c
PDELTA=tools/whimdelta.sh
CORE_FROM=83
PHASE_LIST=$(awk '$1 == "phases" { $1 = ""; sub(/^ +/, ""); print }' phase/stages)
