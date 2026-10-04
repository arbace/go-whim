package p071b

// Whim phase 71b (formerly 144) -- edit() has no goto.  See GOAL.md.
//
// Insert mode jumped to doESCkey, normalchar and do_intr from other cases and
// from before its switch (internal/gen/FINDINGS.md, 11).  The two blocks become
// edit_esc() and edit_normalchar(), each jump a call whose continue or break
// is checked to go where the label's went, and do_intr's block is written out.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.

import (
	"io"
	"strings"

	"github.com/arbace/go-whim/crefactor/clisp"
	"github.com/arbace/go-whim/crefactor/graph"
	"github.com/arbace/go-whim/internal/phase"
)

func init() { phase.RegisterGraph("whim71b", Edit) }

// the blocks the labels name, as the input has them
const (
	w71bIntr = `            if (goto_im())
            {
                if (got_int)
                {
                    (void)vgetc();
                    got_int = FALSE;
                }
                else
                {
                    vim_beep(BO_IM);
                }
                break;
            }
`
	w71bEsc = `            if (ins_at_eol && gchar_cursor() == NUL)
            {
                o_lnum = curwin->w_cursor.lnum;
            }
            if (ins_esc(&count, cmdchar, nomove))
            {
                did_cursorhold = FALSE;
                if (!char_avail() && curbuf->b_last_changedtick_i == curbuf->b_changedtick)
                {
                    curbuf->b_last_changedtick = curbuf->b_changedtick;
                }
                return (c == Ctrl_O);
            }
            continue;
`
	w71bNormal = `            ins_try_si(c);
            if (c == ' ')
            {
                inserted_space = TRUE;
            }
            if (vim_iswordc(c) || c != Ctrl_RSB)
            {
                insert_special(c, FALSE, FALSE);
            }
            break;
`
)

// W71bHelpers are the two functions the labelled blocks become, exported so the
// check requires the identical text: the blocks with the locals they wrote
// passed by pointer, and edit_esc() saying whether Insert mode ends instead of
// returning from edit() itself.
const W71bHelpers = `    static int
edit_esc(long *count, int cmdchar, int nomove, linenr_T *o_lnum)
{
    if (ins_at_eol && gchar_cursor() == NUL)
    {
        *o_lnum = curwin->w_cursor.lnum;
    }

    if (ins_esc(count, cmdchar, nomove))
    {
        did_cursorhold = FALSE;

        if (!char_avail() && curbuf->b_last_changedtick_i == curbuf->b_changedtick)
        {
            curbuf->b_last_changedtick = curbuf->b_changedtick;
        }
        return TRUE;
    }
    return FALSE;
}

    static void
edit_normalchar(int c, int *inserted_space)
{
    ins_try_si(c);

    if (c == ' ')
    {
        *inserted_space = TRUE;
    }

    if (vim_iswordc(c) || c != Ctrl_RSB)
    {
        insert_special(c, FALSE, FALSE);
    }
}

`

// W71bEscCall is what a jump to doESCkey becomes at an indentation.
func W71bEscCall(ind string) string {
	return ind + "if (edit_esc(&count, cmdchar, nomove, &o_lnum))\n" +
		ind + "{\n" + ind + "    return (c == Ctrl_O);\n" + ind + "}\n" + ind + "continue;\n"
}

// W71bNormalCall is what a jump to normalchar becomes at an indentation.
func W71bNormalCall(ind string) string {
	return ind + "edit_normalchar(c, &inserted_space);\n" + ind + "break;\n"
}

// W71bEnclosing names the blocks around pos, outermost first: the header of
// each -- the line holding its `{`, or the line above when the brace is alone
// Edit takes the jumps out of edit().
//
// Insert mode's loop jumped to three labels (internal/gen/FINDINGS.md, 11): doESCkey,
// the second half of the Esc case, from nine places, some before the switch;
// normalchar, the default case's insertion, from seven cases; and do_intr, the
// start of the Esc case, from the default case when the key is the interrupt
// character.  Go cannot jump into a case.
//
//   - doESCkey's block becomes edit_esc(), which says whether Insert mode
//     ends; each jump, and the block, is `if (edit_esc(...)) return (c ==
//     Ctrl_O); continue;`.  A `continue` means the next key only where the
//     innermost loop is the main one -- checked at every site -- and the one
//     jump inside a do-while breaks out of it with esc_now set, tested just
//     after the loop;
//   - normalchar's block becomes edit_normalchar(); each jump is a call and a
//     `break`, checked to leave the main switch;
//   - do_intr's jump is the Esc case's goto_im() test written out, then the
//     edit_esc() call.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3f): the labelled runs are held to
// the text's literals (their C, spacing aside); each jump's loop and switch
// are its ancestors on the graph, not a scan of braces; and every piece --
// the helpers before edit(), each jump, the two runs, esc_now and its test
// after the do-while -- is written in ONE FRAG unit; do_intr's label is
// deleted and its block kept.  History keeps the text version.
func Edit(e *graph.Editor, w io.Writer, _ []string) error {
	v := graph.NewVerbs("editgoto", e, w)
	fn := e.Defn("edit")
	if fn == nil {
		v.Die("edit is not defined")
		return v.Done()
	}
	norm := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	cOf := func(ns []*graph.Node) string {
		var ls []*clisp.Node
		for _, n := range ns {
			ls = append(ls, graph.Lisp(n))
		}
		c, err := clisp.PrintItems(ls)
		if err != nil {
			v.Die("%v", err)
		}
		return norm(string(c))
	}
	// run is the n items from the one after lbl
	run := func(lbl *graph.Node, n int) []*graph.Node {
		ks := e.Parent(lbl).Kids
		for i, x := range ks {
			if x == lbl && i+n < len(ks) {
				return ks[i+1 : i+1+n]
			}
		}
		return nil
	}
	v.In(fn, func(v *graph.Verbs) {
		intr := v.One("(label do_intr)", "do_intr")
		esc := v.One("(label doESCkey)", "doESCkey")
		normal := v.One("(label normalchar)", "normalchar")
		if v.Failed() {
			return
		}
		ir, er, nr := run(intr, 2), run(esc, 3), run(normal, 4)
		if len(ir) != 2 || ir[1] != esc || cOf(ir[:1]) != norm(w71bIntr) || cOf(er) != norm(w71bEsc) || cOf(nr) != norm(w71bNormal) {
			v.Die("a labelled block is not the one this phase was written against")
			return
		}
		// the main loop and its switch
		var main *graph.Node
		for _, x := range fn.Kids {
			if x.Is("for") {
				main = x
			}
		}
		sw := e.Parent(e.Parent(normal))
		if main == nil || !sw.Is("switch") || e.Parent(e.Parent(sw)) != main {
			v.Die("edit()'s loop and switch are not where this phase expects them")
			return
		}
		// inner is the innermost loop and the innermost loop or switch around n
		inner := func(n *graph.Node) (loop, brk *graph.Node) {
			for p := e.Parent(n); p != nil && p != fn; p = e.Parent(p) {
				if p.Is("for") || p.Is("while") || p.Is("do") {
					if loop == nil {
						loop = p
					}
					if brk == nil {
						brk = p
					}
				}
				if p.Is("switch") && brk == nil {
					brk = p
				}
			}
			return loop, brk
		}
		var frags []graph.Frag
		count := map[string]int{}
		var doLoop *graph.Node
		for _, g := range v.Find("(goto _)") {
			label := g.Kids[1].Atom
			loop, brk := inner(g)
			var repl string
			switch label {
			case "doESCkey":
				switch {
				case loop == main:
					repl = W71bEscCall("")
				case loop.Is("do") && brk == loop && doLoop == nil:
					doLoop = loop
					repl = "esc_now = TRUE;\nbreak;\n"
				default:
					v.Die("a jump to doESCkey is inside %s, where continue would not mean the next key", loop.Head())
					return
				}
			case "normalchar":
				if brk != sw || loop != main {
					v.Die("a jump to normalchar is inside %s, where break would not leave the switch", brk.Head())
					return
				}
				repl = W71bNormalCall("")
			case "do_intr":
				if brk != sw || loop != main {
					v.Die("the jump to do_intr is inside %s", brk.Head())
					return
				}
				repl = w71bIntr + W71bEscCall("")
			default:
				v.Die("edit() jumps to %s", label)
				return
			}
			count[label]++
			frags = append(frags, graph.Frag{At: e.SpotOf(g), Src: repl})
		}
		if count["doESCkey"] != 9 || count["normalchar"] != 7 || count["do_intr"] != 1 || doLoop == nil {
			v.Die("the jumps are %v, and this phase was written against 9, 7 and 1, one inside a do-while", count)
			return
		}
		// the do-while's flag, tested just after it
		frags = append(frags, graph.Frag{At: e.SpotAfter(doLoop), Src: "if (esc_now)\n{\n    esc_now = FALSE;\n" + W71bEscCall("    ") + "}\n"})
		// the labelled blocks themselves
		frags = append(frags, graph.Frag{At: e.SpotRun(esc, er[len(er)-1]), Src: W71bEscCall("")},
			graph.Frag{At: e.SpotRun(normal, nr[len(nr)-1]), Src: W71bNormalCall("")})
		decl := v.Find("(def c int 0)")
		if len(decl) != 1 {
			v.Die("edit()'s c is not declared where this phase expects")
			return
		}
		frags = append(frags, graph.Frag{At: e.SpotBefore(fn), Src: W71bHelpers},
			graph.Frag{At: e.SpotAfter(decl[0]), Src: "int esc_now = FALSE;\n"})
		if _, err := e.SpliceC(frags...); err != nil {
			v.Die("edit() jumps nowhere -- %v", err)
			return
		}
		if err := e.Delete(intr); err != nil {
			v.Die("%v", err)
			return
		}
		if v.Count("(goto _)") != 0 || v.Count("(label _)") != 0 {
			v.Die("edit() still jumps or has a label")
			return
		}
		v.Sayf("edit() jumps nowhere: %d jumps to doESCkey call edit_esc(), one of them after leaving its do-while, %d to normalchar call edit_normalchar(), and do_intr's is its block written out", count["doESCkey"], count["normalchar"])
	})
	return v.Done()
}
