# The phases' numbers, old and new

The table `internal/phase/renumber` read once, when the pipeline was numbered
afresh (`doc/PHASES.md` is its account): every LIVE directory of the old
numbering and the identity it took. The FENCED BLOCK is the data; a reader takes
it and ignores the rest.

A row is `new old`:
- `new` a number alone is a phase of the plan, `internal/phase/NNN/`, its
  program registered as `whimN` -- the phase `old` was;
- `new` a number and a letter is a PART: a program the phase of that number
  runs among its steps, but that had a number of its own (a member of a
  compaction group, or a program the front calls), in
  `internal/phase/NNN/x/`, registered as `whimNx`, lettered in the order the
  phase runs them;
- `new` written `archive` is a directory that edited nothing and joins the
  records in `internal/phase/archive/`, under its old number.

The records already in `internal/phase/archive/` are not rows: they keep their
old numbers, and nothing of them moves.

```
0 0
0a 106
0b 105
0c 107
1 1
2 2
2a 61
3 3
3a 57
3b 58
3c 63
3d 65
3e 66
3f 74
4 6
4a 76
4b 67
4c 68
4d 71
4e 85
4f 59
5 7
5a 64
5b 72
5c 73
5d 75
6 14
7 16
8 17
9 20
10 22
11 23
12 25
13 28
14 32
15 48
15a 44
16 49
17 55
18 56
19 60
20 62
21 69
22 70
23 77
24 78
25 79
26 80
27 87
28 89
29 90
30 91
31 92
32 93
33 94
34 95
35 96
36 97
37 98
38 102
38a 100
38b 101
39 103
40 104
41 108
42 109
43 110
44 111
45 112
46 113
47 114
48 115
49 119
49a 117
49b 118
50 120
51 122
51a 121
52 124
53 125
54 126
55 127
56 128
57 129
58 130
59 131
60 132
61 133
62 134
63 135
64 136
65 137
66 138
67 139
68 140
69 141
70 142
71 145
71a 143
71b 144
72 146
73 147
74 149
74a 148
75 150
76 152
76a 151
77 154
77a 153
78 155
79 156
80 157
81 158
82 159
83 160
84 161
85 162
86 165
86a 164
87 167
87a 166
88 169
89 170
90 171
91 172
92 173
93 174
94 175
95 176
96 177
97 178
98 179
99 180
100 181
101 182
102 183
103 184
archive 45
archive 46
archive 47
```
