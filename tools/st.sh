#!/bin/sh
# Run one whimtools subcommand.
#
# Usage: tools/st.sh <subcommand> [args]      (run from the repository root)
#
# WHAT IT IS FOR: finding the binary.  tools/gobuild.sh builds cmd/whimtools
# into a directory keyed on the sources, and a caller should not have to know
# that -- a makefile rule and a person at a prompt both
# say `tools/st.sh <subcommand>` and get the binary the tree implies.
#
# It costs nothing: the builder warm is 16 ms.
#
# The sweep is `tools/st.sh sweep FILE`, the canonicalisers `tools/st.sh canon
# FILE [--once]`.
set -eu

[ $# -gt 0 ] || { echo "usage: tools/st.sh <subcommand> [args]" >&2; exit 2; }

bin=$(tools/gobuild.sh)
exec "./$bin" "$@"
