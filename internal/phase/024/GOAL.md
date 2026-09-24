# Phase 24 — there is no mouse

A terminal mouse is a protocol, not a device: the terminal is asked to report
clicks, it sends escape sequences, the editor decodes them into key codes, and
the normal, insert and command-line loops dispatch those like any other key.
All four layers are here, and an editor driven from a keyboard needs none of
them.

**The island is bounded**, which is what makes this a cut rather than a rewrite.
Thirty-five functions mention the mouse and all but two are reached only from
each other, so `funcreach.py` deletes the interior once the roots are gone. The
tool removes only the roots:

| layer | what goes |
| --- | --- |
| the tables | 22 rows of `nv_cmds[]` point at `nv_error`, and the 14 `<LeftMouse>`/`<ScrollWheelUp>`/`<MouseMove>` rows of the key-name table go, so `:map <LeftMouse>` no longer names anything |
| the dispatch | `edit()`'s insert-mode case run, `getcmdline_int()`'s six case runs, `]<LeftMouse>` in `nv_brackets()` and `g<LeftMouse>` in `nv_g_cmd()`, and the click that dismissed a `Press ENTER` prompt |
| the decoder | `check_termcode_mouse()`'s call, and 41 lines in `set_termname()` that read the terminal's 1006 capability, set `'ttymouse'` from it and install the termcodes |
| the switch | `setmouse()`'s **31** calls, every one a bare statement, and `mch_setmouse()`'s |
| the questions | `mouse_has()` and `mouse_has_any()`, whose three callers outside the island each become the answer they now always get |

## The rows of nv_cmds[] are pointed away, never deleted

**This phase first deleted the 22 rows, and the arrow keys stopped working in
normal mode for twelve phases.** Normal mode finds a key's handler through
`nv_cmd_idx[]`, a sorted index into `nv_cmds[]` that upstream generates and this
tree writes into the C as a constant. Deleting rows left the index 22 entries
longer than the table, still compiling, and every key found past the first hole
resolved to another key's row. Nothing noticed because every harness that typed
an arrow typed it in insert mode, which decodes the arrows in a `switch`.

So the rows stay and answer `nv_error` — rule 3, applied to the normal-mode
table, which is what Phase 30 already did for `K` and CTRL-]. Two checks now
guard it: `tools/nvidxcheck.py`, run by `phasecheck.sh` in every phase, requires
the index to be a permutation of the table's rows, and `tools/arrowcheck.py` —
retired after Phase 82, see there — pressed all four arrows in normal mode, in the `ESC O` form a terminal sends once
vim has switched its keypad to application mode.

## Two names that are not about the mouse

Both checked rather than assumed. `get_mouse_class()`, `find_start_of_word()`
and `find_end_of_word()` classify characters for **double-click word
selection** and are reached only from `do_mouse()`, so they go with it — the
name says mouse and the work is text, which is exactly the shape that survives a
careless sweep.

`WaitForCharOrMouse()` has no mouse in it at all: the name is left over from the
GUI build, where it also polled for motion events. Here it is `input_available()`
and `RealWaitForChar()`. It is folded into `WaitForChar()`, its only caller,
rather than left telling a lie.

## The enumerators stay

`KE_LEFTMOUSE`, `KS_MOUSE` and the rest are constants that cost nothing, and
**deleting an enumerator renumbers every one after it** — several enums in this
file index a parallel table. That is a different kind of change and does not
belong in a phase about capability.

## Four circles, and a tool bug

This phase found more than it removed, and all of it is the same shape: **an
option row is a root for reachability**, so a row keeps its own readers alive
and `--strict` then refuses to drop the row because those readers exist.

1. `did_set_string_option()` asks `if (varp == &p_mouse)`, and
   `check_mouse_termcode()` survives because `did_set_ttymouse()` names it.
2. `'mouse'`, `'mousemodel'` and `'ttymouse'` each name a `did_set_` and an
   `expand_set_` handler in the row itself, and `did_set_mousemodel()` reads
   `p_mousem`. The rows are pointed at NULL first; they go a moment later.
3. `:behave mswin` sets `'mousemodel'` **by name**, and the terminal's
   mouse-protocol reply sets `'ttymouse'` by name — the lookup that returns −1
   for a row that is not there and is not checked. `:behave` is about selection
   and keeps working; it just stops setting an option that has gone.
4. `didset_string_options()` dereferences every string option's global once at
   startup, which is the trap Phase 18 records. A row can be inert to every
   other reader and still be read there.

**And one real tool bug, which cost the most and was worth the most.** The first
run of this phase deleted **654 functions** and left a file that would not
compile. The cause:

```c
static struct mousetable
{
    int     pseudo_code;
    ...
} mouse_table[] =
{
    ...
};
```

gcc reports the unused variable at `} mouse_table[] =`, and `deadsweep.py` ran
forward from there — taking the initialiser and leaving the struct body open, so
the next declaration landed inside it and gcc said *"expected
specifier-qualifier-list before `static`"* a hundred lines later. Everything
after that was garbage compiling on garbage.

It is the same class of mistake as keying on a warning's sentence instead of its
option: **the extent of a thing is not "the line it was reported on"**.
`declaration_extent()` now walks *backwards* too when the declarator starts with
`}`, over the type body and its head. There are five constructs of that shape in
the file — `modmasktable`, `key_name_entry`, `mousetable`, `signalinfo` and
`termcode` — and this is the first phase that ever made one unused.

`dropoptions.py` was bounded at the same time: its guards looked 400 characters
ahead from the row's start, and `'mousefocus'` and `'mousehide'` are
`(char_u *)NULL` — GUI options with no global at all — so the search ran past
the end of the row and found the *next* option's variable. Every guard is now
bounded by the row's own braces.

## The delta

**None the harness records.** No Ex command is a mouse command, no behaviour
case clicks, and the pty harness types keys. The phase adds a check of its own:
`:set mouse=a` must be refused.

It took three tries to write that check, and each failure is one CLAUDE.md
already warns about. Reading the error message finds nothing, because silent Ex
mode prints nothing. Testing whether a later `:w` wrote the file finds nothing
either — **a failing `-c` does not abandon the ones after it**, so
`set nosuchopt` followed by `w` still writes. The exit status is the answer: 0
for an option that exists, 1 for one that does not. It is paired with
`:set ignorecase` as a control, so the check fails if the binary starts exiting
1 whatever it is asked.
