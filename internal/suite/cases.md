# The minimal behaviour corpus

Each line of the fenced block is one editing session: a name, a tab, and the
keystrokes, fed to the editor on stdin (24x80, no terminal, no files).  Escapes:
`\e` Escape, `\r` Enter, `\t` Tab, `\\` a backslash, `\xHH` any byte.  Every
session starts from an empty buffer, so it types its own text first, and ends
with `:q!`.  The cases are vim's own editing surface as upstream has it --
modes, motions, operators, text objects, registers, Ex commands -- restricted to
what an editor with no files can show; a few ask for what whim removed, and the
message it gives is part of the record too.

The oracle is the committed product: `go tool whim test` builds the editor from
`src/whim-vim.c` at a git revision (HEAD by default) and from the working tree,
runs every session on both, and requires the same bytes out and the same exit
status.  Add a case by adding a line; nothing else is stored.

```
insert	ione two three\efour\r:q!\r
append	ione\eatwo\eAthree\e:q!\r
open	ione\eotwo\eOzero\e:q!\r
motions	ione two three four\e0wwD:q!\r
word_end	ione two three\e0eex:q!\r
find	ia,b,c,d,e\e0f,;;x:q!\r
till	ia(b)c(d)\e0t)x:q!\r
match	iif (a (b) c) end\e0f(%x:q!\r
gg_G	i1\r2\r3\r4\r5\eggdGu:q!\r
dd_count	i1\r2\r3\r4\r5\egg2ddp:q!\r
yank_put	ione\eyyp3p:q!\r
change_word	ione two three\e0wcwTWO\e:q!\r
dollar	ione two three\e0d$:q!\r
text_obj_iw	ione two three\e0wdiw:q!\r
text_obj_quote	ia "quoted text" b\e0f"lci"new\e:q!\r
text_obj_paren	if(a, (b, c), d)\e0f(lda(:q!\r
visual_char	ione two three\e0wvey$p:q!\r
visual_line	i1\r2\r3\eggVjd:q!\r
visual_block	iabcd\rabcd\rabcd\egg0l\x16jjld:q!\r
undo_redo	ione\eotwo\eothree\euu\x12:q!\r
repeat	ione one one one\e0cwtwo\eww.w.:q!\r
registers	ione\e"ayyotwo\e"byy"ap"bp:q!\r
macro	i1\r2\r3\egg0qaA!\ejq2@a:q!\r
search	ifoo bar foo baz foo\egg0/foo\rnx:q!\r
search_back	ifoo bar foo baz foo\e$?foo\rx:q!\r
star	ione two one three one\egg0*x:q!\r
substitute	ia-b-c\ra-b\e:%s/-/+/g\r:q!\r
substitute_count	ia a a\ra a\e:%s/a/b/\r:q!\r
global	ikeep\rdrop\rkeep\rdrop\e:g/drop/d\r:q!\r
vglobal	ikeep\rdrop\rkeep\e:v/keep/d\r:q!\r
move_copy	i1\r2\r3\e:1m$\r:1t0\r:q!\r
normal	ione\rtwo\rthree\e:%norm Ax\r:q!\r
join	ione\rtwo\rthree\eggJgJ:q!\r
case	iHello World\e0~w~:s/.*/\\U&/\r:q!\r
case_ops	ihello world\e0gUiwwg~iw:q!\r
indent	ione\rtwo\e:set sw=4 et\rgg>>j>>k<<:q!\r
increment	ix = 7 and 0x0f\e0\x01w5\x01w\x18:q!\r
replace	iabcdef\e0lRXYZ\e0rQ:q!\r
counts	i1\e5.3x:q!\r
marks	i1\r2\r3\r4\eggmaGd'a:q!\r
ex_range	i1\r2\r3\r4\r5\e:2,4d\r:q!\r
ex_errors	:nosuchcommand\r:s/x/y/\r:q!\r
removed	:let x = 1\r:echo 1 + 1\r:q!\r
version	:version\r\r:q!\r
```
