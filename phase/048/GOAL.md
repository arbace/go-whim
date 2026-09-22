# Phase 48 — no `:noswapfile`

There has been no swap file since Phase 21: the memfile is memory. The modifier
set `CMOD_NOSWAPFILE`, whose two readers in `ml_open()` and `buf_copy_options()`
were already empty blocks. It is matched by name in `parse_command_modifiers()`
before the table, so its branch goes as well as its row, and so does its line in
the completion arm for modifiers.

## The delta

**The row**, which succeeded run bare. Measured: 115,588 → **115,568 lines**.
