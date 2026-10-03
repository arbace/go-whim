# Every boundary, measured

The text after every phase -- the snapshots a whole `make whim-build` keeps in
`.cache/boundaries/`, every boundary printed canonically -- counted by `go tool
whim measure .cache/boundaries`.  The build's product was `whim-vim.c` byte for
byte, and so was `q103`.

**Since the fresh numbering** (`doc/PHASES.md`, 2026-10-02): the plan has 104
phases, numbered 0-103 by their place in it, and a row is printed for each and
for nothing else; the snapshots are named by the new numbers. Phases 1-3 are
the front: they cut, at the start, what the product has not, and 4 and 5 cut
what counts on the text they leave. The phases that edit nothing are records in
`internal/phase/archive/`, under their old numbers, and the groups of
`doc/PIPELINE-COMPACTION.md` §3d that still run are rows under the number of
the phase they are. Measured again in full, by the same command, on the
snapshots of a build that gave the product byte for byte.

A row is the boundary AFTER that phase: after its sweep and canonical print,
which every phase has now -- there are no stages, and every row parses.
The columns are `internal/reach`'s entity counts, so a count means the same
thing on every row: `F` function definitions
(prototypes merged in), `O` file-scope objects, `P` prototypes of functions
never defined, `T` typedefs, `S` tagged struct and union definitions, `E`
tagged enum definitions, `M` members, `N` enumerators. `binary` is bytes and
`nm-u` the undefined symbols of the object, both built with THE ONE COMPILE LINE,
`gcc -O0 -fno-stack-protector -static -no-pie -s` (`build.FlagsFor`), with
`SOURCE_DATE_EPOCH=0` -- the same line for every row, so rows whose text is one
text -- 101, 102 and 103 -- are one binary.

**Measured again after the 37 `[[fallthrough]];` were carried through**
(2026-10-03, `doc/C23.md`): every boundary from q000 on now holds the
attribute statements phase 0c writes, where it held a bare `;` -- 37 at q000,
34, 33, 22, 22, then 20 from q005 to the product -- and every row below came
back the same, the binaries included: an attribute is one line for one line
and emits no code.

The phases' `GOAL.md` `Measured` tables that predate canonical seeding
(`d8365fb`) are records of the pipeline as it then ran, and keep their numbers;
this file is what the pipeline measures now. Re-measure it with the two
commands above -- never edit a number by hand.

```
phase  lines    F      O     P    T     S    E    M      N      binary    nm-u  parses
000    173552   3350   1579  1    232   104  5    1546   2858   2015480   145   yes
001    155878   2964   1330  1    220   99   5    1443   2420   1770632   133   yes
002    147575   2844   1277  1    217   97   5    1419   2345   1535496   124   yes
003    110302   2377   1089  1    186   82   5    1164   1858   1143912   117   yes
004    96329    2108   983   1    175   79   5    1102   1648   1014472   98    yes
005    87671    1968   914   0    170   76   5    1022   1389   922216    89    yes
006    86450    1956   907   0    170   76   5    1010   1385   909512    89    yes
007    86374    1956   901   0    170   76   5    1004   1379   909448    89    yes
008    86359    1956   900   0    170   76   5    1002   1377   909448    89    yes
009    85167    1932   891   0    170   76   5    1002   1377   888968    85    yes
010    85045    1929   889   0    170   76   5    1002   1377   884808    83    yes
011    84875    1928   888   0    170   76   5    1002   1376   884712    82    yes
012    84833    1927   888   0    170   76   5    1000   1373   884712    80    yes
013    84792    1927   884   0    170   76   5    995    1368   884712    80    yes
014    84745    1927   883   0    170   76   5    989    1362   880616    80    yes
015    84727    1927   883   0    170   76   5    989    1361   880616    80    yes
016    84635    1926   879   0    170   76   5    987    1355   880520    80    yes
017    84612    1926   877   0    170   76   5    984    1352   880520    80    yes
018    84603    1926   877   0    170   76   5    983    1351   880520    80    yes
019    84350    1921   867   0    170   76   5    981    1346   876360    79    yes
020    82691    1877   840   0    167   75   4    960    1312   863016    70    yes
021    82427    1870   836   0    165   73   4    952    1307   858856    70    yes
022    82197    1865   834   0    165   73   4    950    1303   858856    70    yes
023    81897    1861   831   0    165   73   4    950    1302   854664    70    yes
024    81746    1849   826   0    165   73   4    947    1301   850568    70    yes
025    81218    1819   820   0    165   73   4    947    1299   846152    70    yes
026    80809    1814   817   0    165   73   4    946    1271   841928    70    yes
027    80191    1809   809   0    165   73   4    946    1268   837800    68    yes
028    80168    1809   808   0    165   73   4    945    1266   837768    68    yes
029    80143    1809   808   0    165   73   4    944    1265   837768    68    yes
030    79451    1792   803   0    164   73   4    938    1248   829352    68    yes
031    79450    1792   803   0    164   73   4    938    1248   829352    68    yes
032    77515    1737   785   0    158   69   4    903    1182   811816    65    yes
033    77499    1736   785   0    158   69   4    901    1182   811816    65    yes
034    77436    1735   785   0    158   69   4    898    1179   807720    65    yes
035    77295    1730   781   0    158   69   4    898    1178   807720    62    yes
036    77575    1749   781   0    158   69   4    898    1178   811816    45    yes
037    78174    1767   783   0    158   69   4    898    1178   809352    33    yes
038    78189    1769   786   0    158   69   4    898    1178   809352    31    yes
039    77962    1765   782   0    156   69   4    897    1174   805160    24    yes
040    77985    1766   783   0    156   69   4    897    1174   788264    17    yes
041    77983    1766   781   0    156   69   4    897    1174   788264    17    yes
042    78016    1767   781   9    156   69   4    899    1174   788264    17    yes
043    78064    1767   781   9    156   69   4    899    1186   788264    17    yes
044    78056    1766   783   9    155   69   4    897    1186   788264    17    yes
045    77691    1766   781   9    155   69   4    897    1186   782536    17    yes
046    77605    1764   781   9    155   69   4    897    1186   782536    17    yes
047    77613    1766   781   7    155   69   4    897    1186   782536    17    yes
048    77613    1766   781   6    155   69   4    897    1186   782536    17    yes
049    77634    1769   781   0    155   69   4    896    1186   782536    17    yes
050    77619    1769   781   0    155   69   4    891    1186   782536    17    yes
051    77475    1768   778   0    155   69   4    890    1186   780872    17    yes
052    77539    1771   780   0    155   69   4    890    1187   772680    14    yes
053    77178    1764   777   0    152   67   4    875    1171   768552    14    yes
054    76865    1753   775   0    149   65   4    861    1168   764360    14    yes
055    76757    1754   775   0    150   66   4    861    1167   764360    14    yes
056    76576    1745   774   0    149   65   4    854    1166   760232    14    yes
057    76576    1745   774   0    149   65   4    854    1166   760232    14    yes
058    76549    1745   774   0    149   65   4    854    1166   760232    14    yes
059    76548    1745   774   0    149   65   4    854    1166   760232    14    yes
060    76267    1744   774   0    149   65   4    854    1166   756136    14    yes
061    76230    1742   772   0    149   65   4    853    1165   751976    14    yes
062    76073    1742   772   0    149   65   4    853    1165   751976    14    yes
063    76064    1742   772   0    148   65   4    848    1165   751976    14    yes
064    76042    1742   771   0    147   64   4    843    1165   751912    14    yes
065    76010    1742   771   0    145   62   4    837    1161   751912    14    yes
066    75822    1742   770   0    131   53   4    759    1156   751848    14    yes
067    75828    1741   770   0    131   53   4    759    1156   751848    14    yes
068    75496    1732   766   0    127   51   4    747    1152   751784    14    yes
069    75535    1732   766   0    127   51   4    747    1152   751784    14    yes
070    75534    1732   766   0    127   51   4    747    1152   751784    14    yes
071    75646    1735   766   0    127   51   4    747    1152   751784    14    yes
072    75660    1735   766   0    127   51   4    747    1152   751784    14    yes
073    75715    1737   768   0    127   51   4    747    1152   751784    15    yes
074    75053    1736   768   0    127   51   4    747    1152   747688    15    yes
075    75099    1738   771   0    127   51   4    747    1152   747688    15    yes
076    75189    1744   771   0    127   51   4    752    1152   759176    15    yes
077    75166    1743   771   0    127   51   4    752    1152   759176    15    yes
078    75199    1743   771   0    127   51   4    752    1152   759176    15    yes
079    75201    1743   772   0    127   51   4    752    1152   759176    15    yes
080    75201    1743   772   0    127   51   4    752    1152   759176    15    yes
081    75201    1743   772   0    127   51   4    752    1152   759176    15    yes
082    75204    1743   772   0    127   51   4    752    1152   759496    15    yes
083    75202    1743   772   0    127   51   4    751    1152   759496    15    yes
084    75207    1744   772   0    127   51   4    751    1152   759496    15    yes
085    75201    1743   772   0    127   51   4    751    1153   759496    15    yes
086    75128    1743   769   0    127   51   4    751    1151   758632    15    yes
087    75434    1743   769   0    127   51   4    751    1304   758632    15    yes
088    75405    1743   769   0    127   51   4    751    1304   758632    15    yes
089    75509    1743   769   0    127   51   4    751    1304   762728    15    yes
090    75506    1743   769   0    127   51   4    751    1304   762728    15    yes
091    75508    1743   769   0    127   51   4    751    1304   762728    15    yes
092    75541    1743   769   0    127   51   4    751    1304   762728    15    yes
093    75348    1745   769   0    127   51   4    751    1304   762728    15    yes
094    75383    1749   769   0    127   51   4    751    1304   762728    15    yes
095    75351    1749   754   0    128   52   4    767    1304   762728    15    yes
096    75644    1752   754   0    130   52   4    778    1304   766824    15    yes
097    75646    1752   754   0    130   52   4    778    1304   766824    15    yes
098    75646    1752   754   0    130   52   4    778    1304   766824    15    yes
099    75651    1752   754   0    130   52   4    778    1304   770920    17    yes
100    77632    1752   754   0    184   52   4    914    1304   779112    17    yes
101    77635    1755   754   0    184   52   4    914    1304   775016    17    yes
102    77635    1755   754   0    184   52   4    914    1304   775016    17    yes
103    77635    1755   754   0    184   52   4    914    1304   775016    17    yes
```
