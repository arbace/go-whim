package cut

import (
	"bytes"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// NoStartup stops the editor reading anything at startup.
//
// source_startup_scripts() keeps its name and loses its body ENTIRELY.  On
// the graph (doc/GRAPH-MIGRATION.md, B4) the body is emptied (Body) and the
// two calls are cut as items, counted (history keeps the text version).
func NoStartup(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("nostartup", e, w)
	if e.Defn("source_startup_scripts") == nil {
		v.Die("source_startup_scripts is not defined at file scope")
		return v.Done()
	}
	was := 0
	v.InFunction("source_startup_scripts", func(v *graph.Verbs) { was = bodyLines(v.Text()) })
	v.Muted(func(v *graph.Verbs) { v.Body("source_startup_scripts", "", "source_startup_scripts") })
	v.Sayf("source_startup_scripts was %d lines; it now has no body", was)

	v.Cut("(call source_startup_scripts (addr params))", 1,
		"startup reads nothing, so it does not call the function that read")

	// -u NONE's test in main() reads what nothing writes once the command
	// line is cut: the fall-out closure took it (argvfront, the reform's D1)

	v.Cut("(call set_init_xdg_rtp)", 1, "'runtimepath' stops being rebuilt from $XDG_CONFIG_HOME")
	if v.Failed() {
		return v.Done()
	}
	v.Sayf("%d process_env mentions left for the sweep", bytes.Count(v.Text(), []byte("process_env")))
	return v.Done()
}
