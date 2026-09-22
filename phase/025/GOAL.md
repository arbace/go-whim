# Phase 25 — a write is a write, and nobody owns it

## a write is a write

Writing a file in vim is not one operation. Before the new contents go anywhere
the old file may be renamed or copied aside, its permissions, owner, group, ACL
and timestamps carried over, the write attempted, and the whole thing rolled
back if it fails — and afterwards the copy is kept, or deleted, or renamed again
for `'patchmode'`. That is **437 lines of `buf_write()`**, and what `'backup'`,
`'writebackup'`, `'backupcopy'`, `'backupdir'`, `'backupext'`, `'backupskip'`
and `'patchmode'` are between them.

An embedded editor writes the file it was asked to write.

**`dobackup` is the hinge.** It is `(p_wb || p_bk || *p_pm != NUL)`, so with the
options gone it is FALSE, `backup` stays NULL and `backup_copy` stays FALSE —
and the tests spread through the rest of the function each collapse to the
branch they were already taking under `:set nobackup nowritebackup`, a
configuration vim has always supported. One of them is an `if`/`else if` whose
*else* is the live arm, so the pair collapses to that rather than going;
`buf_setino()` still has to happen.

Three things fall out that are worth naming separately:

  * **`vim_rename()` has five callers and all five are in here** — make the
    backup, put it back when the write fails, put it back when it is abandoned,
    and move it aside for `'patchmode'`. So `vim_copyfile()` goes with it, and
    that is `readlink`, `symlink` and `rename`.
  * `set_file_time()` carried the old file's timestamps onto the backup. One
    caller, and that is `utime`.
  * `mch_get_acl()`, `mch_set_acl()` and `mch_free_acl()` are **already stubs** —
    this build has no ACL support, so one returns NULL and the others do nothing
    with it. They went unnoticed for thirty phases because a stub compiles. The
    `vim_acl_T` that threaded through `buf_write()` to reach them goes too, and
    its three forward declarations go *here* rather than in the sweep: the sweep
    has to compile the file first, and a prototype naming a type this removes is
    an error, not a warning.

`fchown` and `umask` were not on the list and went anyway — every call to both
was inside the backup block.

### The same circle, twice more

`'backupcopy'` names `did_set_backupcopy` and `expand_set_backupcopy` in its own
row, and `'backupext'` and `'patchmode'` share
`did_set_backupext_or_patchmode`; a row is a root, so the handlers survive the
sweep, read `p_bkc` and `p_bex`, and `--strict` then refuses to drop the row
that is the only thing keeping them alive. Phase 24 met this three times. The
rows are pointed at NULL first.

`didset_string_options()` reads `p_bkc` at startup — the trap Phase 18 records,
met again — and `set_init_default_backupskip()` looks its row up **by name**,
the lookup that returns −1 and is not checked.

### Where the symbol count moves

**98 → 92**: `fchown`, `readlink`, `rename`, `symlink`, `umask`, `utime`.

### The delta

**None the harness records.** `:w` writes; it just stops leaving a `~` file
beside what it wrote, which no harness asked for. The phase checks that
directly — overwrite a file and the directory must hold exactly what it held
before, with the new contents in it.

## nobody owns a file

An embedded editor runs where there are no users to tell apart, so asking who
you are is asking a question with no answer. Four places were still asking.

  * `:w!` on a read-only file makes it writable first, but only **if you own
    it**: `st_old.st_uid == getuid()`. The ownership test goes and the `chmod`
    stays. Nothing widens in practice — where the test used to say no, the
    `chmod` now says no instead, and the same error comes back by a different
    route.
  * When a write fails and `!` makes it retry, the mode carried onto the new
    file is masked to `0777`, dropping setuid, setgid and sticky — but only if
    you are not the owner. The test goes and **the masking stays**, which is the
    safe direction: a file this editor writes never carries a setuid bit.
  * `'modeline'` is forced off when `getuid() == ROOT_UID`, a protection against
    a modeline running as root. There is no root here and no `+eval` for a
    modeline to reach.
  * `get_user_name()` was stubbed to `return FAIL;` in Phase 20, when the
    password database went, and its two callers were left writing the answer
    into the swap file's block zero. The second one's `else` — the arm that
    spliced a user name into the recorded file name — has therefore been dead
    since Phase 20 and goes now, along with the `b0_uname` field itself. **A
    struct field is not a variable**: no warning names one that nothing reads,
    and the sweep cannot see it, so it has to be named here.

### Permissions are not ownership

`chmod` and `fchmod` stay, through `mch_setperm()` and `mch_fsetperm()`. A file
still has a mode, `:w!` still has to clear the read-only bit to write, and the
mode of the file that was there is still put back on the file that replaces it.
Removing those would take `:w!` on a read-only file with them, which is a
capability and not a concept — so the phase asserts both halves: `getuid` and
`getgid` gone from `nm -u`, `mch_setperm`/`mch_fsetperm`/`mch_getperm` still
called, and `:w!` over a `chmod 444` file still writes it. No harness writes to
a read-only file, which is why that check lives here.

### Where the symbol count moves

**92 → 90**: `getuid`, `getgid`.

### The delta

**None.**
