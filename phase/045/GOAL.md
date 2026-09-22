# Phase 45 — no `:drop`

`:drop` edited a file by making it the argument list and going to its first
entry: with one window and one buffer it was `:args` plus `:first`, both gone.
`ex_drop()` was the last caller of `set_arglist()` and `ex_rewind()`, and the
sweep takes all three.

## The delta

**None.** `:drop` already failed with no argument. Measured: 115,798 → **115,744
lines**.
