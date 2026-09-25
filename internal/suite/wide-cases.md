# The wide suite's keystroke cases

`whim test --wide` (`make whim-test-wide`) runs these beside the quick suite's
`cases.md`. They are the 102 screen cases of the suite the phases were verified
with (`448e9a8`, `internal/harness/zcaselist.go`), converted by a program: each
case types its OWN text under `paste`, and none opens a file, since the core has
had no way to name one since phase 91.

One line per case: the name, the startup arguments separated by `|`, and the
keys, all tab-separated. The escapes are `cases.md`'s: `\e \r \n \t \\ \xHH`
(a `|` in an argument is `\x7c`).

```
startup		\e:q!\r
ruler_move	+set paste	ione two three\e:set nopaste\r0wwj\e:q!\r
showmode_ins	+set paste	ix\e:set nopaste\rA\e:q!\r
term_report		:set term? t_Co?\r\e:q!\r
incr_hex	+set nrformats=hex|+set paste	i0x0f\e:set nopaste\r$\x01\e:q!\r
incr_bin	+set nrformats=bin|+set paste	i0b0101\e:set nopaste\r$\x01\e:q!\r
incr_oct	+set nrformats=octal|+set paste	i017\e:set nopaste\r$\x01\e:q!\r
incr_dec	+set nrformats=|+set paste	i42\e:set nopaste\r$\x01\e:q!\r
incr_alpha	+set nrformats=alpha|+set paste	iabc\e:set nopaste\r0\x01\e:q!\r
incr_unsigned	+set nrformats=unsigned|+set paste	i-7\e:set nopaste\r$\x01\e:q!\r
decr_hex	+set nrformats=hex|+set paste	i0x10\e:set nopaste\r$\x18\e:q!\r
decr_dec	+set nrformats=|+set paste	i42\e:set nopaste\r$\x18\e:q!\r
incr_count	+set nrformats=|+set paste	i5\e:set nopaste\r10\x01\e:q!\r
incr_midword	+set nrformats=|+set paste	iab12cd34\e:set nopaste\r0\x01$\x01\e:q!\r
autoindent	+set ai|+set paste	i    a\e:set nopaste\rA\rb\e\e:q!\r
ai_empty	+set ai|+set paste	i    a\e:set nopaste\rA\r\eA\rx\e\e:q!\r
format_gq	+set tw=20|+set paste	ione two three four five six seven eight nine ten eleven\e:set nopaste\rgqq\e:q!\r
format_comment	+set tw=20 fo=tcq comments=://|+set paste	i// aaa bbb ccc ddd eee fff ggg hhh\e:set nopaste\rgqq\e:q!\r
open_comment	+set fo=tcqr comments=://|+set paste	i// hello\e:set nopaste\rA\rx\e\e:q!\r
smartindent	+set si sw=4|+set paste	iif (x) {\e:set nopaste\rA\ry;\e\e:q!\r
shift_right	+set sw=4|+set paste	ia\rb\e:set nopaste\rggVG>\e:q!\r
indent_op	+set sw=2|+set paste	ia\rb\rc\e:set nopaste\rgg3>>\e:q!\r
retab_gone	+set ts=8 sw=4 et|+set paste	i\x16\ta\e:set nopaste\r:retab\r\e:q!\r
ins_multi	+set paste	ix\e:set nopaste\rAhello world\e\e:q!\r
ins_ctrl_v	+set paste	ix\e:set nopaste\rA\x16065\e\e:q!\r
ins_bs	+set paste	iabc\e:set nopaste\rA\x08\x08Z\e\e:q!\r
ins_ctrl_w	+set paste	ix\e:set nopaste\rAfoo bar\x17baz\e\e:q!\r
ins_tab_et	+set et sw=4 ts=8|+set paste	ix\e:set nopaste\rI\tA\e\e:q!\r
replace_mode	+set paste	iabcdef\e:set nopaste\r0Rxyz\e\e:q!\r
ins_arrows	+set paste	iabcdef\e:set nopaste\r0i\e[C\e[CZ\e\e:q!\r
mb_motion	+set paste	icaf\xc3\xa9 na\xc3\xafve \xe6\x97\xa5\xe6\x9c\xac\xe8\xaa\x9e\e:set nopaste\r0wdw\e:q!\r
mb_dollar_x	+set paste	i\xc3\xa0\xc3\xa9\xc3\xae\xc3\xb4\xc3\xbb\e:set nopaste\r$x\e:q!\r
mb_upper	+set paste	i\xc3\xa0\xc3\xa9\xc3\xae \xc7\x86 \xc3\x9f\e:set nopaste\rgUU\e:q!\r
mb_tilde	+set paste	ia\xc3\xa0B\xc3\x89\e:set nopaste\r0~~~~\e:q!\r
mb_incr	+set nrformats=hex|+set paste	i\xe6\x97\xa50x0f\xe6\x9c\xac\e:set nopaste\r0\x01\e:q!\r
mb_ga	+set paste	i\xe6\x97\xa5\xe6\x9c\xac\e:set nopaste\r0ga\e:q!\r
subst_magic	+set paste	ifoo123 bar456\e:set nopaste\r:%s/\\v(\\a+)(\\d+)/\\2-\\1/g\r\e:q!\r
subst_amp	+set paste	iaaa\e:set nopaste\r:%s/a/[&]/g\r\e:q!\r
subst_case	+set paste	ihello world\e:set nopaste\r:%s/\\w\\+/\\u&/g\r\e:q!\r
subst_nl	+set paste	ia b\e:set nopaste\r:%s/ /\\r/\r\e:q!\r
subst_count	+set paste	ix\rx\rx\e:set nopaste\r:%s/x/y/\r:1\r\e:q!\r
global_cmd	+set paste	ia1\rb2\ra3\e:set nopaste\r:g/a/s/$/!/\r\e:q!\r
vglobal	+set paste	ia1\rb2\ra3\e:set nopaste\r:v/a/d\r\e:q!\r
search_off	+set paste	iaaa\rbbb\rccc\e:set nopaste\rgg/bbb\rdd\e:q!\r
search_wrap	+set paste	iaaa\rbbb\e:set nopaste\rgg/aaa\rn\e:q!\r
dw_de_db	+set paste	ione two three four\e:set nopaste\r0dwdee\e:q!\r
ci_quote	+set paste	isay "hello" now\e:set nopaste\r0f"ci"bye\e\e:q!\r
da_paren	+set paste	if(a, b) end\e:set nopaste\r0f,da(\e:q!\r
yank_put	+set paste	ione\rtwo\e:set nopaste\rggyyGp\e:q!\r
visual_block	+set paste	iabcd\refgh\rijkl\e:set nopaste\rgg0\x16jjlx\e:q!\r
join_lines	+set paste	ia\rb\rc\e:set nopaste\rggJJ\e:q!\r
join_gJ	+set paste	ia\r  b\e:set nopaste\rgggJ\e:q!\r
dot_repeat	+set paste	ia b c d\e:set nopaste\r0dw..\e:q!\r
macro_q	+set nrformats=|+set paste	i1\r1\r1\e:set nopaste\rggqaA!\ejq2@a\e:q!\r
percent_match	+set paste	if(a, b) end\e:set nopaste\r0f(%\e:q!\r
undo_redo	+set paste	ia\rb\rc\e:set nopaste\rggddddu\x12u\e:q!\r
undo_block	+set paste	iabc\e:set nopaste\r0xxxuu\e:q!\r
undo_after_ins	+set paste	ix\e:set nopaste\rAabc\eu\e:q!\r
undo_U	+set paste	iabc\e:set nopaste\r0xU\e:q!\r
registers	+set paste	ia\rb\e:set nopaste\rgg"ryyG"rp\e:q!\r
reg_list	+set paste	ia\e:set nopaste\ryy:registers\r\e:q!\r
marks	+set paste	ia\rb\rc\e:set nopaste\rggmaGd'a\e:q!\r
mark_list	+set paste	ia\rb\e:set nopaste\rggma:marks\r\e:q!\r
move_copy	+set paste	i1\r2\r3\e:set nopaste\r:2m0\r:1t$\r\e:q!\r
normal_range	+set paste	ia\rb\rc\e:set nopaste\r:%normal A!\r\e:q!\r
sort_gone	+set paste	ib\ra\e:set nopaste\r:sort u\r\e:q!\r
filter_gone	+set paste	ic\ra\e:set nopaste\r:%!sort\r\e:q!\r
read_cmd_gone	+set paste	ix\e:set nopaste\r:r !echo piped\r\e:q!\r
put_expr_gone	+set paste	ix\e:set nopaste\r:put ='added'\r\e:q!\r
ff_gone	+set paste	ia\e:set nopaste\r:set ff=dos\r\e:q!\r
bomb_gone	+set paste	ia\e:set nopaste\r:set bomb\r\e:q!\r
hlsearch_opt	+set hlsearch|+set paste	ialpha\rbeta\rgamma\e:set nopaste\rgg/beta\r\e:q!\r
incsearch_opt	+set incsearch|+set paste	ialpha\rbeta\rgamma\e:set nopaste\rgg/gam\e:q!\r
match_cmd	+set paste	ialpha\rbeta\e:set nopaste\r:match Search /beta/\r\e:q!\r
nohlsearch	+set hls|+set paste	ialpha\rbeta\e:set nopaste\rgg/beta\r:nohlsearch\r\e:q!\r
search_count	+set paste	iaa\raa\raa\e:set nopaste\rgg/aa\rn\e:q!\r
nav_arrows	+set paste	il1\rl2\rl3\e:set nopaste\rgg\e[B\e[B\e[Cx\e:q!\r
nav_home_end	+set paste	iabcdef\e:set nopaste\r0\e[Fx\e[Hx\e:q!\r
scroll_ctrl_f		iline1\rline2\rline3\rline4\rline5\rline6\rline7\rline8\rline9\rline10\rline11\rline12\rline13\rline14\rline15\rline16\rline17\rline18\rline19\rline20\rline21\rline22\rline23\rline24\rline25\rline26\rline27\rline28\rline29\rline30\rline31\rline32\rline33\rline34\rline35\rline36\rline37\rline38\rline39\rline40\egg\x06\e:q!\r
scroll_zz		iline1\rline2\rline3\rline4\rline5\rline6\rline7\rline8\rline9\rline10\rline11\rline12\rline13\rline14\rline15\rline16\rline17\rline18\rline19\rline20\rline21\rline22\rline23\rline24\rline25\rline26\rline27\rline28\rline29\rline30\rline31\rline32\rline33\rline34\rline35\rline36\rline37\rline38\rline39\rline40\e20Gzz\e:q!\r
ctrl_g	+set paste	ia\rb\e:set nopaste\r\x07\e:q!\r
ctrl_g_count	+set paste	ia\rb\e:set nopaste\rg\x07\e:q!\r
visual_show	+set paste	iabcdef\e:set nopaste\r0vlll\e:q!\r
set_listing		:set nu list\r\e:q!\r
hit_enter		:history\r\e:q!\r
unknown_cmd		:nosuchcmd\r\e:q!\r
quit_modified	+set paste	ix\e:set nopaste\r:q\r\e:q!\r
zz_key	+set paste	ix\e:set nopaste\rZZ\e:q!\r
map_tab_percent	+set paste	if(a, b) end\e:set nopaste\r0f(\t\e:q!\r
map_e_acute_undo	+set paste	iabc\e:set nopaste\r0x\xc3\xa9\e:q!\r
map_in_paste	+set paste	ix\xc2\xa7y\e:set nopaste\r\e:q!\r
map_list		:map\r\e:q!\r
ctrl_c_clean		\x03\e:q!\r
ctrl_c_changed	+set paste	ix\e:set nopaste\r\x03\e:q!\r
key_Q	+set paste	ialpha\rbeta\e:set nopaste\rQ\e:q!\r
key_gQ	+set paste	ialpha\rbeta\e:set nopaste\rgQ\e:q!\r
cmd_write	+set paste	ix\e:set nopaste\r:write\r\e:q!\r
cmd_read	+set paste	ix\e:set nopaste\r:read\r\e:q!\r
cmd_edit	+set paste	ix\e:set nopaste\r:edit\r\e:q!\r
cmd_file	+set paste	ix\e:set nopaste\r:file\r\e:q!\r
key_gf	+set paste	inosuchfile\e:set nopaste\r0gf\e:q!\r
reg_percent	+set paste	ix\e:set nopaste\rA \x12%\e\e:q!\r
```
