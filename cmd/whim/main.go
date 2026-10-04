// Command whim is the toolset: every tool a subcommand, run as
// `go tool whim <subcommand>` (go.mod declares this package a tool, so Go
// builds and caches it).  README.md beside it lists each tool, and what each
// retired script became.
//
// The pipeline itself is `whim build` (internal/build's plan on
// crefactor/pipeline's driver, every phase on crefactor/graph's graph); the
// other subcommands are the generators of the translations, the suite
// (`whim test`), the graph's and C-lisp's tools, the analyses that report
// and cut nothing, and the cutters and phase programs one at a time on a
// file (internal/steps' OnText: the file imported, its C view written back).
package main

import (
	"fmt"
	"os"
)

// A tool is one subcommand.  It is handed everything after the subcommand
// name and returns the process exit status.
type tool struct {
	run   func(args []string) int
	usage string
}

// order is the subcommands as they are listed, fixed rather than taken from
// the map: ranging a Go map yields a different order every run, which is the
// Go-shaped version of the determinism trap the Python tools avoid by sorting
// before they report.
var order = []string{
	"sweep",
	"funcreach",
	"score", "cmdnames", "droplocal", "nointro", "noglob", "noequiclass", "nowild", "nostat", "nofnamemod", "notags", "nofind", "noterm", "noshellout", "noruntime", "noabbr", "nostartup", "nohome", "noinert", "noarglist", "noinertopts", "nofencs", "nocmdopts", "nobuflist", "nofloat", "keepbytes", "oneoptset", "optreaders", "noowner", "nogetenv", "nochdir", "nosignals", "noswap", "norecover", "noucmd", "nonfa", "nolocale", "nowinsizes", "nocompl", "nofenc", "noident", "nobackup", "nosession", "onebuffer", "nocindent", "nowildmenu", "nomouse", "notabs", "nomemfile", "nocomplkeys", "lfonly", "nowindows", "noconv", "noenc", "utf8only", "fold", "edit", "query",
	"build", "cemit", "c2lisp", "lisp2c", "graph", "view",
	"parse", "fieldref", "reach", "measure", "cdiff", "test", "cut", "gen", "skel", "java", "clj", "caprice", "whimsy", "whimsical", "whiml", "wpp", "guest", "pre", "gocat", "hscat",
}

var tools = map[string]tool{
	"funcreach":   {runFuncreach, "funcreach <file> [--delete]"},
	"sweep":       {runSweep, "sweep [--root NAME]... [--freeze NAME]... <file.c>"},
	"score":       {runScore, "score"},
	"cmdnames":    {runCmdnames, "cmdnames <file>"},
	"droplocal":   {fileStep("droplocal"), "droplocal <file> <field>..."},
	"nointro":     {fileStep("nointro"), "nointro <file>"},
	"noglob":      {fileStep("noglob"), "noglob <file>"},
	"noequiclass": {fileStep("noequiclass"), "noequiclass <file>"},
	"nowild":      {fileStep("nowild"), "nowild <file>"},
	"nostat":      {fileStep("nostat"), "nostat <file>"},
	"nofnamemod":  {fileStep("nofnamemod"), "nofnamemod <file>"},
	"notags":      {fileStep("notags"), "notags <file>"},
	"nofind":      {fileStep("nofind"), "nofind <file>"},
	"noterm":      {fileStep("noterm"), "noterm <file>"},
	"noshellout":  {fileStep("noshellout"), "noshellout <file>"},
	"edit":        {runEdit, "edit <phase> <file>"},
	"query":       {runQuery, "query <phase> <file>"},
	"noruntime":   {fileStep("noruntime"), "noruntime <file>"},
	"noabbr":      {fileStep("noabbr"), "noabbr <file>"},
	"nostartup":   {fileStep("nostartup"), "nostartup <file>"},
	"nohome":      {fileStep("nohome"), "nohome <file>"},
	"noinert":     {fileStep("noinert"), "noinert <file>"},
	"noarglist":   {fileStep("noarglist"), "noarglist <file>"},
	"noinertopts": {fileStep("noinertopts"), "noinertopts <file>"},
	"nofencs":     {fileStep("nofencs"), "nofencs <file>"},
	"nocmdopts":   {fileStep("nocmdopts"), "nocmdopts <file>"},
	"nobuflist":   {fileStep("nobuflist"), "nobuflist <file>"},
	"nofloat":     {fileStep("nofloat"), "nofloat <file>"},
	"keepbytes":   {fileStep("keepbytes"), "keepbytes <file>"},
	"oneoptset":   {fileStep("oneoptset"), "oneoptset <file>"},
	"optreaders":  {fileStep("optreaders"), "optreaders <file>"},
	"noowner":     {fileStep("noowner"), "noowner <file>"},
	"nogetenv":    {fileStep("nogetenv"), "nogetenv <file>"},
	"nochdir":     {fileStep("nochdir"), "nochdir <file>"},
	"nosignals":   {fileStep("nosignals"), "nosignals <file>"},
	"noswap":      {fileStep("noswap"), "noswap <file>"},
	"norecover":   {fileStep("norecover"), "norecover <file>"},
	"noucmd":      {fileStep("noucmd"), "noucmd <file>"},
	"nonfa":       {fileStep("nonfa"), "nonfa <file>"},
	"nolocale":    {fileStep("nolocale"), "nolocale <file>"},
	"nowinsizes":  {fileStep("nowinsizes"), "nowinsizes <file>"},
	"nocompl":     {fileStep("nocompl"), "nocompl <file>"},
	"nofenc":      {fileStep("nofenc"), "nofenc <file>"},
	"noident":     {fileStep("noident"), "noident <file>"},
	"nobackup":    {fileStep("nobackup"), "nobackup <file>"},
	"nosession":   {fileStep("nosession"), "nosession <file>"},
	"onebuffer":   {fileStep("onebuffer"), "onebuffer <file>"},
	"nocindent":   {fileStep("nocindent"), "nocindent <file>"},
	"nowildmenu":  {fileStep("nowildmenu"), "nowildmenu <file>"},
	"nomouse":     {fileStep("nomouse"), "nomouse <file>"},
	"notabs":      {fileStep("notabs"), "notabs <file>"},
	"nomemfile":   {fileStep("nomemfile"), "nomemfile <file>"},
	"nocomplkeys": {fileStep("nocomplkeys"), "nocomplkeys <file>"},
	"lfonly":      {fileStep("lfonly"), "lfonly <file>"},
	"nowindows":   {fileStep("nowindows"), "nowindows <file>"},
	"noconv":      {fileStep("noconv"), "noconv <file>"},
	"noenc":       {fileStep("noenc"), "noenc <file>"},
	"utf8only":    {fileStep("utf8only"), "utf8only <file>"},
	"fold":        {runFold, "fold <always|never|dropif> <file> <pattern> <count>"},
	"cemit":       {runCemit, "cemit <file.c> [--check]"},
	"c2lisp":      {runC2lisp, "c2lisp [-o OUT] [FILE] | c2lisp --check FILE..."},
	"lisp2c":      {runLisp2c, "lisp2c [-o OUT] [FILE.lc] | lisp2c --check FILE.lc FILE.c"},
	"graph":       {runGraph, "graph [-o OUT] FILE | graph --check FILE... | graph --collect [-o OUT] FILE"},
	"view":        {runView, "view [--ids] [--depth N] [--show node|stmt|fn|none] [--stop HEADS] [--c] [--no-cache] [--time] callers F | callees F | uses NAME | member S.M | type T | def NAME | follow STEPS ROOT [FILE]"},
	"build":       {runBuild, "build [--check] [-v] [--canonical] [--keep-going] [--from N] [--to N] [--src F] [--out F] [--work D] [--keep D]"},
	"parse":       {runParse, "parse <file.c>"},
	"cdiff":       {runCdiff, "cdiff [-n N] A.c B.c"},
	"fieldref":    {runFieldRef, "fieldref <file.c>"},
	"cut":         {runCut, "cut [FILE]"},
	"gen":         {runGen, "gen [--check] [FILE]"},
	"skel":        {runSkel, "skel <editor.c> <outdir> [-bodies | -editor <editor.go> | -java <Class.java> | -clj <editor.clj> | -hs <Editor.hs> | -rs <editor.rs> | -lowerc <lowered.c>]"},
	"java":        {runJava, "java [--out DIR] [FILE]"},
	"clj":         {runClj, "clj [--out DIR] [--editor editor.clj] [--jar FILE] [FILE]"},
	"caprice":     {runCaprice, "caprice [--out DIR] [FILE]"},
	"whimsy":      {runWhimsy, "whimsy [--out DIR] [FILE]"},
	"whimsical":   {runWhimsical, "whimsical [--debug] [--out DIR] [FILE]"},
	"whiml":       {runWhiml, "whiml [--out DIR] [FILE]"},
	"wpp":         {runWpp, "wpp [--out DIR] [FILE]"},
	"guest":       {runGuest, "guest [--arch amd64|arm64] [--hello|--bench] [--alt] [--image] [-o OUT] [FILE]"},
	"pre":         {runPre, "pre casts|order|unions|garrays|voids|gotos|funcs <editor.c>"},
	"gocat":       {runGocat, "gocat <dir>"},
	"hscat":       {runHscat, "hscat [--ghc GHC] [--hsl FILE] [--out DIR] [--no-test] [SRC]"},
	"test":        {runTest, "test [--wide] [--java] [--clojure] [--clojure-editor editor.clj] [--haskell] [--haskell-bin PROGRAM] [--rust] [--scheme] [--scheme-debug] [--ocaml] [--cpp] [--limit DURATION] [--ref REV] [FILE]"},
	"measure":     {runMeasure, "measure <dir of qNNN.c>"},
	"reach":       {runReach, "reach <file.c> [--no-control]"},
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	t, ok := tools[os.Args[1]]
	if !ok {
		usage()
	}
	os.Exit(scoped(func() int { return t.run(os.Args[2:]) }))
}

// scoped runs a subcommand with a TMPDIR of its own and removes it after.
//
// Every temporary a subcommand makes goes through os.MkdirTemp("", ...) or
// through a child that reads TMPDIR -- gcc's cc*.s among them -- and about
// twenty sites never removed theirs: one `make whim-verify` left 1,060
// harness-bin-* directories behind, plus pty homes and gcc temporaries.  A
// private directory removed on the way out catches all of them, including
// sites not written yet, where patching each one would catch only those in
// front of the reader.  Nothing a subcommand leaves for its caller lives
// under TMPDIR: outputs go to paths the caller names.
func scoped(run func() int) int {
	dir, err := os.MkdirTemp("", "whim-")
	if err != nil {
		return run()
	}
	prev, had := os.LookupEnv("TMPDIR")
	os.Setenv("TMPDIR", dir)
	code := run()
	if had {
		os.Setenv("TMPDIR", prev)
	} else {
		os.Unsetenv("TMPDIR")
	}
	os.RemoveAll(dir)
	return code
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: whim <subcommand> [args]")
	for _, name := range order {
		fmt.Fprintf(os.Stderr, "    %s\n", tools[name].usage)
	}
	os.Exit(2)
}
