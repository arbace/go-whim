# FoldX with every seed: what the text closure gave

`TestFoldXEverySeed` holds FoldX with every unwritten object and member a
seed, on the graphs of three snapshots, collected, to what crefactor/xform's
text fall-out closure gave on the same snapshot -- swept by
`crefactor/sweep` and printed canonically -- before xform was deleted
(doc/GRAPH-MIGRATION.md, *Fin as built*). Each row: the snapshot, the
SHA-256 of its text (the input this row is of), the SHA-256 of the text
closure's result, and that result's length in bytes.

Written once at 1c227af with the snapshots of slim-vim.c at that commit's
`src/upstream.sha`: for each n, `xform.FallOut()(qNNN.c, nil, io.Discard)`,
`sweep.Prune` with `whim.Profile.Sweep`, `cemit.Canonical` -- the test's
own steps there. A snapshot of another input has no row, and the test says
so and skips it.

```
40 42b496386c7e296603ae7a69b4fd0179976e071176fcaf0ab0f64263bcd8132c 059ae74aa192123f83bbdd6ff9ed8e3558066da9c77cbc857b2ad2c62e904a5b 2055247
70 33c194bac4db011aae69ce068ac71ef7304345af0c543a159aa768846f172370 a6aecf62a4f1156e5cae5aa4556a233fe02955caa7a6e1b86e158cdfc20980df 1988621
103 e092b3aa87e3f0a9c840fbaedf5495540aeaa4fa3937542d6395b559c63a0334 bb38cb56215d8c2cc38445607f3084b86a3618878ed673cd8bf463f4b4b6539b 2070648
```
