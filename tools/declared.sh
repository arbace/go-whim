#!/bin/sh
# tools/declared.sh FROM TO -- the declared delta of phases FROM to TO.
#
# Each phase declares, in advance, how its binary may move: phase/NNN/delta, a
# token per way, `#` notes saying why, and nothing at all for a phase that
# declares nothing (the words are GOALS.md's, *What is measured*).  This prints
# the declarations of a run of phases in the grammar tools/whimdelta.sh and
# tools/zerodelta.sh read -- `N  tokens` for a phase's first line, continuation
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
    f=phase/$(printf %03d "$p")/delta
    if [ -f "$f" ]; then
        awk -v n="$p" '
            /^[ \t]*#/ || NF == 0 { next }
            { sub(/^[ \t]+/, ""); if (seen++) printf "      %s\n", $0; else printf "%-6s%s\n", n, $0 }' "$f"
    fi
    p=$((p + 1))
done
