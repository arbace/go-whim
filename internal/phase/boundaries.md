# Every boundary, measured

The text after every phase -- the snapshots a whole `make whim-build` keeps in
`.cache/boundaries/`, every boundary printed canonically -- counted by `go tool
whim measure .cache/boundaries`.  The build's product was `whim-vim.c` byte for
byte, and so was `q163`.

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
000    173594   3352   1579  7    251   112  5    1625   2898   2003192   145   yes
001    173594   3352   1579  7    251   112  5    1625   2898   2003192   145   yes
002    173584   3352   1579  7    251   112  5    1625   2898   2002616   145   yes
003    173376   3350   1579  7    251   112  5    1625   2898   1998520   145   yes
004    173375   3350   1579  7    251   112  5    1625   2898   1998520   145   yes
005    159393   3270   1579  20   251   112  5    1625   2898   1826200   145   yes
006    159278   3270   1579  20   251   112  5    1625   2898   1821976   145   yes
007    159180   3270   1579  20   251   112  5    1625   2898   1821976   145   yes
008    158826   3270   1579  20   251   112  5    1625   2898   1817880   145   yes
009    158797   3270   1579  20   251   112  5    1625   2898   1817496   144   yes
010    158749   3270   1579  20   251   112  5    1625   2898   1817080   144   yes
011    158521   3270   1579  20   251   112  5    1625   2898   1812824   143   yes
012    147446   3102   1458  2    217   98   5    1453   2515   1564984   117   yes
013    147396   3102   1458  2    217   98   5    1453   2515   1564984   117   yes
014    147411   3102   1458  2    217   98   5    1453   2515   1564984   117   yes
015    147392   3102   1458  2    217   98   5    1453   2515   1564856   117   yes
016    145479   3078   1435  2    217   98   5    1447   2508   1543160   117   yes
017    145329   3078   1435  2    217   98   5    1443   2508   1538904   117   yes
018    145194   3078   1435  2    217   98   5    1441   2508   1538712   117   yes
019    145165   3078   1435  2    217   98   5    1441   2508   1538712   117   yes
020    144701   3076   1435  2    217   98   5    1441   2508   1534520   116   yes
021    141894   3017   1395  2    210   93   5    1346   2480   1492104   102   yes
022    141760                                                              fails     -     no -- .cache/boundaries/q022.c:141557:1: conflicting types for 'edit_buffers', previous declaration at .cache/boundaries/q022.c:140712:13:, function(pointer to mparm_T) and function(pointer to mparm_T, pointer to char_u) (check.go:269:checkScope:)
023    141614                                                              fails     -     no -- .cache/boundaries/q023.c:141411:1: conflicting types for 'edit_buffers', previous declaration at .cache/boundaries/q023.c:140566:13:, function(pointer to mparm_T) and function(pointer to mparm_T, pointer to char_u) (check.go:269:checkScope:)
024    138826   2966   1374  3    209   91   5    1337   2442   fails     98    yes
025    138043                                                              fails     -     no -- .cache/boundaries/q025.c:73236:22: undefined: B0_UNAME_SIZE (check.go:5157:check:)
026    137964                                                              fails     -     no -- .cache/boundaries/q026.c:73234:22: undefined: B0_UNAME_SIZE (check.go:5157:check:)
027    137959                                                              fails     -     no -- .cache/boundaries/q027.c:73234:22: undefined: B0_UNAME_SIZE (check.go:5157:check:)
028    132553   2886   1346  2    207   91   5    1291   2414   fails     88    yes
029    132530                                                              fails     -     no -- .cache/boundaries/q029.c:123016:38: type struct file_buffer {b_ml memline_T; b_next pointer to struct file_buffer; b_prev pointer to struct file_buffer; b_nwindows int; b_flags int; b_locked int; b_locked_split int; b_ffname pointer to char_u; b_sfname pointer to char_u; b_fname pointer to char_u; b_dev_valid _Bool; b_dev dev_t; b_ino ino_t; b_fnum int; b_key array of 9 char_u; b_changed int; b_ct_di dictitem16_T; b_last_changedtick varnumber_T; b_last_changedtick_pum varnumber_T; b_last_changedtick_i varnumber_T; b_saving _Bool; b_mod_set _Bool; b_mod_top linenr_T; b_mod_bot linenr_T; b_mod_xlines long; b_wininfo pointer to struct wininfo_S; b_mtime long; b_mtime_ns long; b_mtime_read long; b_mtime_read_ns long; b_orig_size off_T; b_orig_mode int; b_namedm array of 26 pos_T; b_visual visualinfo_T; b_last_cursor pos_T; b_last_insert pos_T; b_last_change pos_T; b_changelist array of 100 pos_T; b_changelistlen int; b_new_change _Bool; b_chartab array of 32 char_u; b_maphash array of 256 pointer to struct mapblock; b_first_abbr pointer to struct mapblock; b_op_start pos_T; b_op_start_orig pos_T; b_op_end pos_T; b_modified_was_set _Bool; b_did_filetype _Bool; b_keep_filetype _Bool; b_au_did_filetype _Bool; b_u_oldhead pointer to struct u_header; b_u_newhead pointer to struct u_header; b_u_curhead pointer to struct u_header; b_u_numhead int; b_u_synced _Bool; b_u_seq_last long; b_u_save_nr_last long; b_u_seq_cur long; b_u_time_cur time_T; b_u_save_nr_cur long; b_u_line_ptr undoline_T; b_u_line_lnum linenr_T; b_u_line_colnr colnr_T; b_scanned _Bool; b_p_iminsert long; b_p_imsearch long; b_p_initialized _Bool; b_p_ac int; b_p_ai int; b_p_ai_nopaste int; b_bkc_flags unsigned; b_p_ci int; b_p_bin int; b_p_bh pointer to char_u; b_p_bt pointer to char_u; b_p_bl int; b_p_com pointer to char_u; b_p_cms pointer to char_u; b_p_cot pointer to char_u; b_cot_flags unsigned; b_p_cpt pointer to char_u; b_p_eof int; b_p_eol int; b_p_fixeol int; b_p_et int; b_p_et_nobin int; b_p_et_nopaste int; b_p_ff pointer to char_u; b_p_ft pointer to char_u; b_p_fo pointer to char_u; b_p_flp pointer to char_u; b_p_inf int; b_p_isk pointer to char_u; b_p_fp pointer to char_u; b_p_fs int; b_p_kp pointer to char_u; b_p_lisp int; b_p_lop pointer to char_u; b_p_menc pointer to char_u; b_p_mps pointer to char_u; b_p_ml int; b_p_ml_nobin int; b_p_ma int; b_p_nf pointer to char_u; b_p_pi int; b_p_qe pointer to char_u; b_p_ro int; b_p_sw long; b_p_sn int; b_p_si int; b_p_sts long; b_p_sts_nopaste long; b_p_ts long; b_p_tx int; b_p_tw long; b_p_tw_nobin long; b_p_tw_nopaste long; b_p_wm long; b_p_wm_nobin long; b_p_wm_nopaste long; b_p_ep pointer to char_u; b_tc_flags unsigned; b_p_dict pointer to char_u; b_p_tsr pointer to char_u; b_p_ul long; b_p_lw pointer to char_u; b_no_eol_lnum linenr_T; b_start_eof int; b_start_eol int; b_start_ffc int; b_bad_char int; b_may_swap _Bool; b_did_warn _Bool; b_help _Bool; b_shortname _Bool; b_mapped_ctrl_c int} has no member named b_ucmds (check.go:4913:check:)
030    132371                                                              fails     -     no -- .cache/boundaries/q030.c:122888:38: type struct file_buffer {b_ml memline_T; b_next pointer to struct file_buffer; b_prev pointer to struct file_buffer; b_nwindows int; b_flags int; b_locked int; b_locked_split int; b_ffname pointer to char_u; b_sfname pointer to char_u; b_fname pointer to char_u; b_dev_valid _Bool; b_dev dev_t; b_ino ino_t; b_fnum int; b_key array of 9 char_u; b_changed int; b_ct_di dictitem16_T; b_last_changedtick varnumber_T; b_last_changedtick_pum varnumber_T; b_last_changedtick_i varnumber_T; b_saving _Bool; b_mod_set _Bool; b_mod_top linenr_T; b_mod_bot linenr_T; b_mod_xlines long; b_wininfo pointer to struct wininfo_S; b_mtime long; b_mtime_ns long; b_mtime_read long; b_mtime_read_ns long; b_orig_size off_T; b_orig_mode int; b_namedm array of 26 pos_T; b_visual visualinfo_T; b_last_cursor pos_T; b_last_insert pos_T; b_last_change pos_T; b_changelist array of 100 pos_T; b_changelistlen int; b_new_change _Bool; b_chartab array of 32 char_u; b_maphash array of 256 pointer to struct mapblock; b_first_abbr pointer to struct mapblock; b_op_start pos_T; b_op_start_orig pos_T; b_op_end pos_T; b_modified_was_set _Bool; b_did_filetype _Bool; b_keep_filetype _Bool; b_au_did_filetype _Bool; b_u_oldhead pointer to struct u_header; b_u_newhead pointer to struct u_header; b_u_curhead pointer to struct u_header; b_u_numhead int; b_u_synced _Bool; b_u_seq_last long; b_u_save_nr_last long; b_u_seq_cur long; b_u_time_cur time_T; b_u_save_nr_cur long; b_u_line_ptr undoline_T; b_u_line_lnum linenr_T; b_u_line_colnr colnr_T; b_scanned _Bool; b_p_iminsert long; b_p_imsearch long; b_p_initialized _Bool; b_p_ac int; b_p_ai int; b_p_ai_nopaste int; b_bkc_flags unsigned; b_p_ci int; b_p_bin int; b_p_bh pointer to char_u; b_p_bt pointer to char_u; b_p_bl int; b_p_com pointer to char_u; b_p_cms pointer to char_u; b_p_cot pointer to char_u; b_cot_flags unsigned; b_p_cpt pointer to char_u; b_p_eof int; b_p_eol int; b_p_fixeol int; b_p_et int; b_p_et_nobin int; b_p_et_nopaste int; b_p_ff pointer to char_u; b_p_ft pointer to char_u; b_p_fo pointer to char_u; b_p_flp pointer to char_u; b_p_inf int; b_p_isk pointer to char_u; b_p_fp pointer to char_u; b_p_fs int; b_p_kp pointer to char_u; b_p_lisp int; b_p_lop pointer to char_u; b_p_menc pointer to char_u; b_p_mps pointer to char_u; b_p_ml int; b_p_ml_nobin int; b_p_ma int; b_p_nf pointer to char_u; b_p_pi int; b_p_qe pointer to char_u; b_p_ro int; b_p_sw long; b_p_sn int; b_p_si int; b_p_sts long; b_p_sts_nopaste long; b_p_ts long; b_p_tx int; b_p_tw long; b_p_tw_nobin long; b_p_tw_nopaste long; b_p_wm long; b_p_wm_nobin long; b_p_wm_nopaste long; b_p_ep pointer to char_u; b_tc_flags unsigned; b_p_dict pointer to char_u; b_p_tsr pointer to char_u; b_p_ul long; b_p_lw pointer to char_u; b_no_eol_lnum linenr_T; b_start_eof int; b_start_eol int; b_start_ffc int; b_bad_char int; b_may_swap _Bool; b_did_warn _Bool; b_help _Bool; b_shortname _Bool; b_mapped_ctrl_c int} has no member named b_ucmds (check.go:4913:check:)
031    132056                                                              fails     -     no -- .cache/boundaries/q031.c:122573:38: type struct file_buffer {b_ml memline_T; b_next pointer to struct file_buffer; b_prev pointer to struct file_buffer; b_nwindows int; b_flags int; b_locked int; b_locked_split int; b_ffname pointer to char_u; b_sfname pointer to char_u; b_fname pointer to char_u; b_dev_valid _Bool; b_dev dev_t; b_ino ino_t; b_fnum int; b_key array of 9 char_u; b_changed int; b_ct_di dictitem16_T; b_last_changedtick varnumber_T; b_last_changedtick_pum varnumber_T; b_last_changedtick_i varnumber_T; b_saving _Bool; b_mod_set _Bool; b_mod_top linenr_T; b_mod_bot linenr_T; b_mod_xlines long; b_wininfo pointer to struct wininfo_S; b_mtime long; b_mtime_ns long; b_mtime_read long; b_mtime_read_ns long; b_orig_size off_T; b_orig_mode int; b_namedm array of 26 pos_T; b_visual visualinfo_T; b_last_cursor pos_T; b_last_insert pos_T; b_last_change pos_T; b_changelist array of 100 pos_T; b_changelistlen int; b_new_change _Bool; b_chartab array of 32 char_u; b_maphash array of 256 pointer to struct mapblock; b_first_abbr pointer to struct mapblock; b_op_start pos_T; b_op_start_orig pos_T; b_op_end pos_T; b_modified_was_set _Bool; b_did_filetype _Bool; b_keep_filetype _Bool; b_au_did_filetype _Bool; b_u_oldhead pointer to struct u_header; b_u_newhead pointer to struct u_header; b_u_curhead pointer to struct u_header; b_u_numhead int; b_u_synced _Bool; b_u_seq_last long; b_u_save_nr_last long; b_u_seq_cur long; b_u_time_cur time_T; b_u_save_nr_cur long; b_u_line_ptr undoline_T; b_u_line_lnum linenr_T; b_u_line_colnr colnr_T; b_scanned _Bool; b_p_iminsert long; b_p_imsearch long; b_p_initialized _Bool; b_p_ac int; b_p_ai int; b_p_ai_nopaste int; b_bkc_flags unsigned; b_p_ci int; b_p_bin int; b_p_bh pointer to char_u; b_p_bt pointer to char_u; b_p_bl int; b_p_com pointer to char_u; b_p_cms pointer to char_u; b_p_cot pointer to char_u; b_cot_flags unsigned; b_p_cpt pointer to char_u; b_p_eof int; b_p_eol int; b_p_fixeol int; b_p_et int; b_p_et_nobin int; b_p_et_nopaste int; b_p_ff pointer to char_u; b_p_ft pointer to char_u; b_p_fo pointer to char_u; b_p_flp pointer to char_u; b_p_inf int; b_p_isk pointer to char_u; b_p_fp pointer to char_u; b_p_fs int; b_p_kp pointer to char_u; b_p_lisp int; b_p_lop pointer to char_u; b_p_menc pointer to char_u; b_p_mps pointer to char_u; b_p_ml int; b_p_ml_nobin int; b_p_ma int; b_p_nf pointer to char_u; b_p_pi int; b_p_qe pointer to char_u; b_p_ro int; b_p_sw long; b_p_sn int; b_p_si int; b_p_sts long; b_p_sts_nopaste long; b_p_ts long; b_p_tx int; b_p_tw long; b_p_tw_nobin long; b_p_tw_nopaste long; b_p_wm long; b_p_wm_nobin long; b_p_wm_nopaste long; b_p_ep pointer to char_u; b_tc_flags unsigned; b_p_dict pointer to char_u; b_p_tsr pointer to char_u; b_p_ul long; b_p_lw pointer to char_u; b_no_eol_lnum linenr_T; b_start_eof int; b_start_eol int; b_start_ffc int; b_bad_char int; b_may_swap _Bool; b_did_warn _Bool; b_help _Bool; b_shortname _Bool; b_mapped_ctrl_c int} has no member named b_ucmds (check.go:4913:check:)
032    124175   2732   1278  2    204   90   5    1245   2355   fails     88    yes
033    124165   2732   1278  2    204   90   5    1245   2355   fails     88    yes
034    124141   2732   1278  2    204   90   5    1245   2355   fails     88    yes
035    123607                                                              fails     -     no -- .cache/boundaries/q035.c:119211:78: type winopt_T has no member named wo_eiw (check.go:4882:check:)
036    123271                                                              fails     -     no -- .cache/boundaries/q036.c:118886:78: type winopt_T has no member named wo_eiw (check.go:4882:check:)
037    123242                                                              fails     -     no -- .cache/boundaries/q037.c:118857:78: type winopt_T has no member named wo_eiw (check.go:4882:check:)
038    123208                                                              fails     -     no -- .cache/boundaries/q038.c:118823:78: type winopt_T has no member named wo_eiw (check.go:4882:check:)
039    122789                                                              fails     -     no -- .cache/boundaries/q039.c:118453:78: type winopt_T has no member named wo_eiw (check.go:4882:check:)
040    122650                                                              fails     -     no -- .cache/boundaries/q040.c:118422:78: type winopt_T has no member named wo_eiw (check.go:4882:check:)
041    112966   2437   1129  1    197   88   5    1187   2269   1192040   85    yes
042    112480   2424   1119  1    197   88   5    1185   2259   1187432   85    yes
043    112422   2424   1119  1    197   88   5    1185   2259   1183336   85    yes
044    112418   2424   1119  1    197   88   5    1185   2259   1183336   85    yes
045    112418   2424   1119  1    197   88   5    1185   2259   1183336   85    yes
046    112418   2424   1119  1    197   88   5    1185   2259   1183336   85    yes
047    112418   2424   1119  1    197   88   5    1185   2259   1183336   85    yes
048    112401   2424   1119  1    197   88   5    1185   2259   1183336   85    yes
049    110833   2397   1102  1    195   88   5    1172   2254   1162184   84    yes
050    110027   2379   1093  1    195   88   5    1157   2244   1153416   84    yes
051    109931   2379   1093  1    195   88   5    1157   2244   1153416   84    yes
052    108283   2379   1088  1    195   88   5    1157   2244   1137000   82    yes
053    106309   2340   1065  1    194   88   5    1142   2175   1117992   81    yes
054    106131   2340   1065  1    194   88   5    1142   2175   1101384   81    yes
055    106079   2340   1065  1    194   88   5    1139   2175   1098792   81    yes
056    105923   2340   1065  1    194   88   5    1138   2175   1098344   81    yes
057    104824   2313   1031  0    187   83   5    1093   2153   1085736   81    yes
058    104557   2305   1030  0    187   83   5    1091   2147   1081480   81    yes
059    104261   2305   1030  0    187   83   5    1091   2147   1081064   81    yes
060    98183    2155   983   0    183   81   5    1071   2066   1026568   80    yes
061    98135    2155   983   0    183   81   5    1071   2066   1022024   80    yes
062    97045    2113   956   0    183   81   5    1065   2059   1009000   80    yes
063    96737    2107   956   0    183   81   5    1062   2056   1004904   80    yes
064    94330    2079   946   0    183   81   5    1058   2012   983976    80    yes
065    94272    2077   942   0    183   81   5    1058   2007   983912    80    yes
066    94126    2077   942   0    183   81   5    1058   2007   983912    80    yes
067    93961    2074   924   1    182   81   5    1057   2007   979240    80    yes
068    93751                                                               fails     -     no -- .cache/boundaries/q068.c:7996:13: undefined: aucmd_win (check.go:5157:check:)
069    93660                                                               fails     -     no -- .cache/boundaries/q069.c:3859:31: unexpected '*', expected ')'
070    93443                                                               fails     -     no -- .cache/boundaries/q070.c:3855:31: unexpected '*', expected ')'
071    89599    1984   895   0    178   79   5    1029   1971   941704    80    yes
072    88985    1973   891   0    178   79   5    1015   1971   933512    80    yes
073    88289    1973   891   0    178   79   5    1011   1971   929416    80    yes
074    88059                                                               fails     -     no -- .cache/boundaries/q074.c:36718:33: unexpected '*', expected ')'
075    87747                                                               fails     -     no -- .cache/boundaries/q075.c:36463:33: unexpected '*', expected ')'
076    87690                                                               fails     -     no -- .cache/boundaries/q076.c:36461:33: unexpected '*', expected ')'
077    86324    1936   880   0    170   74   4    964    1837   905672    80    yes
078    86146    1921   874   0    170   74   4    961    1836   905672    80    yes
079    85559    1889   868   0    170   74   4    961    1834   897160    80    yes
080    84072    1883   862   0    170   74   4    961    1330   869288    80    yes
081    84038    1882   862   0    170   74   4    961    1330   869288    80    yes
082    84015    1882   862   0    170   74   4    961    1330   869288    80    yes
083    84015    1882   862   0    170   74   4    961    1330   869288    80    yes
084    84015    1882   862   0    170   74   4    961    1330   869288    80    yes
085    83987    1882   862   0    170   74   4    960    1330   869288    80    yes
086    83987    1882   862   0    170   74   4    960    1330   869288    80    yes
087    83265    1877   853   0    170   74   4    960    1327   865160    78    yes
088    83193    1875   853   0    170   74   4    959    1323   865160    78    yes
089    82184    1856   832   0    170   73   4    949    1303   855624    72    yes
090    81969    1850   827   0    170   73   4    948    1302   851240    72    yes
091    81306    1833   823   0    169   73   4    942    1286   842728    72    yes
092    80200    1817   820   0    169   73   4    942    1269   830216    69    yes
093    78075    1757   798   0    163   69   4    907    1197   812520    66    yes
094    77579    1741   794   0    163   69   4    905    1185   808232    66    yes
095    77483    1739   787   0    163   69   4    902    1182   803720    66    yes
096    77332    1734   782   0    163   69   4    902    1181   803720    62    yes
097    77612    1753   782   0    163   69   4    902    1181   807816    45    yes
098    78211    1771   784   0    163   69   4    902    1181   805352    33    yes
099    78203    1771   784   0    162   69   4    902    1181   805352    33    yes
100    78194    1771   784   0    162   69   4    902    1181   805352    32    yes
101    78200    1772   784   0    162   69   4    902    1181   805352    32    yes
102    78218    1773   787   0    162   69   4    902    1181   805352    31    yes
103    77970    1768   783   0    160   69   4    901    1177   801096    24    yes
104    77992    1769   784   0    160   69   4    901    1177   784200    17    yes
105    78022    1767   784   0    160   69   4    901    1177   788296    17    yes
106    78024    1767   784   0    161   69   4    901    1177   788296    17    yes
107    78024    1767   784   0    161   69   4    901    1177   788296    17    yes
108    78022    1767   782   0    161   69   4    901    1177   788296    17    yes
109    78055    1768   782   9    161   69   4    903    1177   788296    17    yes
110    78103    1768   782   9    161   69   4    903    1189   788296    17    yes
111    78095    1767   784   9    160   69   4    901    1189   788296    17    yes
112    77730    1767   782   9    160   69   4    901    1189   782568    17    yes
113    77644    1765   782   9    160   69   4    901    1189   782568    17    yes
114    77652    1767   782   7    160   69   4    901    1189   782568    17    yes
115    77652    1767   782   6    160   69   4    901    1189   782568    17    yes
116    77652    1767   782   6    160   69   4    901    1189   782568    17    yes
117    77661    1767   782   5    160   69   4    901    1189   782568    17    yes
118    77679    1770   782   2    160   69   4    901    1189   782568    17    yes
119    77673    1770   782   0    160   69   4    900    1189   782568    17    yes
120    77655    1770   782   0    160   69   4    894    1189   782568    17    yes
121    77537    1770   779   0    160   69   4    894    1189   780904    17    yes
122    77462    1768   779   0    160   69   4    893    1187   780872    17    yes
123    77462    1768   779   0    160   69   4    893    1187   780872    17    yes
124    77526    1771   781   0    160   69   4    893    1188   772680    14    yes
125    77163    1764   777   0    157   67   4    878    1172   768552    14    yes
126    76850    1753   775   0    154   65   4    864    1169   764360    14    yes
127    76742    1754   775   0    155   66   4    864    1168   764360    14    yes
128    76561    1745   774   0    154   65   4    857    1167   760232    14    yes
129    76561    1745   774   0    154   65   4    857    1167   760232    14    yes
130    76534    1745   774   0    154   65   4    857    1167   760232    14    yes
131    76533    1745   774   0    154   65   4    857    1167   760232    14    yes
132    76251    1744   774   0    154   65   4    857    1167   756136    14    yes
133    76214    1742   772   0    154   65   4    856    1166   751976    14    yes
134    76053    1742   772   0    154   65   4    855    1166   751976    14    yes
135    76044    1742   772   0    153   65   4    850    1166   751976    14    yes
136    76022    1742   771   0    152   64   4    845    1166   751912    14    yes
137    75990    1742   771   0    150   62   4    839    1162   751912    14    yes
138    75798    1742   770   0    133   53   4    761    1157   751848    14    yes
139    75804    1741   770   0    133   53   4    761    1157   751848    14    yes
140    75474    1732   766   0    130   51   4    749    1153   751784    14    yes
141    75513    1732   766   0    130   51   4    749    1153   751784    14    yes
142    75512    1732   766   0    130   51   4    749    1153   751784    14    yes
143    75539    1733   766   0    130   51   4    749    1153   751784    14    yes
144    75619    1735   766   0    130   51   4    749    1153   751784    14    yes
145    75624    1735   766   0    130   51   4    749    1153   751784    14    yes
146    75638    1735   766   0    130   51   4    749    1153   751784    14    yes
147    75694    1737   768   0    130   51   4    749    1153   751784    15    yes
148    75680    1737   768   0    130   51   4    749    1153   751784    15    yes
149    75062    1736   768   0    130   51   4    749    1153   747688    15    yes
150    75108    1738   771   0    130   51   4    749    1153   747688    15    yes
151    75107    1738   771   0    129   51   4    750    1153   750632    15    yes
152    75198    1744   771   0    130   51   4    754    1153   759176    15    yes
153    75198    1744   771   0    130   51   4    754    1153   759176    15    yes
154    75175    1743   771   0    130   51   4    754    1153   759176    15    yes
155    75208    1743   771   0    130   51   4    754    1153   759176    15    yes
156    75210    1743   772   0    130   51   4    754    1153   759176    15    yes
157    75210    1743   772   0    130   51   4    754    1153   759176    15    yes
158    75210    1743   772   0    130   51   4    754    1153   759176    15    yes
159    75213    1743   772   0    130   51   4    754    1153   759496    15    yes
160    75209    1743   772   0    129   51   4    753    1153   759496    15    yes
161    75214    1744   772   0    129   51   4    753    1153   759496    15    yes
162    75208    1743   772   0    129   51   4    753    1154   759496    15    yes
163    75208    1743   772   0    129   51   4    753    1154   759496    15    yes
```
