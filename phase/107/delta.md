107 DECLARES NOTHING AT ALL, and it is phase 106's kind: THE BINARY IS BYTE-IDENTICAL,
788,488 bytes either side, built with SOURCE_DATE_EPOCH=0 and the boundary's own flags.
113 `__attribute__((unused))` are deleted, 20 `__attribute__((fallthrough));` are
respelled as the C23 `[[fallthrough]];`, and 6 `format`/`format_arg` are kept -- 139
attributes to 6, 117 lines changed, not one line added or removed, and not one
statement changed.  An attribute of any of these three kinds emits no code: `unused`
suppresses a diagnostic, `fallthrough` gives one a hint, and `format` decides what gcc
will check.  So tier 1 of CLAUDE.md's verification table applies in full and subsumes
every screen case, every Ex-command row, every command line and every pty scenario at
once; tools/zerodelta.sh --phase 107 corroborates.

AND THE BINARY IS BLIND TO THE ONE DECISION HERE THAT COULD BE WRONG, which is why the
phase does not rest on it alone.  MEASURED: removing the six survivors as well leaves
the binary STILL cmp-identical, while `-Wformat=2` goes from 115 `-Wformat-nonliteral`
warnings to zero.  The evidence for keeping them is therefore the warnings and not the
bytes: the identical 115 in the identical 53 functions before and after -- phase 105's
invariant -- with two controls that break it in opposite directions, vim_snprintf's
`format(printf, 3, 4)` removed giving 0 and `_()`'s `format_arg(1)` removed giving 135.
