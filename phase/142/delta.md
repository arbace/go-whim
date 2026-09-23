142 declares nothing, and the change is real: a command-line error's version
line loses ", compiled <date> <time>".  It is written to stderr, which 85's
stderr-moved excludes from every comparison, so no record can show it.  The
check reads the line from both binaries, and requires the output built at two
SOURCE_DATE_EPOCHs to be the same bytes where the input's differ.
