package p144

// Whim phase 144 -- edit() has no goto.  See GOAL.md.
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
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/edit"
)

func init() { edit.Register("whim144", Edit) }

// the blocks the labels name, as the input has them
const (
	w144Intr = `            if (goto_im())
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
	w144Esc = `            if (ins_at_eol && gchar_cursor() == NUL)
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
	w144Normal = `            ins_try_si(c);
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

// W144Helpers are the two functions the labelled blocks become, exported so the
// check requires the identical text: the blocks with the locals they wrote
// passed by pointer, and edit_esc() saying whether Insert mode ends instead of
// returning from edit() itself.
const W144Helpers = `    static int
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

// W144EscCall is what a jump to doESCkey becomes at an indentation.
func W144EscCall(ind string) string {
	return ind + "if (edit_esc(&count, cmdchar, nomove, &o_lnum))\n" +
		ind + "{\n" + ind + "    return (c == Ctrl_O);\n" + ind + "}\n" + ind + "continue;\n"
}

// W144NormalCall is what a jump to normalchar becomes at an indentation.
func W144NormalCall(ind string) string {
	return ind + "edit_normalchar(c, &inserted_space);\n" + ind + "break;\n"
}

// W144Enclosing names the blocks around pos, outermost first: the header of
// each -- the line holding its `{`, or the line above when the brace is alone.
func W144Enclosing(s string, pos int) []string {
	b := cutil.Blank([]byte(s))
	var stack []string
	lines := strings.Split(s[:pos], "\n")
	off := 0
	for li, l := range lines {
		bl := string(b[off : off+len(l)])
		for _, ch := range bl {
			switch ch {
			case '{':
				h := strings.TrimSpace(l)
				if h == "{" {
					for k := li - 1; k >= 0; k-- {
						if t := strings.TrimSpace(lines[k]); t != "" {
							h = t
							break
						}
					}
				}
				stack = append(stack, h)
			case '}':
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			}
		}
		off += len(l) + 1
	}
	return stack
}

var w144Loop = regexp.MustCompile(`^(for \(|while \(|do$)`)

// W144Inner is the innermost enclosing loop, and the innermost loop or switch.
func W144Inner(encl []string) (loop, breaks string) {
	for k := len(encl) - 1; k >= 0; k-- {
		h := encl[k]
		if w144Loop.MatchString(h) {
			if loop == "" {
				loop = h
			}
			if breaks == "" {
				breaks = h
			}
		}
		if strings.HasPrefix(h, "switch (") && breaks == "" {
			breaks = h
		}
	}
	return loop, breaks
}

// Whim144 takes the jumps Out of edit().
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
//     jump inside a do-while breaks Out of it with esc_now set, tested just
//     after the loop;
//   - normalchar's block becomes edit_normalchar(); each jump is a call and a
//     `break`, checked to leave the main switch;
//   - do_intr's jump is the Esc case's goto_im() test written Out, then the
//     edit_esc() call.
func Edit(text []byte, w io.Writer) ([]byte, error) {
	p := edit.Ph{Tag: "editgoto", W: w}
	blank := cutil.Blank(text)
	a, z, ok := cutil.FindDefinition(text, blank, "edit")
	if !ok {
		return nil, p.Die("edit is not defined")
	}
	s := string(text[a:z])
	for _, need := range []string{"        do_intr:\n" + w144Intr + "        doESCkey:\n" + w144Esc, "        normalchar:\n" + w144Normal} {
		if strings.Count(s, need) != 1 {
			return nil, p.Die("a labelled block is not the one this phase was written against")
		}
	}
	const main, sw = "for (;;)", "switch (c)"
	// every jump, from the last so the offsets hold
	jr := regexp.MustCompile(`(?m)^( *)goto (doESCkey|normalchar|do_intr);\n`)
	ms := jr.FindAllStringSubmatchIndex(s, -1)
	count := map[string]int{}
	doSite := -1
	for k := len(ms) - 1; k >= 0; k-- {
		m := ms[k]
		ind, label := s[m[2]:m[3]], s[m[4]:m[5]]
		loop, brk := W144Inner(W144Enclosing(s, m[0]))
		var repl string
		switch label {
		case "doESCkey":
			switch {
			case loop == main:
				repl = W144EscCall(ind)
			case loop == "do" && brk == "do" && doSite < 0:
				doSite = m[0]
				repl = ind + "esc_now = TRUE;\n" + ind + "break;\n"
			default:
				return nil, p.Die("a jump to doESCkey is inside %q, where continue would not mean the next key", loop)
			}
		case "normalchar":
			if brk != sw || loop != main {
				return nil, p.Die("a jump to normalchar is inside %q, where break would not leave the switch", brk)
			}
			repl = W144NormalCall(ind)
		case "do_intr":
			if brk != sw || loop != main {
				return nil, p.Die("the jump to do_intr is inside %q", brk)
			}
			var b strings.Builder
			for _, l := range strings.SplitAfter(w144Intr, "\n") {
				if l != "" {
					b.WriteString(ind[:len(ind)-12] + l)
				}
			}
			repl = b.String() + W144EscCall(ind)
		}
		count[label]++
		s = s[:m[0]] + repl + s[m[1]:]
	}
	if count["doESCkey"] != 9 || count["normalchar"] != 7 || count["do_intr"] != 1 || doSite < 0 {
		return nil, p.Die("the jumps are %v, and this phase was written against 9, 7 and 1, one inside a do-while", count)
	}
	// the do-while's flag, tested just after it
	// The canonical text writes a do-while's end as `}` on its own line and
	// `while (...);` on the next (the sweep's old canonicalisers put the `;` on a
	// third), and the indentation the insertion takes is the brace's.
	dw := regexp.MustCompile(`(?m)^( *)\}\n *while \([^\n]*\)(?:\n *)?;\n`)
	wm := dw.FindStringSubmatchIndex(s[doSite:])
	if wm == nil {
		return nil, p.Die("the do-while around the jump has no end")
	}
	at := doSite + wm[1]
	ind := s[doSite+wm[2] : doSite+wm[3]]
	s = s[:at] + ind + "if (esc_now)\n" + ind + "{\n" + ind + "    esc_now = FALSE;\n" +
		strings.ReplaceAll(W144EscCall(ind+"    "), "\n", "\n") + ind + "}\n" + s[at:]
	// the labelled blocks themselves
	s = strings.Replace(s, "        do_intr:\n"+w144Intr+"        doESCkey:\n"+w144Esc, w144Intr+W144EscCall("            "), 1)
	s = strings.Replace(s, "        normalchar:\n"+w144Normal, W144NormalCall("            "), 1)
	decl := "    int c = 0;\n"
	if strings.Count(s, decl) != 1 {
		return nil, p.Die("edit()'s c is not declared where this phase expects")
	}
	s = strings.Replace(s, decl, decl+"    int esc_now = FALSE;\n", 1)
	if strings.Contains(s, "goto ") || regexp.MustCompile(`(?m)^[a-zA-Z_]+:$`).MatchString(s) {
		return nil, p.Die("edit() still jumps or has a label")
	}
	p.Say(fmt.Sprintf("edit() jumps nowhere: %d jumps to doESCkey call edit_esc(), one of them after leaving its do-while, %d to normalchar call edit_normalchar(), and do_intr's is its block written out", count["doESCkey"], count["normalchar"]))
	return []byte(string(text[:a]) + W144Helpers + s + string(text[z:])), nil
}
