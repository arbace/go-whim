#!/bin/sh
# Delete what the cut left unreachable, to a fixpoint -- all six kinds, with
# the canonicalisers as the seventh member of every round.
#
# Usage: tools/sweep.sh <file.c>      (run from the repository root)
#
# THE WORK IS NOW GO.  This file is the entry point the phase programs name and
# nothing else: it builds the module and hands the file to `whimtools sweep`,
# which is internal/sweep/sweep.go.  The shell version it replaces is
# in this repository's history, and the two were compared before the swap --
# the same report line for line, including the skip cache's `passed this text
# already` entries, and the same swept bytes.
#
# What the Go does differently is not what it computes but how often it pays
# for the file: the shell ran thirteen programs a round, each re-reading and
# rewriting a multi-megabyte file, and the Go reads once, transforms in memory,
# and writes twice -- for deadsweep, which asks gcc about the file, and
# deadenums, which asks tools/enumvals.sh.  Measured on whim125's unswept
# output, 79,380 lines: 15.675s became 8.353s and the output was identical.
# What is left is gcc, which neither version can avoid.
#
# The three lines it names -- cmd/, internal/, go.mod and go.sum -- are what
# tools/gobuild.sh builds the binary from; this file only finds it.
set -eu

f=${1:?usage: sweep.sh <file.c>}
bin=$(tools/gobuild.sh)
exec "./$bin" sweep "$f"
