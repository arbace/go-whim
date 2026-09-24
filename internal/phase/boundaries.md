# Every boundary, measured

The text after every phase, from one `go tool whim build --keep D` of the
committed `slim-vim.c` at `3873ec0` -- the build's product was `whim-vim.c` byte
for byte, and so was `q163`, and the product has not moved since -- counted by
`go tool whim measure D`.

A row is the boundary AFTER that phase: after its sweep where the schedule puts
one (`internal/build/plan.go`), so a phase that shares a sweep with the ones after
it is measured before that sweep, and there the text need not parse -- those rows
say `no` and leave the counts empty. The columns are `internal/reach`'s entity
counts, so a count means the same thing on every row: `F` function definitions
(prototypes merged in), `O` file-scope objects, `P` prototypes of functions
never defined, `T` typedefs, `S` tagged struct and union definitions, `E`
tagged enum definitions, `M` members, `N` enumerators. `binary` is bytes and
`nm-u` the undefined symbols of the object, both built with THE ONE COMPILE LINE,
`gcc -O0 -fno-stack-protector -static -no-pie -s` (`build.FlagsFor`), with
`SOURCE_DATE_EPOCH=0` -- the same line for every row, so q82, q83 and q84, whose
text is one text, are one binary.

The phases' `GOAL.md` `Measured` tables that predate canonical seeding
(`d8365fb`) are records of the pipeline as it then ran, and keep their numbers;
this file is what the pipeline measures now. Re-measure it with the two
commands above -- never edit a number by hand.

```
phase  lines    F      O     P    T     S    E    M      N      binary    nm-u  parses
000    174048   3352   1579  7    251   112  5    1625   2898   2003192   145   yes
001    174048   3352   1579  7    251   112  5    1625   2898   2003192   145   yes
002    174038   3352   1579  7    251   112  5    1625   2898   2002616   145   yes
003    173834   3350   1579  7    251   112  5    1625   2898   1998520   145   yes
004    173833   3350   1579  7    251   112  5    1625   2898   1998520   145   yes
005    159854   3270   1579  20   251   112  5    1625   2898   1826200   145   yes
006    159739   3270   1579  20   251   112  5    1625   2898   1821976   145   yes
007    159641   3270   1579  20   251   112  5    1625   2898   1821976   145   yes
008    159287   3270   1579  20   251   112  5    1625   2898   1817880   145   yes
009    159258   3270   1579  20   251   112  5    1625   2898   1817496   144   yes
010    159210   3270   1579  20   251   112  5    1625   2898   1817080   144   yes
011    158982   3270   1579  20   251   112  5    1625   2898   1812824   143   yes
012    148032   3102   1458  7    224   104  5    1477   2515   1564984   117   yes
013    147982   3102   1458  7    224   104  5    1477   2515   1564984   117   yes
014    148001   3102   1458  7    224   104  5    1477   2515   1564984   117   yes
015    147982   3102   1458  7    224   104  5    1477   2515   1564856   117   yes
016    146066   3078   1435  7    224   104  5    1471   2508   1543160   117   yes
017    145915   3078   1435  7    224   104  5    1467   2508   1538904   117   yes
018    145780   3078   1435  7    224   104  5    1465   2508   1538712   117   yes
019    145751   3078   1435  7    224   104  5    1465   2508   1538712   117   yes
020    145298   3076   1435  7    224   104  5    1465   2508   1534520   116   yes
021    142495   3017   1395  7    216   99   5    1358   2480   1492104   102   yes
022    142382                                                              fails     -     no -- .tmp/bnd/q022.c:142179:1: conflicting types for 'edit_buffers', previous declaration at .tmp/bnd/q022.c:141334:13:, function(pointer to mparm_T) and function(pointer to mparm_T, pointer to char_u) (check.go:269:checkScope:)
023    142237                                                              fails     -     no -- .tmp/bnd/q023.c:142034:1: conflicting types for 'edit_buffers', previous declaration at .tmp/bnd/q023.c:141189:13:, function(pointer to mparm_T) and function(pointer to mparm_T, pointer to char_u) (check.go:269:checkScope:)
024    139447   2966   1374  8    215   97   5    1349   2442   fails     98    yes
025    138679                                                              fails     -     no -- .tmp/bnd/q025.c:73720:22: undefined: B0_UNAME_SIZE (check.go:5157:check:)
026    138617                                                              fails     -     no -- .tmp/bnd/q026.c:73719:22: undefined: B0_UNAME_SIZE (check.go:5157:check:)
027    138612                                                              fails     -     no -- .tmp/bnd/q027.c:73719:22: undefined: B0_UNAME_SIZE (check.go:5157:check:)
028    133182   2886   1346  7    213   97   5    1303   2414   fails     88    yes
029    133159                                                              fails     -     no -- .tmp/bnd/q029.c:123621:38: type struct file_buffer {b_ml memline_T; b_next pointer to struct file_buffer; b_prev pointer to struct file_buffer; b_nwindows int; b_flags int; b_locked int; b_locked_split int; b_ffname pointer to char_u; b_sfname pointer to char_u; b_fname pointer to char_u; b_dev_valid _Bool; b_dev dev_t; b_ino ino_t; b_fnum int; b_key array of 9 char_u; b_changed int; b_ct_di dictitem16_T; b_last_changedtick varnumber_T; b_last_changedtick_pum varnumber_T; b_last_changedtick_i varnumber_T; b_saving _Bool; b_mod_set _Bool; b_mod_top linenr_T; b_mod_bot linenr_T; b_mod_xlines long; b_wininfo pointer to struct wininfo_S; b_mtime long; b_mtime_ns long; b_mtime_read long; b_mtime_read_ns long; b_orig_size off_T; b_orig_mode int; b_namedm array of 26 pos_T; b_visual visualinfo_T; b_last_cursor pos_T; b_last_insert pos_T; b_last_change pos_T; b_changelist array of 100 pos_T; b_changelistlen int; b_new_change _Bool; b_chartab array of 32 char_u; b_maphash array of 256 pointer to struct mapblock; b_first_abbr pointer to struct mapblock; b_op_start pos_T; b_op_start_orig pos_T; b_op_end pos_T; b_modified_was_set _Bool; b_did_filetype _Bool; b_keep_filetype _Bool; b_au_did_filetype _Bool; b_u_oldhead pointer to struct u_header; b_u_newhead pointer to struct u_header; b_u_curhead pointer to struct u_header; b_u_numhead int; b_u_synced _Bool; b_u_seq_last long; b_u_save_nr_last long; b_u_seq_cur long; b_u_time_cur time_T; b_u_save_nr_cur long; b_u_line_ptr undoline_T; b_u_line_lnum linenr_T; b_u_line_colnr colnr_T; b_scanned _Bool; b_p_iminsert long; b_p_imsearch long; b_p_initialized _Bool; b_p_ac int; b_p_ai int; b_p_ai_nopaste int; b_bkc_flags unsigned; b_p_ci int; b_p_bin int; b_p_bh pointer to char_u; b_p_bt pointer to char_u; b_p_bl int; b_p_com pointer to char_u; b_p_cms pointer to char_u; b_p_cot pointer to char_u; b_cot_flags unsigned; b_p_cpt pointer to char_u; b_p_eof int; b_p_eol int; b_p_fixeol int; b_p_et int; b_p_et_nobin int; b_p_et_nopaste int; b_p_ff pointer to char_u; b_p_ft pointer to char_u; b_p_fo pointer to char_u; b_p_flp pointer to char_u; b_p_inf int; b_p_isk pointer to char_u; b_p_fp pointer to char_u; b_p_fs int; b_p_kp pointer to char_u; b_p_lisp int; b_p_lop pointer to char_u; b_p_menc pointer to char_u; b_p_mps pointer to char_u; b_p_ml int; b_p_ml_nobin int; b_p_ma int; b_p_nf pointer to char_u; b_p_pi int; b_p_qe pointer to char_u; b_p_ro int; b_p_sw long; b_p_sn int; b_p_si int; b_p_sts long; b_p_sts_nopaste long; b_p_ts long; b_p_tx int; b_p_tw long; b_p_tw_nobin long; b_p_tw_nopaste long; b_p_wm long; b_p_wm_nobin long; b_p_wm_nopaste long; b_p_ep pointer to char_u; b_tc_flags unsigned; b_p_dict pointer to char_u; b_p_tsr pointer to char_u; b_p_ul long; b_p_lw pointer to char_u; b_no_eol_lnum linenr_T; b_start_eof int; b_start_eol int; b_start_ffc int; b_bad_char int; b_may_swap _Bool; b_did_warn _Bool; b_help _Bool; b_shortname _Bool; b_mapped_ctrl_c int} has no member named b_ucmds (check.go:4913:check:)
030    133014                                                              fails     -     no -- .tmp/bnd/q030.c:123507:38: type struct file_buffer {b_ml memline_T; b_next pointer to struct file_buffer; b_prev pointer to struct file_buffer; b_nwindows int; b_flags int; b_locked int; b_locked_split int; b_ffname pointer to char_u; b_sfname pointer to char_u; b_fname pointer to char_u; b_dev_valid _Bool; b_dev dev_t; b_ino ino_t; b_fnum int; b_key array of 9 char_u; b_changed int; b_ct_di dictitem16_T; b_last_changedtick varnumber_T; b_last_changedtick_pum varnumber_T; b_last_changedtick_i varnumber_T; b_saving _Bool; b_mod_set _Bool; b_mod_top linenr_T; b_mod_bot linenr_T; b_mod_xlines long; b_wininfo pointer to struct wininfo_S; b_mtime long; b_mtime_ns long; b_mtime_read long; b_mtime_read_ns long; b_orig_size off_T; b_orig_mode int; b_namedm array of 26 pos_T; b_visual visualinfo_T; b_last_cursor pos_T; b_last_insert pos_T; b_last_change pos_T; b_changelist array of 100 pos_T; b_changelistlen int; b_new_change _Bool; b_chartab array of 32 char_u; b_maphash array of 256 pointer to struct mapblock; b_first_abbr pointer to struct mapblock; b_op_start pos_T; b_op_start_orig pos_T; b_op_end pos_T; b_modified_was_set _Bool; b_did_filetype _Bool; b_keep_filetype _Bool; b_au_did_filetype _Bool; b_u_oldhead pointer to struct u_header; b_u_newhead pointer to struct u_header; b_u_curhead pointer to struct u_header; b_u_numhead int; b_u_synced _Bool; b_u_seq_last long; b_u_save_nr_last long; b_u_seq_cur long; b_u_time_cur time_T; b_u_save_nr_cur long; b_u_line_ptr undoline_T; b_u_line_lnum linenr_T; b_u_line_colnr colnr_T; b_scanned _Bool; b_p_iminsert long; b_p_imsearch long; b_p_initialized _Bool; b_p_ac int; b_p_ai int; b_p_ai_nopaste int; b_bkc_flags unsigned; b_p_ci int; b_p_bin int; b_p_bh pointer to char_u; b_p_bt pointer to char_u; b_p_bl int; b_p_com pointer to char_u; b_p_cms pointer to char_u; b_p_cot pointer to char_u; b_cot_flags unsigned; b_p_cpt pointer to char_u; b_p_eof int; b_p_eol int; b_p_fixeol int; b_p_et int; b_p_et_nobin int; b_p_et_nopaste int; b_p_ff pointer to char_u; b_p_ft pointer to char_u; b_p_fo pointer to char_u; b_p_flp pointer to char_u; b_p_inf int; b_p_isk pointer to char_u; b_p_fp pointer to char_u; b_p_fs int; b_p_kp pointer to char_u; b_p_lisp int; b_p_lop pointer to char_u; b_p_menc pointer to char_u; b_p_mps pointer to char_u; b_p_ml int; b_p_ml_nobin int; b_p_ma int; b_p_nf pointer to char_u; b_p_pi int; b_p_qe pointer to char_u; b_p_ro int; b_p_sw long; b_p_sn int; b_p_si int; b_p_sts long; b_p_sts_nopaste long; b_p_ts long; b_p_tx int; b_p_tw long; b_p_tw_nobin long; b_p_tw_nopaste long; b_p_wm long; b_p_wm_nobin long; b_p_wm_nopaste long; b_p_ep pointer to char_u; b_tc_flags unsigned; b_p_dict pointer to char_u; b_p_tsr pointer to char_u; b_p_ul long; b_p_lw pointer to char_u; b_no_eol_lnum linenr_T; b_start_eof int; b_start_eol int; b_start_ffc int; b_bad_char int; b_may_swap _Bool; b_did_warn _Bool; b_help _Bool; b_shortname _Bool; b_mapped_ctrl_c int} has no member named b_ucmds (check.go:4913:check:)
031    132700                                                              fails     -     no -- .tmp/bnd/q031.c:123193:38: type struct file_buffer {b_ml memline_T; b_next pointer to struct file_buffer; b_prev pointer to struct file_buffer; b_nwindows int; b_flags int; b_locked int; b_locked_split int; b_ffname pointer to char_u; b_sfname pointer to char_u; b_fname pointer to char_u; b_dev_valid _Bool; b_dev dev_t; b_ino ino_t; b_fnum int; b_key array of 9 char_u; b_changed int; b_ct_di dictitem16_T; b_last_changedtick varnumber_T; b_last_changedtick_pum varnumber_T; b_last_changedtick_i varnumber_T; b_saving _Bool; b_mod_set _Bool; b_mod_top linenr_T; b_mod_bot linenr_T; b_mod_xlines long; b_wininfo pointer to struct wininfo_S; b_mtime long; b_mtime_ns long; b_mtime_read long; b_mtime_read_ns long; b_orig_size off_T; b_orig_mode int; b_namedm array of 26 pos_T; b_visual visualinfo_T; b_last_cursor pos_T; b_last_insert pos_T; b_last_change pos_T; b_changelist array of 100 pos_T; b_changelistlen int; b_new_change _Bool; b_chartab array of 32 char_u; b_maphash array of 256 pointer to struct mapblock; b_first_abbr pointer to struct mapblock; b_op_start pos_T; b_op_start_orig pos_T; b_op_end pos_T; b_modified_was_set _Bool; b_did_filetype _Bool; b_keep_filetype _Bool; b_au_did_filetype _Bool; b_u_oldhead pointer to struct u_header; b_u_newhead pointer to struct u_header; b_u_curhead pointer to struct u_header; b_u_numhead int; b_u_synced _Bool; b_u_seq_last long; b_u_save_nr_last long; b_u_seq_cur long; b_u_time_cur time_T; b_u_save_nr_cur long; b_u_line_ptr undoline_T; b_u_line_lnum linenr_T; b_u_line_colnr colnr_T; b_scanned _Bool; b_p_iminsert long; b_p_imsearch long; b_p_initialized _Bool; b_p_ac int; b_p_ai int; b_p_ai_nopaste int; b_bkc_flags unsigned; b_p_ci int; b_p_bin int; b_p_bh pointer to char_u; b_p_bt pointer to char_u; b_p_bl int; b_p_com pointer to char_u; b_p_cms pointer to char_u; b_p_cot pointer to char_u; b_cot_flags unsigned; b_p_cpt pointer to char_u; b_p_eof int; b_p_eol int; b_p_fixeol int; b_p_et int; b_p_et_nobin int; b_p_et_nopaste int; b_p_ff pointer to char_u; b_p_ft pointer to char_u; b_p_fo pointer to char_u; b_p_flp pointer to char_u; b_p_inf int; b_p_isk pointer to char_u; b_p_fp pointer to char_u; b_p_fs int; b_p_kp pointer to char_u; b_p_lisp int; b_p_lop pointer to char_u; b_p_menc pointer to char_u; b_p_mps pointer to char_u; b_p_ml int; b_p_ml_nobin int; b_p_ma int; b_p_nf pointer to char_u; b_p_pi int; b_p_qe pointer to char_u; b_p_ro int; b_p_sw long; b_p_sn int; b_p_si int; b_p_sts long; b_p_sts_nopaste long; b_p_ts long; b_p_tx int; b_p_tw long; b_p_tw_nobin long; b_p_tw_nopaste long; b_p_wm long; b_p_wm_nobin long; b_p_wm_nopaste long; b_p_ep pointer to char_u; b_tc_flags unsigned; b_p_dict pointer to char_u; b_p_tsr pointer to char_u; b_p_ul long; b_p_lw pointer to char_u; b_no_eol_lnum linenr_T; b_start_eof int; b_start_eol int; b_start_ffc int; b_bad_char int; b_may_swap _Bool; b_did_warn _Bool; b_help _Bool; b_shortname _Bool; b_mapped_ctrl_c int} has no member named b_ucmds (check.go:4913:check:)
032    124809   2732   1278  7    210   96   5    1257   2355   fails     88    yes
033    124799   2732   1278  7    210   96   5    1257   2355   fails     88    yes
034    124775   2732   1278  7    210   96   5    1257   2355   fails     88    yes
035    124241                                                              fails     -     no -- .tmp/bnd/q035.c:119837:78: type winopt_T has no member named wo_eiw (check.go:4882:check:)
036    123905                                                              fails     -     no -- .tmp/bnd/q036.c:119512:78: type winopt_T has no member named wo_eiw (check.go:4882:check:)
037    123876                                                              fails     -     no -- .tmp/bnd/q037.c:119483:78: type winopt_T has no member named wo_eiw (check.go:4882:check:)
038    123842                                                              fails     -     no -- .tmp/bnd/q038.c:119449:78: type winopt_T has no member named wo_eiw (check.go:4882:check:)
039    123423                                                              fails     -     no -- .tmp/bnd/q039.c:119079:78: type winopt_T has no member named wo_eiw (check.go:4882:check:)
040    123284                                                              fails     -     no -- .tmp/bnd/q040.c:119048:78: type winopt_T has no member named wo_eiw (check.go:4882:check:)
041    113592   2437   1129  6    203   94   5    1199   2269   1192040   85    yes
042    113106   2424   1119  6    203   94   5    1197   2259   1187432   85    yes
043    113048   2424   1119  6    203   94   5    1197   2259   1183336   85    yes
044    113044   2424   1119  6    203   94   5    1197   2259   1183336   85    yes
045    113044   2424   1119  6    203   94   5    1197   2259   1183336   85    yes
046    113044   2424   1119  6    203   94   5    1197   2259   1183336   85    yes
047    113044   2424   1119  6    203   94   5    1197   2259   1183336   85    yes
048    113027   2424   1119  6    203   94   5    1197   2259   1183336   85    yes
049    111457   2397   1102  6    201   94   5    1184   2254   1162184   84    yes
050    110651   2379   1093  6    201   94   5    1169   2244   1153416   84    yes
051    110555   2379   1093  6    201   94   5    1169   2244   1153416   84    yes
052    108911   2379   1088  6    201   94   5    1169   2244   1137000   82    yes
053    106932   2340   1065  6    200   94   5    1154   2175   1117992   81    yes
054    106754   2340   1065  6    200   94   5    1154   2175   1101384   81    yes
055    106702   2340   1065  6    200   94   5    1151   2175   1098792   81    yes
056    106546   2340   1065  6    200   94   5    1150   2175   1098344   81    yes
057    105447   2313   1031  5    193   89   5    1105   2153   1085736   81    yes
058    105180   2305   1030  5    193   89   5    1103   2147   1081480   81    yes
059    104884   2305   1030  5    193   89   5    1103   2147   1081064   81    yes
060    98807    2155   983   5    189   87   5    1083   2066   1026568   80    yes
061    98759    2155   983   5    189   87   5    1083   2066   1022024   80    yes
062    97669    2113   956   5    189   87   5    1077   2059   1009000   80    yes
063    97361    2107   956   5    189   87   5    1074   2056   1004904   80    yes
064    94954    2079   946   5    189   87   5    1070   2012   983976    80    yes
065    94896    2077   942   5    189   87   5    1070   2007   983912    80    yes
066    94750    2077   942   5    189   87   5    1070   2007   983912    80    yes
067    94606    2074   924   6    188   87   5    1069   2007   979240    80    yes
068    94402                                                               fails     -     no -- .tmp/bnd/q068.c:8343:13: undefined: aucmd_win (check.go:5157:check:)
069    94311                                                               fails     -     no -- .tmp/bnd/q069.c:3990:31: unexpected '*', expected ')'
070    94094                                                               fails     -     no -- .tmp/bnd/q070.c:3986:31: unexpected '*', expected ')'
071    90229    1984   895   5    184   85   5    1041   1971   941704    80    yes
072    89622    1973   891   5    184   85   5    1027   1971   933512    80    yes
073    88932    1973   891   5    184   85   5    1023   1971   929416    80    yes
074    88702                                                               fails     -     no -- .tmp/bnd/q074.c:37167:33: unexpected '*', expected ')'
075    88390                                                               fails     -     no -- .tmp/bnd/q075.c:36912:33: unexpected '*', expected ')'
076    88333                                                               fails     -     no -- .tmp/bnd/q076.c:36910:33: unexpected '*', expected ')'
077    86960    1936   880   5    176   80   4    976    1837   905672    80    yes
078    86782    1921   874   5    176   80   4    973    1836   905672    80    yes
079    86198    1889   868   5    176   80   4    973    1834   897160    80    yes
080    84708    1883   862   5    176   80   4    973    1330   869288    80    yes
081    84674    1882   862   5    176   80   4    973    1330   869288    80    yes
082    84111    1882   862   0    170   74   4    961    1330   869288    80    yes
083    84111    1882   862   0    170   74   4    961    1330   869288    80    yes
084    84111    1882   862   0    170   74   4    961    1330   869288    80    yes
085    84083    1882   862   0    170   74   4    960    1330   869288    80    yes
086    84083    1882   862   0    170   74   4    960    1330   869288    80    yes
087    83359    1877   853   0    170   74   4    960    1327   865160    78    yes
088    83287    1875   853   0    170   74   4    959    1323   865160    78    yes
089    82277    1856   832   0    170   73   4    949    1303   855624    72    yes
090    82061    1850   827   0    170   73   4    948    1302   851240    72    yes
091    81398    1833   823   0    169   73   4    942    1286   842728    72    yes
092    80292    1817   820   0    169   73   4    942    1269   830216    69    yes
093    78157    1757   798   0    163   69   4    907    1197   812520    66    yes
094    77661    1741   794   0    163   69   4    905    1185   808232    66    yes
095    77565    1739   787   0    163   69   4    902    1182   803720    66    yes
096    77414    1734   782   0    163   69   4    902    1181   803720    62    yes
097    77697    1753   782   0    163   69   4    902    1181   807816    45    yes
098    78302    1771   784   0    163   69   4    902    1181   805352    33    yes
099    78294    1771   784   0    162   69   4    902    1181   805352    33    yes
100    78285    1771   784   0    162   69   4    902    1181   805352    32    yes
101    78291    1772   784   0    162   69   4    902    1181   805352    32    yes
102    78308    1773   787   0    162   69   4    902    1181   805352    31    yes
103    78062    1768   783   0    160   69   4    901    1177   801096    24    yes
104    78089    1769   784   0    160   69   4    901    1177   784200    17    yes
105    78114    1767   784   0    160   69   4    901    1177   788296    17    yes
106    78116    1767   784   0    161   69   4    901    1177   788296    17    yes
107    78116    1767   784   0    161   69   4    901    1177   788296    17    yes
108    78112    1767   782   0    161   69   4    901    1177   788296    17    yes
109    78136    1768   782   9    161   69   4    903    1177   788296    17    yes
110    78153    1768   782   9    161   69   4    903    1189   788296    17    yes
111    78144    1767   784   9    160   69   4    901    1189   788296    17    yes
112    77779    1767   782   9    160   69   4    901    1189   782568    17    yes
113    77693    1765   782   9    160   69   4    901    1189   782568    17    yes
114    77703    1767   782   7    160   69   4    901    1189   782568    17    yes
115    77702    1767   782   6    160   69   4    901    1189   782568    17    yes
116    77702    1767   782   6    160   69   4    901    1189   782568    17    yes
117    77712    1767   782   5    160   69   4    901    1189   782568    17    yes
118    77730    1770   782   2    160   69   4    901    1189   782568    17    yes
119    77724    1770   782   0    160   69   4    900    1189   782568    17    yes
120    77706    1770   782   0    160   69   4    894    1189   782568    17    yes
121    77588    1770   779   0    160   69   4    894    1189   780904    17    yes
122    77513    1768   779   0    160   69   4    893    1187   780872    17    yes
123    77513    1768   779   0    160   69   4    893    1187   780872    17    yes
124    77583    1771   781   0    160   69   4    893    1188   772680    14    yes
125    77220    1764   777   0    157   67   4    878    1172   768552    14    yes
126    76913    1753   775   0    154   65   4    864    1169   764360    14    yes
127    76810    1754   775   0    155   66   4    864    1168   764360    14    yes
128    76627    1745   774   0    154   65   4    857    1167   760232    14    yes
129    76627    1745   774   0    154   65   4    857    1167   760232    14    yes
130    76600    1745   774   0    154   65   4    857    1167   760232    14    yes
131    76599    1745   774   0    154   65   4    857    1167   760232    14    yes
132    76317    1744   774   0    154   65   4    857    1167   756136    14    yes
133    76280    1742   772   0    154   65   4    856    1166   751976    14    yes
134    76119    1742   772   0    154   65   4    855    1166   751976    14    yes
135    76110    1742   772   0    153   65   4    850    1166   751976    14    yes
136    76088    1742   771   0    152   64   4    845    1166   751912    14    yes
137    76059    1742   771   0    150   62   4    839    1162   751912    14    yes
138    75865    1742   770   0    133   53   4    761    1157   751848    14    yes
139    75871    1741   770   0    133   53   4    761    1157   751848    14    yes
140    75542    1732   766   0    130   51   4    749    1153   751784    14    yes
141    75581    1732   766   0    130   51   4    749    1153   751784    14    yes
142    75580    1732   766   0    130   51   4    749    1153   751784    14    yes
143    75608    1733   766   0    130   51   4    749    1153   751784    14    yes
144    75692    1735   766   0    130   51   4    749    1153   751784    14    yes
145    75697    1735   766   0    130   51   4    749    1153   751784    14    yes
146    75711    1735   766   0    130   51   4    749    1153   751784    14    yes
147    75763    1737   768   0    130   51   4    749    1153   751784    15    yes
148    75750    1737   768   0    130   51   4    749    1153   751784    15    yes
149    75132    1736   768   0    130   51   4    749    1153   747688    15    yes
150    75181    1738   771   0    130   51   4    749    1153   747688    15    yes
151    75180    1738   771   0    129   51   4    750    1153   750632    15    yes
152    75251    1744   771   0    130   51   4    754    1153   759176    15    yes
153    75251    1744   771   0    130   51   4    754    1153   759176    15    yes
154    75228    1743   771   0    130   51   4    754    1153   759176    15    yes
155    75272    1743   771   0    130   51   4    754    1153   759176    15    yes
156    75273    1743   772   0    130   51   4    754    1153   759176    15    yes
157    75273    1743   772   0    130   51   4    754    1153   759176    15    yes
158    75273    1743   772   0    130   51   4    754    1153   759176    15    yes
159    75276    1743   772   0    130   51   4    754    1153   759496    15    yes
160    75272    1743   772   0    129   51   4    753    1153   759496    15    yes
161    75279    1744   772   0    129   51   4    753    1153   759496    15    yes
162    75273    1743   772   0    129   51   4    753    1154   759496    15    yes
163    75225    1743   772   0    129   51   4    753    1154   759496    15    yes
```
