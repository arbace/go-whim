# Phase 27 — `[[=a=]]` stops meaning "a with any accent"

A POSIX bracket expression has three bracketed forms inside it, and they are
three different features that happen to share a syntax:

| | | |
| --- | --- | --- |
| `[[:alpha:]]` | a character **class** | stays |
| `[[.x.]]` | a collating **element** | stays |
| `[[=a=]]` | an equivalence **class** | goes |

The third means "this character and every accented form of it", and expanding it
takes **`reg_equi_class()`, 1,397 lines** — a switch over every base letter
listing its variants across Latin-1, Latin Extended-A and Latin Extended-B. It
was the largest single function left in the file and the least used: reached
only when a pattern contains `[=`, and nothing in the editor writes one.

Two call sites, and the sweep did the rest: the bracket parser in `regatom()`,
where the `get_equi_class()` arm goes so `[=` falls through to being taken one
character at a time; and `skip_regexp()`'s scan, which asked the same question
only to know how far to skip.

`\w`, `\a` and `[[:alpha:]]` are a different mechanism and are untouched. The
phase checks both halves, because only the pair is a check: `[[=a=]]` must stop
matching an accented `a`, and `[[:alpha:]]` must still classify.
