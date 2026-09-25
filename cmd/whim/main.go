// Command whim is every tool the pipeline run.
//
// It is one binary with subcommands rather than one binary per tool, because
// every subcommand works on the same multi-megabyte file and the sweep runs
// thirteen of them to a fixpoint: as separate processes they re-read and
// re-scan that file thirteen times a round, which is the cost this rewrite is
// meant to remove.  Phase programs still name a distinct tools/go path per
// subcommand so that tools/implhash.sh keeps its per-tool invalidation.
//
// The subcommands are drop-in replacements: same argv, same rewrite-in-place,
// same stdout, same exit codes as the Python they stand in for.  tools/sweep.sh
// detects that a tool did something by taking sha256 of the file and by
// nothing else, so agreement means BYTE agreement and each one is held to it
// against real inputs.  (That comparison lived in arbace/slim-vim, beside the
// Python it compared against; this repository carries no Python at all.)
//
// The C front end (modernc.org/cc/v4, pinned and patched) is deliberately not
// used by any sweep subcommand.  Those run on text that six deleting tools
// have already cut and that nothing has compiled since, so it need not be
// valid C.  Parsing belongs to the phase edit programs, whose input is a
// boundary that compiled.
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
	"score", "cmdnames", "cmdidxs", "dropoptions", "retire", "droplocal", "nointro", "noargv0", "noglob", "noequiclass", "nowild", "nostat", "nofnamemod", "notags", "nofind", "noterm", "noshellout", "noruntime", "noabbr", "dropopts", "nostartup", "nohome", "nocmdargs", "noinert", "noarglist", "noinertopts", "nofencs", "nocmdopts", "nobuflist", "nofloat", "keepbytes", "oneoptset", "optreaders", "noowner", "nogetenv", "nochdir", "nosignals", "noswap", "norecover", "noucmd", "nonfa", "nolocale", "nowinsizes", "nocompl", "nofenc", "noident", "nobackup", "nosession", "onebuffer", "nocindent", "nowildmenu", "nomouse", "notabs", "nomemfile", "nocomplkeys", "lfonly", "nowindows", "noconv", "noenc", "utf8only", "fold", "edit", "query",
	"build", "cemit",
	"parse", "fieldref", "reach", "measure", "test", "gen", "skel", "java", "clj", "pre", "gocat",
}

var tools = map[string]tool{
	"funcreach":   {runFuncreach, "funcreach <file> [--delete]"},
	"sweep":       {runSweep, "sweep [--root NAME]... [--freeze NAME]... <file.c>"},
	"score":       {runScore, "score"},
	"cmdnames":    {runCmdnames, "cmdnames <file>"},
	"cmdidxs":     {runCmdidxs, "cmdidxs <file> [--check|--update]"},
	"dropoptions": {fileStep("dropoptions"), "dropoptions <file> <option-name>..."},
	"retire":      {fileStep("retire"), "retire <file> <command>..."},
	"droplocal":   {fileStep("droplocal"), "droplocal <file> <field>..."},
	"nointro":     {fileStep("nointro"), "nointro <file>"},
	"noargv0":     {fileStep("noargv0"), "noargv0 <file>"},
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
	"dropopts":    {fileStep("dropopts"), "dropopts <file> <-x|--long>..."},
	"nostartup":   {fileStep("nostartup"), "nostartup <file>"},
	"nohome":      {fileStep("nohome"), "nohome <file>"},
	"nocmdargs":   {fileStep("nocmdargs"), "nocmdargs <file>"},
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
	"build":       {runBuild, "build [--check] [-v] [--canonical] [--keep-going] [--from N] [--to N] [--src F] [--out F] [--work D] [--keep D]"},
	"parse":       {runParse, "parse <file.c>"},
	"fieldref":    {runFieldRef, "fieldref <file.c>"},
	"gen":         {runGen, "gen [--check]"},
	"skel":        {runSkel, "skel <editor.c> <outdir> [-bodies | -editor <editor.go> | -java <Class.java> | -clj <editor.clj> | -lowerc <lowered.c>]"},
	"java":        {runJava, "java [--out DIR] [FILE]"},
	"clj":         {runClj, "clj [--out DIR] [--editor editor.clj] [--jar FILE] [FILE]"},
	"pre":         {runPre, "pre casts|order|unions|garrays|voids|gotos|funcs <editor.c>"},
	"gocat":       {runGocat, "gocat <dir>"},
	"test":        {runTest, "test [--wide] [--java] [--clojure] [--clojure-editor editor.clj] [--ref REV] [FILE]"},
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
