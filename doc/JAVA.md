# The editor in Java: design and milestones

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
- `jeditor/rt/`: the runtime, by hand, package `whim.rt` -- `BytePtr`,
  `ShortPtr`, `IntPtr`, `LongPtr`, `BoolPtr`, `Ptr<T>` and `Rt` (`memmove`,
  `memset`, `memcmp`, a char array's initial value) -- as `editor/crt.go` is
  the Go's, and `SelfTest.java`, its own test with no framework.
- `jeditor/`: the generated `Editor.java`, the host interface and the terminal
  host (the Foreign Function & Memory API, JDK 22 and later, calls `ioctl`,
  `select` and `read` as the Go's `syscall` does), and the launcher.

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
   `editor.c` written, and `javac` compiling `Editor.java`.
3. **The host and the launcher**, and `whim test` running the Java editor: the
   45 cases, then `--wide`, required to answer as the C does.
4. **Kept current:** `make whim-build` writes `Editor.java` as it writes
   `editor.go`, and a check refuses a stale one.

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
