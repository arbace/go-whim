# The wide suite's command lines

Each line is one invocation's arguments, separated by `|` (an empty line is no
arguments); every invocation then gets the keys `\e:q!\r`. From the archived
suite (`448e9a8`, `zargv.go`). It is a record of what each answers, whatever
that is: spellings of options the editor no longer takes are here on purpose,
because how it refuses them is behaviour too. `f.txt` names a file the core
cannot open.

```

+q!
+set nu|+q!
+set nu
-T|xterm
-T
-Txterm
-T|no-such-term-9x
-e
-E
-e|-s
-v
-
--
--ttyfail
-R
-c|q!
-u|NONE
-i|NONE
-m
-Z
-y
f.txt
f.txt|g.txt
--version
--help
-h
+
+q!|f.txt
--|+q!
```
