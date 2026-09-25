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
  it reads (`an`, the pointer classes, `gen`'s collection). A mode of `Run`:
  `whim skel <editor.c> <dir> -java <dir>`.
- `jeditor/rt/`: the runtime, by hand -- `BytePtr` and its kin, the unsigned
  helpers, `memmove` and the rest -- as `editor/crt.go` is the Go's.
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
   left.
2. **The rest of the constructs** the coverage report names, by frequency:
   function pointers, unions, the labeled-block `goto`, the growarray, the
   variadic call, the 64 KB split. The measure: every function of `editor.c`
   written, and `javac` compiling `Editor.java`.
3. **The host and the launcher**, and `whim test` running the Java editor: the
   45 cases, then `--wide`, required to answer as the C does.
4. **Kept current:** `make whim-build` writes `Editor.java` as it writes
   `editor.go`, and a check refuses a stale one.
