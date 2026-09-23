#!/bin/sh
# tools/declared.sh FROM TO -- the declared delta of phases FROM to TO.
#
# Each phase declares, in advance, how its binary may move: phase/NNN/delta.md,
# a token per way inside a FENCED BLOCK, prose around it saying why, and no
# block at all for a phase that declares nothing (the words are GOALS.md's,
# *What is measured*).  The fence is what separates the data from the prose:
# everything outside one is a note, and a phase that declares nothing is all
# note.  This prints
# the declarations of a run of phases in the grammar tools/whimdelta.sh and
# tools/coredelta.sh read -- `N  tokens` for a phase's first line, continuation
# lines indented, notes and blank lines left out -- so a checker reads one list
# however the phases keep theirs.
#
# FROM and TO are plain integers; the directory is named with printf %03d, and
# never with shell arithmetic on a padded name, which would read 010 as octal.
set -eu

from=${1:?usage: declared.sh FROM TO}
to=${2:?usage: declared.sh FROM TO}
p=$from
while [ "$p" -le "$to" ]; do
    f=phase/$(printf %03d "$p")/delta.md
    if [ -f "$f" ]; then
        awk -v n="$p" '
            /^[ \t]*```/ { fence = !fence; next }
            !fence || NF == 0 { next }
            { sub(/^[ \t]+/, ""); if (seen++) printf "      %s\n", $0; else printf "%-6s%s\n", n, $0 }' "$f"
    fi
    p=$((p + 1))
done
