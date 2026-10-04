package cut

import (
	"bytes"
	"fmt"
	"io"

	"github.com/arbace/go-whim/crefactor/graph"
)

// NoTags removes every way to ask for a tag.
//
// The seven places a tag could still be asked for, each with the words its
// refusal uses.  Every one is COUNTED: a tag edit that matched twice would
// take a second construct with the same shape somewhere else in the file,
// and one that matched none has had its anchor moved under it.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B4): `if (!checkclearopq(...)) {X}`
// is rewritten `(void)checkclearopq(...);` by template, the ifs and the case
// runs cut by form, the table's row deleted (INITROW); the mentions left
// are the text's count on the C view (history keeps the text version).
func NoTags(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("notags", e, w)
	const keep = "(cast void (call checkclearopq (-> cap oap)))"
	v.Rewrite("(if (! (call checkclearopq (-> cap oap))) (block (call ex_help nullptr)))", keep, 1,
		"the <Help> key, which reached do_tag through ex_help")
	v.Rewrite(`(if (! (call checkclearopq (-> cap oap))) (block (call do_tag (cast (ptr char_u) "") DT_POP _ _ _)))`, keep, 1,
		"CTRL-T, the tag stack pop")
	v.Cut("(if (|| (== (-> xp xp_context) EXPAND_TAGS) (== (-> xp xp_context) EXPAND_TAGS_LISTFILES)) (block (return (call expand_tags _ _ _ _))))", 1,
		"tag completion on the command line")
	v.Cut("(if (== (-> xp xp_context) EXPAND_HELP) (block (if (== (call find_help_tags _ _ _ _) OK) (block (return OK))) (return FAIL)))", 1,
		"help-tag completion on the command line")
	v.InFunction("set_context_by_cmdname", func(v *graph.Verbs) {
		var run []string
		for _, c := range []string{"tag", "stag", "ptag", "ltag", "tselect", "stselect", "ptselect", "tjump", "stjump", "ptjump"} {
			run = append(run, "(case CMD_"+c+")")
		}
		run = append(run, "(if (!= (call vim_strchr p_wop WOP_TAGFILE) nullptr) (block _) (block _))",
			"(= (-> xp xp_pattern) arg)", "(break)")
		v.CutRun("the ten command cases that asked for a tag context", run...)
	})
	v.CutRun("CTRL-X CTRL-] tag completion in insert mode",
		"(case (paren (+ 5 CTRL_X_WANT_IDENT)))", "(call get_next_tag_completion)", "(break)")
	v.InTable("command_complete_tab", func(v *graph.Verbs) {
		deleteRowTyped(v, "(init (paren EXPAND_TAGS) (init (paren (cast (ptr char_u) \"tag\")) _))",
			"-complete=tag as a name :command accepts")
	})
	if v.Failed() {
		return v.Done()
	}
	text := v.Text()
	fmt.Fprintf(w, "  notags       %d do_tag mentions and %d find_tags mentions left "+
		"for the sweep\n",
		bytes.Count(text, []byte("do_tag")), bytes.Count(text, []byte("find_tags")))
	return nil
}
