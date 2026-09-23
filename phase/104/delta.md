104 DECLARES NOTHING AT ALL, and it is 101, 102 and 103's kind: the code runs, the
instrument sees it run, and it does the same thing.  Every byte this editor has ever
put on a stream instead of a screen -- 20 statements in five functions -- now goes
through one `vim_host_message(msg, len, err)` the launcher installs and
`host_message()` performs with `write(err ? 2 : 1, ...)`, and two full recordings
either side are BYTE-IDENTICAL: `diff -r` reports 0 lines across 102 screen cases,
ref-excmds.txt, ref-argv.txt, ref-pty.txt and ref-term.txt.  Only the syscall
underneath the bytes changes.

AND THE INSTRUMENT CAN SEE IT, which is what separates this from phases 85, 95 and 103.
24 of the 30 ref-argv.txt rows really do carry a message -- 23 mainerr and one
report_term_error -- and the phase proves it marks the SAME 24 either side, with the
input built with `write(2, "MESSAGE-OUT\n", 12)` at all nineteen output statements and
the output with the identical instrument inside host_message().  So an empty
declaration here is not blindness; it is the strongest kind this pipeline has short of
phase 99's byte-identical binary.

THE PHASE OWES A CONTROL FOR EXACTLY THAT REASON, and it is one character:
`write(err ? 2 : 1, ...)` made `write(err ? 1 : 1, ...)` moves all 24 records, 217
lines of `diff -r` -- where tools/zerodelta.sh names only 14 of them, ten of the 24
being argv rows phases 87 and 88 already declared and tools/zcompare.py therefore no
longer comparing.  That gap is why phase/104/check.sh diffs the two recordings
itself and keeps zerodelta.sh as the second opinion.

ONE THING REALLY CHANGES AND NO INSTRUMENT IN THIS PIPELINE CAN SEE IT: mainerr and
report_term_error assemble into a 1024-byte buffer, so a message built from an argv
string longer than about 930 characters is now capped.  MEASURED on both binaries: an
unknown option of 900 characters is byte-identical and one of 2,000 is 2,095 bytes on
the input and 1,023 here; a `-T` of 2,000 is 2,039 against 1,023.  It is not declared,
because nothing in the corpus is within 800 characters of it and `phase/104/delta` is
a list of records that moved; it is pinned as a probe in both directions instead.
