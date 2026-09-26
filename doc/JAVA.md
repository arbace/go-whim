# The editor in Java: design and milestones

The Java editor is called **braaam** (`braaam/`; `bin/braaam`, `braaam.jar`).

`AGENDA.md`'s one queued item. The C core (`editor.c`) is translated to Java by a
second backend of `crefactor/togo`, from the same analysis the Go is written
from -- not by translating the Go -- and held to the same test: `whim test`
runs the Java editor on every case and requires it to answer as the C does.
Written 2026-09-25, when the Go editor was an `Editor` instance on a `Host`
(`editor/`), and the C had 49 gotos, each out of a loop or switch.

## What Java needs that Go did not

Measured on `editor/editor.go` (2026-09-25):

| C / Go | count | Java |
|---|---:|---|
| `Ptr[byte]`, a C string that walks or is compared | 2,212 of 2,307 `Ptr` | `BytePtr`: a `byte[]` and an offset. Java generics hold no primitives, so there is one class per element kind -- `BytePtr`, `ShortPtr`, `IntPtr`, `LongPtr` -- and `Ptr<T>` over a `T[]` for structs |
| a pointer to a scalar that never walks (`&local`) | -- | the same classes over a one-element array: Java has no address of a local, so an address-taken local is that array from its declaration on |
| unsigned arithmetic | 570 uses | the bits of `int`/`long`/`short`/`byte`, with `Integer.divideUnsigned`, `remainderUnsigned`, `compareUnsigned`, `>>>`, `toUnsignedLong`, and `& 0xff` / `& 0xffff` where a narrow unsigned value widens |
| structs by value | 99 types | classes; C's assignment of a struct is a copy (`a.set(b)`), a struct member of a struct is its own object, made with it |
| function pointers | 24 types, 165 functions used as values | a functional interface per signature, and method references (`this::ex_edit`), bound to the instance as the Go's method values are |
| `goto` | 49, each forward, out of a loop or switch, to a label of an enclosing block | a labeled block, `L: { ... break L; ... }`, the label's statement after it: Java's `break` leaves a labeled block from inside any loop or switch in it |
| `switch` | -- | Java's `switch` falls through as C's does; a `case` must be a constant, as it is in C |
| variadic `vim_snprintf` | 174 calls | `Object...`, as the Go's `...any` |
| package state | 821 variables | fields of `Editor`, the functions reaching them its methods -- the instance the Go has since `togo`'s instance pass |

Java's own limits, which the emitter must respect:

- **A method's bytecode is at most 64 KB.** The Go's `initGlobals` is 3,543 lines
  of table initialisation, and `regmatch` 1,008: the initialiser is written as
  several methods, and a function too large is refused with its size until a
  rule splits it.
- **A class's constant pool holds at most 65,535 entries.** One `Editor` class
  with 1,470 methods, 821 fields and 1,627 string literals may reach it: the
  literals go to a class of their own, and the core may be split into
  classes by the call graph if the count says so.

## Where it lives

- `crefactor/togo/java*.go`: the backend, in the same package as the analysis
  it reads (`an`, the pointer classes). A mode of `Run`:
  `whim skel <editor.c> <dir> -java <dir>/Editor.java` writes the class,
  named after the file, and beside it `Editor.java.refused`, every function
  refused with its reason. `java.go` is the frame (types, struct classes, the
  class, the report), `java_expr.go` the expressions, `java_stmt.go` the
  statements and methods; `java_test.go` the tests.
- `braaam/rt/`: the runtime, by hand, package `whim.rt` -- `BytePtr`,
  `ShortPtr`, `IntPtr`, `LongPtr`, `BoolPtr`, `Ptr<T>`, `Rt` (`memmove`,
  `memset`, `memcmp` on bytes and in elements, a char array's initial value,
  a `void *` read back), `Ga` (the growarray's storage, typed at its use) and
  `Struct<T>` (what every struct class can do: `set`, `zero`) -- as
  `editor/crt.go` is the Go's, and `SelfTest.java`, its own test with no
  framework.
- `braaam/`: the rest of the editor in Java, by hand -- `host/` (package
  `whim.host`: the `Host` interface, `Exit`, `Printf` -- vim_snprintf --, and
  `Term` and `Signals`, the terminal host, which calls `ioctl`, `select`,
  `read` and `write` through the Foreign Function & Memory API, JDK 22 and
  later, as the Go's `syscall` does) and `Whim.java` (the glue, a subclass of
  `Editor`, and the launcher's `main`) -- and `braaam.go`, the Go that builds
  it: `Editor.java` generated from a `whim-vim.c`, compiled with the rest, and
  a launcher script. `Editor.java` is generated and tracked (`braaam/Editor.java`,
  milestone 4); `make bin/braaam` compiles it under `lib/braaam/`, and `make braaam.jar` packs the classes as
  `./braaam.jar`. *Milestone
  3, as built*.

The JDK on the machine is 26.

## Milestones

Each is verified before the next starts.

1. **A vertical slice on foreign C.** The runtime, and the emitter for the
   integer types signed and unsigned, strings as `BytePtr`, arrays, structs
   by value, control flow with `switch`, globals as fields and functions as
   methods. Refused constructs are named per function, as the Go's `-bodies`
   does. Verified by tests that translate small C programs, compile them with
   `javac`, and require the Java to print what the gcc-built C prints -- with
   a wrong translation as a control. And a coverage report on `editor.c`:
   functions written, and the refusals by reason -- the measure of the work
   left. **Done**; see *Milestone 1, as built*.
2. **The rest of the constructs** the coverage report names, by frequency
   (*The work list* below): the variadic call, the growarray, the functions
   of bytes on what is not bytes, the address of a member, function
   pointers, unions, the labeled-block `goto`. The measure: every function of
   `editor.c` written, and `javac` compiling `Editor.java`. **Done**; see
   *Milestone 2, as built*.
3. **The host and the launcher**, and `whim test` running the Java editor: the
   45 cases, then `--wide`, required to answer as the C does. **Done**: the
   host, the launcher and the suite's hookup (*Milestone 3, as built*), and
   with milestone 2 the Java editor answers all 45 cases and all 240 wide
   ones exactly as the C does.
4. **Kept current:** `make whim-build` writes `Editor.java` as it writes
   `editor.go`, and a check refuses a stale one. **Done**: `whim gen` writes
   `braaam/Editor.java` beside `editor/editor.go` from the same `editor.c`,
   and refuses outright if the Java backend refused any part of the core;
   `whim gen --check` (`make whim-editor-check`) refuses a stale one --
   measured with a control, one byte of the tracked file changed. The build
   writes it through `whim gen`, as it does `editor.go`.

## Milestone 1, as built

**The tests** (`crefactor/togo/java_test.go`, skipped without `gcc`, `javac`
or `java`): five C programs that name nothing in vim -- the integer types and
their conversions, control flow, C strings and arrays, structs, and what a
profile tells (allocators, frees, the functions of bytes) -- each translated
with every function written, compiled with the runtime, run, and required to
print what the gcc-built C prints. The C calls two host functions it only
declares, `out` and `outs`; the C harness defines them with `printf`, the
Java one as the abstract class's methods. The unsigned cases include
wraparound, division and remainder, comparison with a negative int, right
shifts of every width, widening and narrowing of unsigned char and short, and
compound assignments through pointers. The **control** undoes one rule at a
time in the generated Java -- unsigned division as Java's signed `/`, `>>>`
as `>>`, an unsigned char widened with its sign, a struct assigned by
reference instead of `set` -- and each must move the output: all four do.
Another test requires a function pointer, a union and a goto refused by name,
with the rest of the class compiling and running; and `SelfTest.java`, the
runtime's own, is run by another.

**Measured on `editor.c`** (73,632 lines, 2026-09-25): of its **1,701**
function definitions, **1,457 written and 244 refused**. Of the **1,683** the
Go translates -- the other 18 its runtime replaces (`Profile.Runtime`) --
**1,448 written, 235 refused**; of those 18 the Java writes the 9 C string
functions and refuses the 9 that handle a `void *` (the allocators,
`musl_mem*`, `ga_grow_inner`). Of the file-scope objects, 10 initial values
are refused: 3 tables of function pointers (`cmdnames`, `nv_cmds`,
`options`), 5 compound literals (`redobuff` and its kin) and 2 addresses of
members (`filltab`, `lcstab`).

**It compiles.** `Editor.java` is 47,022 lines -- 98 struct classes, the
fields, 12 `initGlobalsN` methods, 17 abstract host methods, the 1,457
methods and 244 stubs, each a refused function's signature and a body that
throws `UnsupportedOperationException("refused: <why>")`, so that its callers
compile. `javac -Xlint:all` compiles it with the runtime in 2.5 s, no error,
46 warnings: 26 lossy compound assignments (C's truncation, meant) and 20
fall-throughs (C's, meant). Neither limit was reached: no method is over
64 KB, and the constant pool holds. What compiles is not yet shown to be the
editor: that is milestone 3, when the host runs it.

**How the slice maps C**, and where it departs from the table above:

- An integer is the Java primitive of its size, holding its bits; `_Bool` is
  `boolean`, and a comparison is a `boolean` until an int is wanted
  (`c ? 1 : 0`). Every integer constant is written as its value in its C
  type: C's constant arithmetic is not Java's.
- **Every pointer to a scalar is a runtime pointer class, whether it walks
  or not** -- the analysis decides nothing there, since Java has no other
  address of a scalar. It decides only for pointers to structs: a plain
  reference to the struct's class, or a `Ptr<S>` over an `S[]` when the
  class walks. A pointer to a pointer is a `Ptr<...>` over the slots.
- **An array is always a Java array** -- `byte[]`, `int[]`, `S[]`,
  `BytePtr[]` -- never a `BytePtr` variable; it decays into a pointer over the
  same storage (`new BytePtr(a, 0)`), so `buf[i]` is Java's own index and
  a pointer into it compares equal to the array's.
- NULL is `null`; the pointer classes are immutable values, `p++` is
  `p = p.add(1)`, and `==` is `BytePtr.eq(p, q)`.
- **Every local is declared at the method's top**, with its zero or its
  storage: Java refuses a read it cannot prove assigned, and C's local in a
  loop keeps its value from one turn to the next (at -O0), which one
  declaration does too. A braced initializer makes the object anew where C
  has it, since C zeroes what the list leaves out.
- A scalar or pointer whose address is taken -- local, parameter, global or
  static -- is a one-element array from its declaration on; a parameter is
  copied into one as the method starts.
- Java refuses a statement it proves unreachable and a method that may end
  without a return: the backend computes Java's rule (JLS 14.22) on the C,
  writes nothing after a statement that cannot complete (dead in C too), and
  ends a method that can fall off its end with a `throw`.
- A condition or an increment that does something first is written before
  its test; a `continue` that must reach it leaves a labeled block around
  the body, `c1: { ... break c1; ... }`. No `goto` is written: the eight
  functions that have one are refused, for milestone 2.
- The initial values are written into `initGlobalsN` methods of at most 400
  lines each, which the design above foresaw for the 64 KB limit.
- The profile's allocators, frees and functions of bytes (on bytes) are in
  already: `(T *)alloc(sizeof(T))` is `new S_T()`, `k * sizeof(T)` an
  `S_T.array(k)` walked by a `Ptr`, bytes a `BytePtr.alloc(n)`; a free is
  nothing. An allocator is sized by its first argument, as the profile
  says, and an uncast one takes its type from its destination.
- A function declared and not defined is the host's: an abstract method, so
  the class is abstract, and the host a subclass.
- Struct classes are named `S_<tag>`, `T_<typedef>` for a typedef of an
  anonymous struct, `A_<n>` for one with neither. The class is in the unnamed
  package unless `Profile.JavaPackage` names one.

### The work list for milestone 2

The 235 refusals of the 1,683, by reason, most frequent first -- each function
counted once, for the first construct that stopped it, so fixing one reason
may uncover another in the same function:

| # | reason | functions | what it takes |
|--:|---|--:|---|
| 1 | a variadic call | 75 | every one is `vim_snprintf`: `Object...` and a formatter, as the Go's `format.go` |
| 2 | a void pointer | 49 | 48 are the growarray's `ga_data`, 1 `host_alloc`'s signature: the growarray, typed at its first use as the Go's `GaData[T]` |
| 3 | memmove, memset or memcmp of no bytes | 34 | on structs, ints, arrays of them: element copies, `set(new S())`, `Arrays.fill` |
| 4 | the address of a member | 31 | `&buf->b_p_xx` handed on as an `int *` or `char_u **`: a member whose address is taken as a one-element array, as a local's is (and 2 initial values) |
| 5 | a function pointer | 20 | a functional interface per signature and method references (and the 3 tables' initial values) |
| 6 | a union | 18 | the option callbacks' old and new values, `attrentry_T`'s `ae_u`, the regexp engine's saved positions |
| 7 | a goto | 8 | the 49 gotos: a labeled block, `L: { ... break L; ... }` |

and 5 compound literals among the initial values.

## Milestone 2, as built

**Measured on `editor.c`** (73,632 lines, 2026-09-25): of its **1,701**
function definitions, **1,693 written, 8 the Java runtime's, 0 refused**, and
no initial value refused. The 8 are the allocators (`alloc`, `alloc_clear`,
`lalloc`, `lalloc_clear`) and `musl_memmove`, `musl_memcpy`, `musl_memset`,
`musl_memcmp`: every call of them is written as the runtime's
(`BytePtr.alloc`, `new S()`, `Rt.memmove`, ...), no call reaches a method,
and none is written -- as the Go's runtime replaces them. Of the 18 the Go's
runtime replaces, the Java writes the other 10: the 9 C string functions,
translated, and `ga_grow_inner`, a rule of the profile's
(`RuntimeBody.Java`, in `internal/whim/gen.go`).

**It compiles and it is the editor.** `Editor.java` is 65,356 lines: 105
struct and union classes, 11 functional interfaces, the fields and 165
function-value fields, 15 `initGlobalsN` methods, the 17 abstract host
methods -- unchanged from milestone 1, so `braaam/Whim.java` is too -- 9
growarray accessors and the 1,693 methods. `javac -Xlint:all` compiles it
with the runtime in 3.2 s, no error, 63 warnings: 40 lossy compound
assignments and 23 fall-throughs, C's own. The constant pool holds 18,355
entries; no method is over 64 KB. `whim test --java`: the Java editor answers
all **45** cases exactly as the C does, its control seen by 41 (the Go's
too); `--wide --java`: all **240**, the control seen as the Go's is (94 keys,
6 pty). The Java cases take 1.6 s and 7.3 s.

**What each construct became**, and why:

- **A variadic call** (`vim_snprintf`, 174 calls): the arguments past the
  parameters as `Object...` receives them, C's default promotions and then
  boxed, as the Go passes `...any` and as `host/Printf.java` reads them -- a
  char, short or int an `Integer` (an unsigned char or short widened without
  its sign first), an **unsigned int zero-extended to a `Long`**, so that it
  reads back as C's would under `%u` and `%lu` alike, a long a `Long`, a
  pointer its class, an array the pointer it decays to, NULL a null
  `Object`. A constant is written as a number: `'o'` would box as a
  `Character`.
- **A `void *` is an `Object`.** The core has two: `host_alloc`'s result,
  cast where it is used (`(BytePtr) host_alloc(n)`, so the host must return
  a `BytePtr`), and the growarray's `ga_data`, which follows the Go's
  `GaData[T]`: where the C casts it -- or converts it uncast, to a parameter
  or a variable -- it is a static accessor of the Java type it becomes,
  `GA_IntPtr(gap)`, `GA_Ptr_T_hl_group_T(gap)`, which asks `Ga` for storage
  of that type as large as `ga_maxlen` (`Profile.GrowArray.MaxLen`), made at
  the first typed use or grown there, and keeps it in `ga_data`. So
  `ga_grow_inner` only sets `ga_maxlen`, and the next typed use grows the
  storage (the Go grows it at once when it exists, which no C can tell). A
  growarray used as two element types is a `ClassCastException`. An
  `Object` read back as a plain struct takes the one object a `Ptr` in it
  points at (`Rt.obj`), and as a `Ptr` wraps a plain one (`Rt.ptr`).
- **memmove, memset and memcmp** in the elements the C's pointers point at
  before their cast to bytes (`&a` of an array is the array's): on bytes as
  before; on shorts, ints and longs `Rt.memmove` and `Rt.fill` with the
  fill byte repeated through each element (`memset(p, 0xff, ...)` of ints is
  -1s); on pointers a copy or a NULL fill of the references; on structs
  `set()` element by element (`Rt.moveStructs`, from the end when the two
  overlap that way, so each element keeps its own object), `zero()` for a
  fill of 0, and member by member for a fill of another byte through a struct
  of scalars (`lpos_T`, 0xff); memcmp of two structs, where only equality is
  asked, `eq()`, written for the classes that are compared (one, `hl_group_T`,
  and what it holds). The struct classes implement `whim.rt.Struct<T>` for
  it.
- **The address of a scalar or pointer member** (`&buf->b_p_ts`): the
  member is a **one-element array in its struct**, as an address-taken local
  is, and `&s->m` a pointer over it -- 73 members. Chosen over an
  accessor-backed pointer class because the pointer classes stay the one
  representation every other pointer to a scalar has (a `BytePtr`, an
  `IntPtr` over an array), so `int *` from a member and from a local mix
  freely, compare with `eq` and walk nowhere; the cost is `[0]` on every
  read of those members. A struct's copy copies the value into its own
  array.
- **Function pointers**: a functional interface per Java signature,
  `interface FnN { R call(...); }` -- two C function types that Java writes
  alike are one interface, since Java's types are nominal -- keyed as the
  Go's are (`fp:<object>`); a function used as a value is a field of the
  class holding the method reference, **one per function and interface**
  (`final Fn3 fp_ex_edit = this::ex_edit;`), so two uses of one function
  are one object and compare equal with `==`, as C's pointers do (a method
  reference made at each use would not). Where the analysis gave the
  interface's parameters other classes than the method's (a plain struct
  reference against a `Ptr`), the field holds an adapter lambda, as the Go
  has a closure: one, `cmp_keyvalue_value_n` for `keyvalue_bsearch`. A call
  through one is `fp.call(...)`; NULL is null. The tables `cmdnames`,
  `nv_cmds` and `options` are initialised with the fields.
- **Unions**: a class like a struct's (`U_<tag>`, or `T_`/`A_` as structs
  are), **every member its own field**, as the Go's union is: the core
  writes a member and reads that member back (the option callbacks' old and
  new values, `ae_u`, the regexp engine's saved positions); `set()` copies
  them all.
- **goto**: all 49, in 8 functions, are forward jumps to a label that is an
  item of a block holding the goto (verified: a goto that is not is refused,
  by name). The items from the first that holds a goto to `L` up to `L`'s
  are a **labeled block**, `L_x: { ... }`, the goto `break L_x;` and the
  label's statement after the block -- 10 blocks. Two blocks that would
  cross are nested by starting the later where the earlier starts. A label
  an earlier item of its block jumps to makes what follows it reachable
  again, in the emitter's JLS 14.22 rule too; a label nothing jumps to does
  not -- the code after a jump stays as dead as C has it, and nothing of it
  is written (`TestJavaUnusedLabel`, which the old rule, any label live,
  fails with javac's "unreachable statement").
- **Compound literals** (`(char_u [1]){NUL}` in `redobuff` and its four
  kin): storage of their own, made where C evaluates them.
- **A struct held plainly compared with a `Ptr` over structs** (`rp ==
  &((regitem_T *)regstack.ga_data)[regstack.ga_len - 1]`, with `ga_len`
  0): `Ptr.is(p, x)`, the element `p` points at is `x`, and a `p` outside
  its array points at nothing -- converting `p` to a plain reference would
  read `[-1]`. The only such comparison in the core; the first thing the
  editor stopped at once every function was written.

**The tests** (`crefactor/togo/java_test.go`), each a C program that names
nothing in vim, translated with every function written, compiled, run, and
required to print what gcc's build prints: `TestJavaVarargs` (every integer
width, signed and not, pointers, arrays, characters, booleans, through the
harness's own printf of `Object...`), `TestJavaGrowArray` (a growarray of
ints, structs, pointers and bytes, typed, uncast and cleared, with its own
`ga_grow_inner` rule; the functions of bytes on all of them, overlapping
both ways; memcmp of structs; a `void *` there and back; compound literals;
`Ptr.is`), `TestJavaPointers` (member addresses from code and from a table's
initial value, a struct copied past them; tables of function pointers,
calls, comparisons, NULL, `&f`, a function returning one; unions, anonymous
and nested, designated and positional) and `TestJavaGoto` (out of nested
loops, out of a switch and a loop in it, to the next label, to an empty
statement at a loop's end, crossing blocks). **`TestJavaControl2`** undoes
one rule per construct and requires the output to move: an unsigned char
passed with its sign, structs moved as references, a member's address that
copies it, a union copied without one member, a pointer to another
function, a goto's break that leaves the wrong block -- all six move.
`TestJavaRefuses` names what is left to refuse: floating point, a variadic
definition, a goto that is no forward jump. `SelfTest.java` checks `Ga`,
the element-wise functions, `Ptr.is` and `Rt.obj`/`Rt.ptr`. The Go does not
move: `whim gen --check` is clean.

## Milestone 3, as built

Written 2026-09-25, beside milestone 2 (in progress in `crefactor/togo`), on an
`Editor.java` with 1,457 of 1,701 functions written.

**The pieces**, each the Java of a Go file in `editor/`:

| Java | Go | what |
|---|---|---|
| `host/Host.java` | `host.go`'s `Host` | what the core needs of the world, in Java's types: `winSize()` is `{rows, cols}` or null, `ttyKeys(fd)` a record or null, input and output a `byte[]`, an offset and a length |
| `host/Exit.java` | `editor.Exit` | what a host that must not end the process throws in `exit`; `Whim.main(host, args)` catches it and returns the code |
| `Whim.java` | `host.go`'s glue, `New`, `Main` | `final class Whim extends Editor`: each of the 17 abstract host methods, in the C's signature, a line of glue to the Host -- in the unnamed package, since `Editor` and its package-private methods are; `host_alloc` with the arena's accounting (1 GiB, 16-byte rounding, the exhaustion message) returning `BytePtr.alloc(n)` as the `Object` the core casts; `main(Host, byte[][])` and the launcher's `main(String[])` |
| `host/Printf.java` | `format.go` | vim_snprintf, line for line, given the core's `gettext_`, `emsg`, `iemsg`, `IObuff`, `emsg_iobuff_room`, `iobuff_or`, `e_val_too_large`, `utfc_ptr2len` and `utf_ptr2cells` through an interface, `Printf.Core`, which `Whim` implements -- so it runs, and is tested, without an `Editor` |
| `host/Term.java`, `host/Signals.java` | `term/term.go` | the terminal host |
| `braaam.go` | `cmd/whim/main.go`, `make bin/whim` | the build: `Cut` (the Makefile's cut of `editor.c`), `Build` (cut, generate, compile, launcher), `Compile`, `WriteSources` (the Java sources are embedded) and `WriteLauncher` |

**The arguments of vim_snprintf**, which milestone 2 writes as `Object...`:
an `Integer`, `Short` or `Long` sign-extends and a `Byte`, `Character` or
`Boolean` zero-extends, then the conversion keeps the width it names (so an
unsigned int's bits in an `Integer` print right under `%u` and `%x`; a C
`unsigned int` passed to `%lu` must be boxed as the `Long` C would have
converted it to); `%s` takes a `BytePtr`, a `byte[]` or a `String`, and null
is NULL. `%p` prints an address no C run would, as the Go's does.

**The terminal host** calls the C library the JVM runs on (musl here) through
`java.lang.foreign`: `ioctl` (a variadic downcall, TCGETS, TCSETS and
TIOCGWINSZ on the kernel's `struct termios`, as the Go's), `select`, `read`,
`write`, `pipe2`, `kill`, `getpid` and `sigpending`, errno captured with
`Linker.Option.captureCallState`. The signals are `sun.misc.Signal`
(module `jdk.unsupported`: no flag is needed to use it, and javac warns, as it
must, that it is internal API; `jdk.internal.misc.Signal` would need
`--add-exports java.base/jdk.internal.misc=ALL-UNNAMED` for nothing more).
What differs from the C, each marked DEVIATION in the source:

- **A handler runs on a thread of the JVM's**, not on the thread it
  interrupted: Java has no asynchronous handler. As in the Go, the handler
  sets the flag the C handler sets and writes a byte to a wake-up pipe that
  every wait of the host also selects on, since the JVM installs its handlers
  with SA_RESTART and no call of ours returns EINTR. The handler runs a moment
  after the signal arrives, where the C's runs at once: a check made in that
  moment does not see it yet (the host's own test waits for it).
- **SIGHUP and SIGTERM** run the core's `deathtrap` on the core's thread at the
  next wait, read or sleep of the host, not wherever the core was; as the Go.
- **`raise` of a signal the host catches** runs its handler's effect directly.
- **`suspend`** sets SIGTSTP to SIG_DFL, sends it to the process group, and
  puts the handler back only once the signal is no longer pending
  (`sigpending`, at most a second): the JVM is many threads, and the kernel may
  hand a process-directed signal to another of them a moment after `kill`
  returns -- measured: with the handler put back at once, the process was not
  stopped and read the signal as a key. The C is one thread and stops before
  `kill` returns.
- **The JVM keeps some signals** (SIGSEGV, SIGBUS, SIGFPE, SIGILL, SIGQUIT and
  its own); the editor catches none of them. SIGINT, SIGTERM and SIGHUP are
  taken from the JVM's shutdown hooks, which the editor does not use; `-Xrs`
  would forbid it, and the launcher does not pass it.
- **`exit`** is `System.exit`, as the Go's `os.Exit`.
- **The arguments** are the bytes the launcher was handed: the last entries
  of `/proc/self/cmdline`, since Java's `String[] args` are decoded and a byte
  the platform's charset cannot decode is lost there; argv[0] is the launcher
  script's `$0` (`-Dwhim.argv0`).
- **The core runs on a thread with a 1 GiB stack**, where the C has the
  process's 8 MiB: an interpreted frame is larger than a compiled one, and
  vim recurses. A recursion the C would crash on is a `StackOverflowError`.
- **An exception the core throws** -- a stub, or a null or an index where the C
  would read what is there -- ends the editor with its first frames on stderr
  and status 70, where the C would crash or go on.

**The launcher** is a shell script, `exec java ... -cp <classes> Whim "$@"`, so
that the JVM is the process its caller started (its pid, its process group,
its signals), with `--enable-native-access=ALL-UNNAMED` (without it the JVM
prints a warning on the editor's screen), `-XX:-UsePerfData` (no hsperfdata
file in /tmp for every run), `-XX:+UseSerialGC -XX:TieredStopAtLevel=1`
(measured, a run to the stub: 215 ms without them, 188 ms with), and
`-XX:-DontCompileHugeMethods`: HotSpot does not compile a method over 8,000
bytes of bytecode, and `regmatch` is one (in the Clojure editor, 60 hot
functions are); a heavy session, 5,000 lines and three substitutions, took
1.5 s without it and 1.0 s with it (`whim test`'s heavy case, which times
every editor and fails one over 25 times the C's time). An AppCDS
archive (`-XX:+AutoCreateSharedArchive`) was tried and dropped: the archive is
not written when the editor ends with `System.exit`, and the JVM then prints
its error on stdout. **`make bin/braaam`** (`go tool whim java [--out DIR]
[FILE]`) writes it: `editor.c` cut from `src/whim-vim.c`, `Editor.java`
generated with the profile `whim gen` uses, compiled with the rest in 5.0 s
(javac 2.5 s of it) into `lib/braaam/classes`, and `bin/braaam`. JDK 26
compiles it with no error; under `-nowarn` the only warnings javac still
prints are the nine mandatory ones for `sun.misc.Signal` above.

**`whim test --java`** (`make whim-test-java`), and `--wide --java`: after
the C and the Go comparisons, the candidate's `whim-vim.c` is built into the
Java editor (`internal/suite/java.go`, beside the four other builds), and
every case is run on it and on the C candidate, in parallel, required to
answer with the same bytes and status. **Its control is its own**: `" INSERT"`
spelled `" INSERX"` in the generated `Editor.java` (required to be there
once), compiled apart, which must move at least one case *of the Java
editor's own answers* -- measured against the C, a Java editor that stops
before it draws would see every control. What stops the Java editor is
counted from its stderr, by method and exception. The default suite is
unchanged and green. Measured: `whim test --java` 9.5 s (the Java cases, both
builds, 1.1 s), `--wide --java` 12.7 s (5.1 s).

**The tests**: `editor/format_java_test.go` runs `Printf` (with
`editor/testdata/PrintfProbe.java`) and `format.go` on 37 formats -- flags,
widths and precisions given and taken from arguments, every length and
conversion, `%S` with double-width characters, positional arguments, NULL,
truncation down to a size of 0, and the five format errors, whose messages
the Java must report -- and requires the same return value and the same bytes
in the buffer, its whole length; with `fmtbase('o')` changed to 10 as a
control, it fails on 2. `internal/suite/java_test.go` runs `Term` (with
`internal/suite/testdata/TermProbe.java`) on a pseudo-terminal 30x100 and on a
file: the size, the modes, input with and without a timeout, SIGWINCH raised
and SIGWINCH, SIGINT, SIGTSTP and SIGTERM delivered by kill(1), a delay, a
suspend (stopped and continued on the file; discarded by the kernel on the
pseudo-terminal, whose probe leads an orphaned process group), stderr and the
exit status -- 4 runs of 4 alike. `braaam/braaam_test.go` requires `Cut` to
give the Makefile's bytes, and `whim java` to build a launcher that runs.

### How far Editor.java runs

**Every case, as the C.** With milestone 1's `Editor.java` the class loaded
and its state was made, and `vim_main` was a stub (its `CLEAR_FIELD(params)`,
a memset of `mparm_T`); all 45 quick cases and all 240 wide ones stopped
there. Tracing `whim-vim.c` built with `-finstrument-functions` gave
milestone 2 its order: the session `ihello<Esc>:q!` called 95 stubs, first
`vim_main`, `estack_init` and `ga_grow_inner` (the growarray),
`cmdline_init` (memset), the allocators, the option functions (the address
of a member), `set_string_option_direct` (vim_snprintf); the 45 cases 136
stubs -- 34 a void pointer, 32 a variadic call, 26 the functions of bytes,
21 the address of a member, 13 a function pointer, 7 a goto, 3 a union --
and the refused initial values (`cmdnames`, `nv_cmds`, `options`,
`redobuff` and its kin, `filltab`, `lcstab`) were read at startup. With
every function written, the one thing left was `Ptr.is` (*Milestone 2, as
built*); `whim test --java` and `--wide --java` then answer every case
exactly as the C does.
