# FRAG on the snapshots: what the text verbs gave

`TestFragOnSnapshots` holds each splice on q(N-1)'s graph to the text
program's splice printed canonically. Four of its cases wrote that splice
with crefactor/edit's text verb set (`edit.E`: `Body`, `InFunction`,
`Literal`, `Sub`), deleted after 864655e, the last commit that has it; their
results are recorded here, since whole files of 2 MB each are no test data.
Each row: the case, the snapshot it runs on, the SHA-256 of that snapshot's
text (the input this row is of), the SHA-256 of the text verbs' result
printed canonically, and its length in bytes.

Written once at 864655e with the snapshots of slim-vim.c at that commit's
`src/upstream.sha`: each case's `text` there -- the acts its comment in
`frag_test.go` lists, run by `edit.New` on qNNN.c and `Done` -- put through
`cemit.Canonical`, the test's own steps. A snapshot of another input has no
row, and the test says so and skips the case.

```
body/68 67 ec64071bfb1dc6b695a6daba84f81f912c8541f5ea21b7956eab0cb2c9f2b776 12fe4843182da1fc787e93f2e69d8cc4a4a6ea97731e3c96c268fad6b83040f6 2007391
body/61 60 b9b3ce116a4f7124245f4efa14231401b4aec2b154c09ccb63241d0085c4c56b 0426564284a57da47078582b93d45fddcc943e6c2b3ec7ad9af0079a303f5ac2 2017253
run/22 21 81dfc7464038d0d092d415d5b303512546efa80a564f6a2ccbdb98697d2f950e 951774ca230a8997c5baafed744004f45be189bdb97170645f18debc71b00d5b 2181531
together/21 20 5f5dca5912d919dcfd0784cfcf6217e6ffaeecc2d12b677b638b67cd253b6cfc c04bc933b9afb176678d1cf46f4df33ede7c4cc10b4ffc98760067dc1c1162a2 2185954
```
