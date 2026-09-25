# The wide suite's terminal cases

Each case runs the editor on a real terminal, a pseudo-terminal of the given
size and `TERM`, so the paths a pipe never reaches run: `isatty`, the window size
asked of the terminal, the terminal modes set and restored. To stay exact, every
key is queued before the editor starts, on a terminal already raw and not
echoing, so the editor finds all its input waiting from the first read, as with
the quick suite's file. From the archived suite's scenarios (`448e9a8`,
`zpty.go`), plus sizes.

One line per case: name, rows, columns, `TERM`, keys (`cases.md` escapes),
tab-separated.

```
size_24x80	24	80	xterm	:set lines? columns?\r\e:q!\r
size_30x100	30	100	xterm	:set lines? columns?\r\e:q!\r
size_10x40	10	40	xterm	:set lines? columns?\r\e:q!\r
size_60x200	60	200	xterm	:set lines? columns?\r\e:q!\r
raw_typing	24	80	xterm	ihello world\e:set term?\r\e:q!\r
nav_arrows	24	80	xterm	il1\rl2\rl3\egg\e[B\e[B\e[C\e:q!\r
sel_arrows	24	80	xterm	ialpha one\e0\e[1;2C\e[1;2C\e:q!\r
wrap_long	10	40	xterm	i0123456789012345678901234567890123456789012345678901234567890123456789\e:q!\r
vt100	24	80	vt100	ione\rtwo\e:set term?\r\e:q!\r
dumb	24	80	dumb	ione\e:q!\r
```
