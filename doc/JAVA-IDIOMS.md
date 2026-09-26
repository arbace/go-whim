# How the Java editor could be more idiomatic Java: a survey

**Status (2026-09-26): a survey; nothing in it is done.** It is written after
`GO-IDIOMS.md`, and in its manner: every number was counted, not estimated,
unless it says otherwise; each item says what the pattern is, how many sites
it has, what it would become, who would do it -- the Java backend's printer
(`crefactor/togo/java*.go`), the analysis it shares with the Go, a pipeline
phase that changes the C, or the hand-written runtime and host -- what it
costs, what it risks and what it gains. `braaam/Editor.java` is generated and
stays so: nothing here is a change to be made by hand to it.

It covers `braaam/Editor.java` as tracked at `2ca3292` (65,356 lines,
generated from the 73,632 lines of `src/editor.c`), and the hand-written
`braaam/rt/` (10 files, 902 lines), `braaam/host/` (5 files, 1,922 lines) and
`braaam/Whim.java` (330 lines). No tracked file changed; the instruments are
throwaways under the worktree's `.tmp/jid/` (listed at the end).

**The short answer.** The Java is C written in Java's syntax, and most of what
makes it read so is not the pointer model -- which stays, for the reason the
Go's did -- but the printer: it writes every constant as a bare number (all
961 case labels: `case 27:` where the C says `case ESC:`), every local at the
top of its method with a zero it overwrites (3,449), almost half of its
23,779 parentheses where Java's precedence needs none (11,127), a `& 0xff` on
every byte it widens (2,184), and the file's tables one member at a time
(9,251 statements for 4,121 rows). Those are printer rules, cheap, and the
first two can be proved with no test at all: javac, with `-g:none`, writes
**byte-identical class files** for them (measured, below). What the C causes
-- the one-element arrays behind `pos.lnum[0]`, the int flags, the gettext
calls -- is a phase each, and the Go gains from each too.

## How it was measured

| Tool | What it gave |
| --- | --- |
| `javac -Xlint:all` (JDK 26, the machine's) | 73 warnings: in `Editor.java` 40 lossy compound assignments and 23 fall-throughs; in the host 9 for `sun.misc.Signal` and 1 restricted method. The class-level `@SuppressWarnings({"unchecked", "rawtypes"})` hides 7 more (3 unchecked, 4 raw `Ptr`) |
| `.tmp/jid/count/JCount.java` | a throwaway program on the JDK's compiler tree API (`com.sun.source`): it parses and attributes `Editor.java` against the runtime and counts declarations, locals, pointer calls, boxes and their uses, casts, comparisons, parentheses by precedence, loops, switches and labels, and writes the call and field graph (`out/edges.tsv`, `out/sites.tsv`) |
| `.tmp/jid/graph/main.go` | a throwaway: every method mapped to the vim 9.2 source file that defines a function of its name (`/root/test/vim-9.2.1031/src`; 1,629 of 1,718 found, the rest whim's, musl's or the initialisers), the files grouped into 11 subsystems, and the calls and field references counted within and across them |
| PMD 7.16.0, categories best practices, code style, design, error prone, performance | 40,882 findings on `Editor.java`, 1,636 on the hand-written Java. PMD 7.16 cannot read JDK 26's class files (major 70), so it ran on a Temurin 21 for Alpine, downloaded beside it |
| Checkstyle 10.26.1, `google_checks.xml` and `sun_checks.xml` | the naming and length checks below; its indentation rule (Google's two spaces, 63,073 findings) and Sun's magic numbers and Javadoc are not idiom and are left out |
| `javac -g:none` twice and `cmp` | the proof of a cosmetic rule: three hand edits of a copy (a case label by name, a constant expression by names, the parentheses of one `return` removed) compile to byte-identical class files |
| `grep -c`, `awk` | the textual counts, each with its pattern in `.tmp/jid/` |

**What exists offline.** The JDK's own `javac -Xlint` and `javap` are the only
Java linters on the machine; Maven's repository holds the Error Prone
*annotations* and nothing that runs. PMD, Checkstyle and the JDK 21 were
fetched for this survey into `.tmp/jid/tools/`; none belongs in the build.

**Verification works today, and it has a new instrument.** At `2ca3292`, in
the survey's worktree: `whim test --java` -- the Java editor answers all 45
cases as the C does, its own control seen by 41; `whim test --wide --java` --
all 240, the control seen by 94 keys and 6 pty cases; `make
whim-editor-check` -- `Editor.java` is what the backend writes. Every item
below is verified by those three. And a rule that changes only how the Java
is spelled -- parentheses, a constant's name for its value, a char literal
for its code, `Long.MAX_VALUE` for `9223372036854775807L` -- can be proved
without running anything: javac folds constants and drops parentheses, so the
class files of the old and the new `Editor.java`, compiled with `-g:none`,
are **byte for byte the same** (measured on the three edits above: `cmp`
silent, and `javap -c -p` identical). It is the Java's `SOURCE_DATE_EPOCH`
binary comparison, and cheaper: a `whim java --same-classes OLD NEW` would
make it a check.

## Where the lines go

| Lines | What | Note |
| --- | --- | --- |
| 927 | `static final` constants (920 `int`, 6 `long`), alphabetical | every enumerator the code names; 870 ALL_CAPS already, 56 not (`CMD_append`, `map_result_get`) |
| 3,987 | 105 struct and union classes (51 `S_`, 48 `T_`, 6 `A_`) | 746 members, and in each `set`, `copy`, `zero` and `array` (526 methods, 3,881 lines) |
| 55 | 11 functional interfaces, `Fn1`..`Fn11` | |
| 986 | the instance fields: 480 plain, 506 `final` (arrays and structs), 164 of them method references (`fp_ex_edit = this::ex_edit`) | |
| 9,557 | the constructor and `initGlobals0`..`14` | 9,510 statements, 9,251 of them one member of one table row |
| ~49,800 | 1,702 methods (and 17 abstract host methods, 9 growarray accessors) | median 11 lines, 90th percentile 64; 92 over 100 lines, 29 over 200, 7 over 500 (`regmatch` 1,054, `win_line` 787, `regatom` 777) |

## The patterns

### 1. Constants written as their values

- **The pattern:** the backend writes "every integer constant as its value in
  its C type" (`JAVA.md`, *Milestone 1*), because C's constant arithmetic is
  not Java's. The rule reaches further than its reason:
  - **every case label is a number: 961 of 961.** `edit` (`Editor.java:20932`)
    is `case 27: case 3:` where the C says `case ESC: case Ctrl_C:`, and
    `case -18795:` is `K_INS`. The C's own labels are 347 characters, 493 names,
    209 expressions and 24 numbers; the Go writes 24 numbers (`case K_DEL,
    K_KDEL:`). The cause is one line: `switchStmt` hands `convK` the
    `*cc.ConstantExpression`, which `exprTo`'s "a name or a character stays
    one" test does not look through, so it folds (`java_stmt.go:690`,
    `java_expr.go:304`);
  - a constant expression of names is its value: `options[0].flags[0] =
    29700L` (`:12213`) is `P_STRING | P_VI_DEF | P_RCLR`, which the Go
    writes; `mark_adjust(lnum + 1L, 9223372036854775807L, ...)` (`:16559`) is
    `(linenr_T)LONG_MAX`, 55 times; `for (; i < 26; i++)` (`:33453`) is
    `'z' - 'a' + 1`; `vim_strnsize(s, 2147483647)` is `MAXCOL`;
  - a character that needs an escape is its code: `== 92` for `'\\'`
    (`rem_backslash`, `:18169`), `== 9` for `'\t'` (`skipwhite`, `:17825`).
- **Measured:** integer literals other than 0 and ±1, strings removed: 4,621 in
  the Java's method bodies against 3,212 in the Go's, and 14,595 in its
  initialisers against the Go's 6,751 -- about 9,000 numbers more than the Go
  writes from the same C.
- **What it becomes:** a case label as C spells it; a constant expression as
  C spells it **when Java's evaluation of it gives C's value** (the backend
  evaluates both: C's is `cc`'s `Value()`, Java's is 32-bit or 64-bit two's
  complement on the same tree, and they are compared at generation), and its
  value otherwise; `LONG_MAX`/`INT_MAX` as `Long.MAX_VALUE`/`Integer.MAX_VALUE`;
  every character constant as a Java char literal with its escape.
- **Who:** the printer. **Cost:** S. **Risk:** none: javac folds all of it,
  and the class files are byte-identical -- the proof is `cmp`, before any
  test. **Value:** high; the switch tables (`edit`, `normal_cmd`,
  `getcmdline_int`) become readable.

### 2. Parentheses

- **The pattern:** 23,779 parenthesised expressions. By Java's precedence
  (the counter's table): 8,003 are the syntax of an `if`, `while` or `switch`,
  4,105 are needed, and **11,127 are not** -- 8,014 around an operand that
  binds tighter (`((State & MODE_INSERT) != 0) || (restart_edit != 0)`),
  2,002 "clarifying" ones (`&&` inside `||`, the same operator on its left),
  999 around a whole assignment, argument or return, 112 around a name or a
  call; 172 are doubled (`if (!(((int) d.get()) != 0))`, `musl_strcpy`). PMD's
  conservative `UselessParentheses` flags 2,605.
- **What it becomes:** parentheses where precedence needs them, and -- as
  Java programmers write, and PMD's defaults allow -- around a `&&` inside a
  `||` and around a bitwise operand of a comparison (`(x & F) != 0`, which
  Java needs anyway).
- **Who:** the printer: `jparen` (`java_expr.go:32`) parenthesises every
  compound operand; the value it wraps would carry its precedence instead.
- **And line length:** the printer never breaks a line, and 1,693 lines of
  the methods are over 100 columns (1,014 over 120; Checkstyle's
  `LineLength`), most of them conditions. A condition broken before its
  top-level `&&` or `||` once it passes 100 columns is the idiom, and only
  whitespace: the same proof holds.
- **Cost:** S to M. **Risk:** none, and provably: byte-identical class files.

### 3. Locals at the top, and the loop's start before the loop

- **The pattern:** every local is declared at the method's top with its
  zero (`java_stmt.go:147`): **4,039 declarations** open their methods, 3,449
  of them `= 0`, `= null` or `= false` in 840 methods, which the body then
  overwrites -- PMD's `UnusedAssignment` counts 3,431 initialisers never used,
  and `PrematureDeclaration` 1,278. `skipwhite` (`:17822`) is `BytePtr p =
  null; p = q;`; `del_chars` (`:16742`) declares `bytes`, `i`, `p` and `l` at
  the top, writes `i = 0L;` and then `for (; (i < count) && ...; i++)`, where
  the C has `for (i = 0; i < count && *p != NUL; ++i)` -- **491 of the 608
  `for` loops** have their start moved out of them. 350 generator
  temporaries (`t1`) are declared there too.
- **Why it is so:** Java refuses a read it cannot prove assigned, and a C
  local declared without a value inside a loop keeps its last value at -O0.
  The Go had the same two constraints and solved them: `body_stmt.go:95`
  declares a local where C does, unless its function has a `goto`, it is a
  `case`'s own local, or it is uninitialised inside a loop. Only **10 Java
  methods** have a labeled block (the gotos), and they hold 127 of the 3,449.
- **What it becomes:** the Go's rule, with Java's twist: a local with an
  initialiser is declared with it where C declares it; one without, outside a
  loop, is declared there with its zero, as the Go's `var x T` is zeroed
  (and without it where every path assigns it first, which is javac's own
  definite-assignment rule, JLS 16, computed on the C as the backend already
  computes JLS 14.22); the 10 labeled-block methods keep their locals at the
  top, since a declaration inside a labeled block is out of scope after it.
  A `for` whose C start is an expression is written with it; `for (int i =
  0; ...)` where `i` is the loop's alone.
- **Who:** the printer. **Cost:** M. **Risk:** low, the Go's: a C local
  declared without a value in a loop and read before it is written is kept
  at the top, as the Go keeps it; javac refuses any declaration placed where
  a use cannot see it. **Value:** high: most of the 3,449 dead zeros and
  1,278 early declarations go, and the Java reads like the C's scoping.

### 4. Bytes: `(p.get() & 0xff) == NUL`

- **The pattern:** a C `char_u` is a Java `byte` holding its bits, so each
  read that widens it is masked: `& 0xff` **2,184 times** (1,147 on a
  `get()`), `& 0xffff` 101. But **1,200 of them** only compare for equality
  with a constant -- a named one 615 times (NUL 487 of them, then `TAB`,
  `ESC`, `KS_EXTRA`), a character literal 585 times -- and for a constant
  from 0 to 127 the mask changes nothing: `p.get() == NUL` is the same test.
- **What it becomes:** the printer drops the mask when the other side is a
  constant in 0..127; where a byte widens for arithmetic, the runtime gets
  `u()` and `u(k)` (`p.get() & 0xff`, `p.at(k) & 0xff`), so what is left reads
  `p.u(1) == CSI` or `c = p.u()`.
- **Who:** the printer, and `BytePtr` for `u`. **Cost:** S. **Risk:** low
  (a constant from 128 to 255 keeps its mask, which the rule tests); the
  bytecode changes, so the suites verify it.

### 5. Tables written a member at a time, and constant casts

- **The pattern:** the C's tables are initialised one member per statement:
  `foldCase[0].rangeStart = 65; foldCase[0].rangeEnd = 90; ...` (`:8206`) for
  `{0x41, 0x5a, 1, 32}`, and the option `ambiwidth` as six statements
  (`:12211`) where the C has one row. **9,251 statements for 4,121 rows**:
  `options` 970, `foldCase` 824, `toUpper` 796, `toLower` 732,
  `utf_iscomposing_combining` 708, `nv_cmds` 492. And Java's assignment
  conversion already narrows a constant that fits, so `utf8len_tab[92] =
  (byte) 1;` needs no cast: 1,165 `(byte)` and 329 `(short)` casts of
  constants are of that kind (and PMD's `UnnecessaryCast` finds 641
  widening casts to `int` or `long` that Java makes itself).
- **What it becomes:** each struct class gets a constructor of its members
  in order, and a table is an array initialiser, one row a line, with the
  row's constants as item 1 writes them: `new T_convertStruct(0x41, 0x5a, 1,
  32)`, `new S_vimoption(lit("ambiwidth"), lit("ambw"), P_STRING | P_VI_DEF
  | P_RCLR, ...)`; a table of bytes is `new byte[] {1, 1, ...}`. The
  64 KB method limit still applies, so the tables stay spread over
  initialiser methods, by table. Of the struct types, 11 are written
  nowhere after their initialisation (`T_convertStruct`, `S_interval`,
  `S_cmdname`, `S_nv_cmd`, `T_decomp_T`, ...), and those could be records.
- **Who:** the printer (`java.go`'s `structClass`, `initInto`). **Cost:** M.
  **Risk:** low: the objects made are the same, which a dump of the fields
  after construction compares, old against new.
- **Value:** the initialisers fall from 9,557 lines to about 4,300, and read
  as the C's tables do.

### 6. Out-parameters and the one-element arrays

- **The pattern:** Java has no address of a variable, so every scalar or
  pointer whose address the C takes is a one-element array from its
  declaration on, and each read is `x[0]` (`JAVA.md`, *Milestone 2*). **468
  objects are boxed, read `[0]` 6,509 times:**

  | boxed | objects | `[0]` reads | address sites |
  | --- | ---: | ---: | ---: |
  | struct members | 76 | 3,864 | 169 |
  | locals and parameters | 284 | 1,689 | 390 (382 of them a call's argument) |
  | globals | 108 | 956 | 156 |

  The members are the costly ones, and a few C sites cause most of them:

  | member | `[0]` reads | its address is taken in |
  | --- | ---: | --- |
  | `pos_T.lnum` | 1,120 | `mark_adjust_internal`, 11 times |
  | `pos_T.col` | 1,004 | `cursor_pos_info`, once (`getvcols(..., &min_pos.col, &max_pos.col, 0)`) |
  | `string_T.string` | 340 | `do_put`, once |
  | `vimoption_T.flags` | 235 | `did_set_option`, `set_option_default`, `do_set_option_string` |
  | `cmdarg_T.nchar` | 93 | `normal_cmd_get_more_chars`, once |
  | `exarg_T.cmd`, `.arg` | 81, 73 | 7 and 5 sites: `parse_command_modifiers` (6), `do_one_cmd` (2), four more |
  | `winopt_T.wo_*`, `buf_T.b_p_*` | 15 to 46 each | `get_varp` and `get_varp_allbuf`: the option table's variable pointers |

  `mark_adjust_internal` is vim's `one_adjust()` macro, expanded 13 times:
  `lp = &(curbuf->b_namedm[i].lnum); if (*lp >= line1 && *lp <= line2) ...`
  (`Editor.java:33454`, `lp = new LongPtr(curbuf.b_namedm[i].lnum, 0)`). So
  every `w_cursor.lnum` in the file is `w_cursor.lnum[0]`
  (`check_cursor_col_win`, `:38450`) because of one macro.
- **The parameters:** 162 parameters are `IntPtr` or `LongPtr`, and **152 never
  walk** -- read, written, tested for NULL, handed on: out-parameters
  (`getvcol(wp, pos, &start, &cursor, &end)` and `getvvcol`, called 48
  times). Of 105 `Ptr<>` parameters, 96 never walk (`char_u **arg`, which
  the callee advances).
- **What it becomes, in three parts:**
  1. **A C phase for `mark_adjust_internal` and `cursor_pos_info`** (15
     address sites): the macro as a function of the value, `x =
     one_adjust(x, line1, line2, amount, amount_after)`, and the two columns
     through locals.
     `pos_T.lnum` and `.col` are then unboxed: **2,124 `[0]`** go, and 36 more
     (`w_old_cursor_lnum`, `w_old_visual_lnum`). The C reads better too (a
     call for 13 copies of a macro), and the Go's `editor.go` changes only
     there. Cost S, risk none (`whim test`, all the editors).
  2. **Copy-in, copy-out at the call**, a printer rule with an analysis: a
     local or member whose address is only ever a call's argument, where the
     callee stores the pointer nowhere and neither it nor anything it calls
     names that object otherwise, is a plain variable; the call makes the
     box and writes it back (`int[] cs$ = {cs}; ... getvcol(..., new
     IntPtr(cs$, 0), ...); cs = cs$[0];`). That is 382 of the 390 local
     address sites and most member ones; it moves the noise from 6,509 reads
     to about 500 calls. Cost M-L (the "names it otherwise" analysis is the
     work; a call in the middle of an expression needs a helper), risk
     medium: a wrong "never otherwise" is a stale read, which the suite may
     not reach.
  3. The idiomatic end, **out-parameters as results** (`getvcol` returning a
     record of three columns), is a redesign of the C's signatures: 152
     parameters in a few dozen functions, each with its callers. Not
     recommended before 1 and 2 have been measured again.
- The option table's variable pointers (`get_varp`, 956 reads of boxed globals
  and the `wo_*`/`b_p_*` members) are the option machinery's design: they
  stay, and are item 6's residue.

### 7. `int` truth values the C still has

- **The pattern:** phase 166 made the core's yes-or-no functions, locals,
  members and parameters `bool`, and not the file-scope objects: **92** are
  still `static int x = TRUE;` or `= FALSE;` in `whim-vim.c` (`VIsual_active`,
  `msg_scroll`, `redraw_cmdline`, `exiting`). In the Java: `TRUE`/`FALSE`
  named 951 times, `x != 0` 1,952 times and `x == 0` 1,293 (620 of the 3,245
  are flag tests, `(x & F) != 0`, which stay), `VIsual_active != 0` 113, and
  `b ? 1 : 0` 183 (`bufref_valid`, `:16072`, `del_bytes`, `:16772`).
- **What it becomes:** phase 166's rule on file-scope objects (a global that
  only ever holds an answer, never `++`, `|=` or its address taken); the
  printer then writes `if (VIsual_active)`.
- **Who:** a C phase (`crefactor/xform`'s `BoolRet` extended), for both
  editors. **Cost:** M. **Risk:** low, as 166's was.

### 8. What else the C spells that no one would write

- **`_()`, gettext, is the identity** (`whim-vim.c:825`) and is called 441
  times: `gettext_(BytePtr.lit("E123: ..."))` 391 times in the Java, 399
  in the Go. A C phase drops the calls (the host names `gettext_`, so the
  function stays). S, none.
- **vim's ASCII class macros, inlined:** `(unsigned)c - 'A' < 26` on 109 lines
  of the C, which Java can only say as `Integer.compareUnsigned(c - 'A', 26)
  < 0` -- 137 of the Java's 150 `compareUnsigned` (`may_adjust_key_for_ctrl`,
  `:38934`; `musl_strcasecmp`, `:15668`, with four per line). A C phase names
  them again (`ascii_isupper(c)`, static and small, as vim's `ASCII_ISUPPER`
  was). S, none.
- **Two constant conditions:** `if (0)` in `win_update` (`:20446`) and
  `else if (!TRUE)` in `ex_z` (`:23430`), dead in both editors; phase 164's
  kind. S, none.

### 9. Side effects the Go had to split and Java need not

- **The pattern:** Java, unlike Go, has `i++`, `--len` and assignment as
  expressions, and evaluates left to right. The backend splits them as the
  Go must: `ccline.cmdbuff[i++] = ccline.cmdbuff[j++];` is five lines with
  `t1` and `t2` (`cmdline_erase_chars`, `:26800`); `special_to_buf`
  (`:38794`) writes `t1 = dlen; dlen++; dst.set(t1, (byte) -128);` three
  times; `while (*s != NUL && --len >= 0)` is a boolean temporary and an `if`
  (`vim_strnsize`, `:17434`). Counted: 400 temporary assignments -- 132 an
  integer's post-increment, 109 a pointer's post-walk, 51 a condition's
  boolean.
- **What it becomes:** an integer's `++`, `--` and `=` inline where C's order
  and Java's agree (the analysis's evaluation-order partition, `crefactor/ccx`,
  says where C leaves it open). A pointer's `*p++` keeps its temporary: the
  pointer classes are immutable, and `(p = p.add(1)).at(-1)` is worse.
- **Who:** the printer. **Cost:** M. **Risk:** low to medium -- evaluation
  order is the whole of it; the suites and `TestJavaControl2`-style controls.

### 10. Small printer rules

Each is S and none; the counts are the Java's today.

| Rule | Sites | Source |
| --- | ---: | --- |
| `@Override` on each struct class's `set` and `zero` | 210 | PMD `MissingOverride` |
| the struct's other object `o$` named `o` (a member named `o` renames instead) | 116 | PMD `AvoidDollarSigns` |
| the host methods' parameters by the C prototype's names (`musl_tty_keys(int p0, IntPtr p1, ...)` is `(int fd, IntPtr bs, ...)`) | 17 methods | `Editor.java:15521` |
| `!(a == b)` as `a != b` | 131 | PMD `LogicInversion` |
| a narrowing compound assignment made explicit (`n = (int) (n + count)`) | 40 | javac `lossy-conversions` |
| a C fall-through marked `// fall through`, and `@SuppressWarnings("fallthrough")` on its method | 23 in 10 switches | javac `fallthrough` |
| the 60 switches with no fall-through in arrow form (`case K_DEL, K_KDEL -> ...`): 364 grouped labels and 470 closing `break`s go; the 6 that only return (`handle_x_keys`, `get_varp_allbuf`, `can_bs`) as switch expressions | 60 | the counter |
| phase 173's `do { ... } while (false)` as a labeled block, as the other gotos are | 11 | the counter |
| an empty-bodied `for` as a `while` (`for (; s.get() != 0; s = s.add(1)) {}`, `musl_strlen`) | 40 | PMD `EmptyControlStatement` |
| every method `private` but the 17 host methods and the 10 the glue and `Printf.Core` call (`vim_main`, `deathtrap`, `emsg`, `iemsg`, `gettext_`, `utfc_ptr2len`, ...) | 1,710 package-private today | PMD `CommentDefaultAccessModifier` |
| a method that reaches no field of the editor, directly or through its callees, `static` | 249 | the graph |
| the class in a named package (`Profile.JavaPackage` exists and is unset), so `Whim` need not be in the unnamed one | 1 | PMD `NoPackage` |
| the class-level `@SuppressWarnings({"unchecked", "rawtypes"})` narrowed to the 7 sites | 7 | javac |

The annotations, the names, the fall-through comment, the explicit
narrowing (which is what `+=` compiles to already), `a != b` and the empty
`for` should leave the class files as they are, and the class-file check
says whether they do; the switch forms, the labeled block, `private` and
`static` change the bytecode and are verified by the suites.

### 11. One class, one file

- **The pattern:** `Editor` is 65,356 lines: 1,735 methods, 986 fields, 926
  constants, 105 nested classes, 11 interfaces. PMD calls it a `GodClass`,
  `TooManyMethods` and `TooManyFields`; the Go is the same shape (one
  `Editor`, its methods), by the instance pass's design.
- **Measured, the subsystems** (methods mapped to vim's files, the files to
  11 groups -- normal, text, screen, option, cmdline, buffer, regexp, term,
  window, input, message):
  - of 826 fields the methods name, **592 are named by one subsystem only**,
    114 by two, 47 by three, and 28 by six or more (`curwin` in 338 methods
    of 12 groups, `curbuf` 216 of 11, `State`, `got_int`, `IObuff`, `Rows`);
  - but **5,239 of 8,471 call sites** call into another subsystem (of the
    distinct caller-callee pairs, a group keeps 35% for `normal`, 26% for
    `cmdline`, at best 63% for `window`), and 3,710 field references would
    cross (2,941 of them to the 28 shared fields).
- **What it could become:**
  - **the file, split, with the class whole** -- the printer writes the struct
    classes, the interfaces and the constants as top-level classes of the
    package (the constants as a `final class` imported statically), and the
    table initialisers as a class of their own that fills an editor (item
    5): `Editor.java` falls to about 51,000 lines of methods and fields, and
    nothing else moves. S to M, none;
  - **the class, split by subsystem** -- eleven classes and 8,949 references
    qualified (`screen.win_update(...)`, `state.curwin`). It is item 11 of
    `GO-IDIOMS.md` again: 8,949 of the 20,489 references to methods and
    fields rewritten to say the same thing, for a split the call graph does
    not support (most calls cross). **Not recommended.**
- **Methods of a struct:** 99 methods take a `win_T *` and 55 a `buf_T *`
  first; as methods of those classes they would need the editor too (they
  read `curwin`, `p_*`), which a non-static inner class gives. It moves the
  same references around; not recommended for the same reason.

### 12. Names

- **The pattern:** Checkstyle's naming checks on `Editor.java`: `MethodName`
  1,563, `MemberName` 1,477, `LocalVariableName` 1,035, `ParameterName` 546,
  `TypeName` 105. 1,526 of the 1,735 methods are snake_case (`ml_get_buf_len`,
  `win_update`); 511 of the 746 struct members begin with a short prefix and
  an underscore (`b_`, `w_`, `ga_`); the classes are `S_file_buffer`,
  `S_window_S`, `T_pos_T`. Constants are ALL_CAPS already, as Java's are
  (870 of 926).
- **What it could become:** a name map in the printer: classes by their
  typedef in PascalCase (`buf_T` → `Buf`, `win_T` → `Win`, `pos_T` → `Pos`),
  which is the cheap part and costs nothing in traceability; methods and
  members in camelCase, each carrying its C name (`@C("del_chars")`, a
  source-retention annotation, or a one-line comment) so that `grep` from
  the C still lands. `Whim.java` and `Printf.Core`'s glue follow.
- **Who:** the printer, and the glue by hand. **Cost:** M. **Risk:** none
  behaviourally (javac checks every name); it gives up the Java's word for
  word match with `editor.c` and with `editor.go`, which is what makes the
  three editors comparable line by line. Do the class names; the rest last,
  if ever -- the Go's verdict (`GO-IDIOMS.md` item 13).

### 13. The runtime and the host

- **Idiomatic already:** `host/Host.java` is an interface in Java's types (a
  `record TtyKeys`, `byte[]`, offset and length), the embedding host's end is
  an exception (`Exit`, caught by `Whim.main`), `Printf` reaches the core
  through an interface (`Printf.Core`) and is tested without it, `Term` calls
  the C library through the FFM API. `javac -Xlint` finds nothing in them
  but the 9 `sun.misc.Signal` warnings (the price of Java having no other
  signal API) and FFM's restricted method.
- **Not:** the pointer classes are five near-copies (`BytePtr`, `ShortPtr`,
  `IntPtr`, `LongPtr`, `BoolPtr`), which Java's generics force until they
  hold primitives; their public `a` and `i` fields and static `eq` could be
  records (`record BytePtr(byte[] a, int i)`: a record's `equals` compares an
  array by reference, which is C's pointer equality), with `u()` of item 4.
  `Printf` has one unused private method (`formatOverflowError`,
  `Printf.java:415`: PMD `UnusedPrivateMethod`). `Term`'s nine `catch
  (Throwable t) { throw new Error(t); }` are the FFM idiom for
  `invokeExact` and stay.
- **PMD on the hand-written Java:** 1,636 findings, style almost all
  (`ShortVariable` 384, final parameters 319, `OnlyOneReturn` 104).
- **Who:** by hand. **Cost:** S. **Risk:** none (`SelfTest.java`, the suites).

## Ranking (value against cost)

| Rank | Item | Who | Cost | Risk | What it removes | Verify |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | 1: constants as the C names them | printer | S | none | 961 numeric case labels, ~9,000 bare numbers | identical class files; `whim-editor-check` |
| 2 | 2: parentheses by precedence | printer | S-M | none | 11,127 parentheses | identical class files |
| 3 | 3: locals where C declares them, the `for`'s start in the `for` | printer (the Go's rule) | M | low | 3,449 dead zeros, 1,278 early declarations, 491 split loops | `whim test --java`, `--wide --java` |
| 4 | 6.1: `one_adjust` as a function of the value, `cursor_pos_info`'s columns | C phase | S | none | 2,160 `[0]` reads, 2 boxed members | `whim-build-check` for the phase's own link, `whim test` (C, Go, `--java`) |
| 5 | 4: bytes compared without the mask; `u()` | printer + runtime | S | low | ~1,200 of 2,184 `& 0xff` | `whim test --java`, `--wide --java` |
| 6 | 10: the small printer rules | printer, glue | S each | none | the table's column | identical class files where marked; the suites |
| 7 | 8: `_()`, the ASCII macros, two dead branches | C phase(s) | S | none | 391 `gettext_`, 137 `compareUnsigned` | `whim test` (all three editors) |
| 8 | 5: tables as rows, constant casts dropped | printer | M | low | ~5,200 lines, 1,494 casts | a field dump after construction, old against new; the suites |
| 9 | 7: file-scope flags as `bool` | C phase (166's rule) | M | low | 951 `TRUE`/`FALSE`, 183 `? 1 : 0`, many `!= 0` | `whim test` (all three) |
| 10 | 11: struct classes, constants, tables in their own files; a named package | printer + glue | S-M | none | a 65k-line file | `whim java`, the suites |
| 11 | 9: side effects inline | printer | M | low-medium | 400 temporaries | the suites, a control per rule |
| 12 | 6.2: copy-in, copy-out for addresses only a call takes | printer + analysis | M-L | medium | most of 6,509 `[0]` reads | the suites, and a control |
| 13 | 12: Java names | printer name map + glue | M | none | 4,726 Checkstyle naming findings | javac; the suites |
| -- | 6.3 (out-parameters as results), 11's subsystem split | -- | L | -- | -- | not recommended |

### Recommended first three

1. **Items 1 and 2 together, proved by the class files.** Both are printer
   rules and neither changes a byte of what javac writes, so the evidence
   is `cmp` on `-g:none` builds -- worth making a check of its own (`whim
   java --same-classes`), since item 10's cosmetic rules reuse it. After
   them the switch tables read as the C's and almost half of the
   parentheses are gone.
2. **Item 3, the Go's declaration rule ported.** The largest single change
   in how the methods read; the rule and its one real risk are known from the
   Go, where it holds today.
3. **Item 6.1, one C phase.** Fifteen address sites in two functions box
   the two members every cursor reads; a phase that removes them takes
   2,160 `[0]` from the Java and reads better in the C.

## Not worth doing, and why

- **C strings as `String`, or as `byte[]` and an `int`.** `GO-IDIOMS.md`'s
  reason holds, measured again on the Java: of 674 `BytePtr` locals, 476
  escape (344 are handed to a call, 148 copied, 117 derive another pointer
  by `p.add(k)`, 60 are returned; a local may do several); 198 are cursors
  a method keeps to itself, which could be split into an array and an
  index, but `line[i]` beside 932 `BytePtr`s that stay is two models where
  one reads better. 1,130 locals and parameters, 84 fields and members and
  64 arrays of them are `BytePtr`; it is one pointer class, compared and
  subtracted (68 `eq`, 165 `sub`, 70 orderings), walked backwards, and
  edited in place. `BytePtr` *is* Java's idiom for it: an array and an
  offset.
- **Java enums for C enums.** Of the C's 859 enum declarations, 27 have more
  than one enumerator; the rest are `#define`s the preprocessor's removal
  made enums. The 27 are mostly indices (`HLF_*` into `highlight_attr`,
  `CMD_*` into `cmdnames`), bit sets (`TERM_SYNC_OUTPUT_*`) and codes used
  in arithmetic, with a few closed sets (`set_op_T`, `magic_T`,
  `mokstate_T`); an `enum` would add `.ordinal()` everywhere they are used
  as what they are. Grouping their constants by enum would help reading,
  and is naming (item 12).
- **Exceptions for FAIL.** Phase 166 made OK and FAIL `true` and `false`
  (59 of the names are left, and 285 methods return `boolean`); a failure
  has already said why through `emsg`, and carries nothing for an exception
  to hold. `Exit` is the one exception the editor needs, and it has it.
- **`try`/`finally` for the C's cleanups.** Phases 170-173 turned the
  cleanup gotos into tails and blocks; 10 labeled blocks and 49 `break`s are
  left, and a labeled `break` is Java's own forward jump. A `finally` would
  run on `Exit` too, which the C's cleanup never does.
- **The growarray as `ArrayList`.** 107 typed accessor calls and 112 uses
  of `ga_len`, with element pointers taken into the storage and walked; an
  `ArrayList` needs the C rewritten to accessors first, for a class whose
  whole use is two fields and `ga_grow`. The accessors are small and typed
  already.
- **Records for the structs.** They are mutable and assigned by copy (`set`
  700 times); only the 11 written-once table types of item 5 qualify.
- **The unsigned helpers.** `Integer.compareUnsigned` and its kin (240 calls,
  137 of them item 8's macros) and `>>>` (49) are how Java says unsigned
  arithmetic. The widths stay, for the Go's reason (overflow guards written
  for 32 bits).
- **Splitting large methods.** `regmatch` (1,054 lines), `win_line` (787),
  `regatom` (777) are the C's; as in the Go, high risk for no semantic gain.

## The throwaway files

All under `.tmp/jid/` in the survey's worktree, none tracked:

- `count/JCount.java`: the tree-API counter; `out/counts.txt`, `methods.tsv`,
  `boxes.tsv` (each boxed object: reads, address sites), `params.tsv`,
  `bplocals.tsv`, `litdst.tsv`, `edges.tsv`, `sites.tsv`, `samples.txt`;
- `graph/main.go`: the subsystem graph; `vimfuncs.tsv`, `mfile.tsv`: vim's
  functions by file, and each method's file;
- `pmd-all.csv`, `pmd-hand.csv`, `cs-google.txt`, `cs-sun.txt`,
  `cs-hand.txt`, `xlint.txt`, `xlint-nosup.txt`: the linters' raw output;
- `bc/`: the class-file identity experiment;
- `tools/`: PMD 7.16.0, Checkstyle 10.26.1, Temurin 21 (Alpine).
