package cut

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/crefactor/graph"
)

// noconvPointers are the ten mb_* indirections and the UTF-8 function each
// becomes.
//
// A SLICE AND NOT A MAP.  Python 3.7+ dicts iterate in insertion order and Go
// maps iterate randomly; nothing in the rewriting depends on the order, since
// the ten names are disjoint, but the refusal names the first one that fails
// and that should be the same name on both sides.
var noconvPointers = []struct{ ptr, fn string }{
	{"mb_ptr2len", "utfc_ptr2len"}, {"mb_ptr2len_len", "utfc_ptr2len_len"},
	{"mb_char2len", "utf_char2len"}, {"mb_char2bytes", "utf_char2bytes"},
	{"mb_ptr2cells", "utf_ptr2cells"}, {"mb_ptr2cells_len", "utf_ptr2cells_len"},
	{"mb_char2cells", "utf_char2cells"}, {"mb_off2cells", "utf_off2cells"},
	{"mb_ptr2char", "utf_ptr2char"}, {"mb_head_off", "utf_head_off"},
}

var (
	mbName    = regexp.MustCompile(`^mb_\w+$`)
	utfName   = regexp.MustCompile(`^utf\w+$`)
	latinName = regexp.MustCompile(`^latin_\w+$`)
)

// ncFold folds the ifs of the text's head ONE AT A TIME, FROM THE LAST (a
// condition can repeat inside its own block, and folding the first would
// then fold an inner copy out from under the outer one): HeadFold, a cond
// written `else C` the arms `else if (C)`.  count < 0: as many as there are.
func ncFold(v *graph.Verbs, cond, what string, always bool, count int) {
	kind := "never"
	if always {
		kind = "always"
	}
	arm := strings.HasPrefix(cond, "else ")
	v.HeadFold(kind, arm, strings.TrimPrefix(cond, "else "), count, what)
}

// NoConv makes a file read and written as the UTF-8 bytes it holds.
//
// ON THE GRAPH (doc/GRAPH-MIGRATION.md, B3a): the text version's line
// regexps, brace matching and literals are acts on the nodes -- the folds
// one at a time from the last, as the text folded them, statements and
// runs cut, operands dropped, the startup call made a loop (FRAG) -- and the
// ten mb_* pointers' calls are calls of the UTF-8 functions, each callee
// replaced by a use of the function, counted by edge, before the pointers
// go.  The report is the text's (history keeps the text version).
func NoConv(e *graph.Editor, w io.Writer) error {
	v := graph.NewVerbs("noconv", e, w)

	v.InFunction("getargopt", func(v *graph.Verbs) {
		v.DropIf(`(== (call strncmp (cast (ptr char) (paren arg)) (cast (ptr char) (paren "enc")) (paren 3)) 0)`, 1,
			"++enc and ++encoding")
		v.SpliceFirst("(if (|| (== pp nullptr) (!= (deref arg) '=')) _*)", "(return OK)", "(return FAIL)",
			"getargopt: every ++ argument but ++edit is unknown")
	})

	// readfile went at phase 1 (readfront, phase 31's move)

	v.InFunction("buf_write", func(v *graph.Verbs) {
		ncFold(v, "(&& (!= eap nullptr) (!= (-> eap force_enc) 0))", "buf_write honouring ++enc", false, -1)
		v.Cut(`(= fenc (cast (ptr char_u) ""))`, 1, "buf_write setting the empty encoding")
		v.Cut("(= converted (call need_conversion fenc))", 1, "buf_write asking whether to convert")
		for _, f := range []struct {
			cond, what string
			always     bool
		}{
			{"converted", "buf_write allocating conversion buffers", false},
			{"(&& converted (== wb_flags 0) (== (. write_info bw_iconv_fd) (cast iconv_t (- 1))))",
				"buf_write refusing a conversion it cannot do", false},
			{"(! converted)", "buf_write skipping the conversion check", true},
			{"else notconverted", `the "[NOT converted]" write message`, false},
			{"else converted", `the "[converted]" write message`, false},
		} {
			ncFold(v, f.cond, f.what, f.always, -1)
		}
		v.Cut("(call vim_free fenc_tofree)", 1, "buf_write freeing ++enc's name")
		// Only buf_write_bytes()'s conversion set bw_conv_error, and it has gone.
		ncFold(v, "(. write_info bw_conv_error)", "a conversion error in a write", false, 2)
		v.DropOperand("(! (. write_info bw_conv_error))", 1, "a conversion error keeping the buffer modified")
		// The pass that only checked a conversion is never taken: the loop
		// body clears the flag before its first test.
		ncFold(v, "checking_conversion", "buf_write checking a conversion without writing", false, -1)
		ncFold(v, "(! checking_conversion)", "buf_write syncing after the pass that wrote", true, -1)
		v.Cut("(= (. write_info bw_conv_buf) nullptr)", 1, "buf_write clearing the conversion buffer")
		v.Cut("(call vim_free (. write_info bw_conv_buf))", 1, "buf_write freeing the conversion buffer")
	})

	v.InFunction("buf_write_bytes", func(v *graph.Verbs) {
		v.DropIf("(! (& flags FIO_NOCONVERT))", 1, "buf_write_bytes converting, which no write flag asks for")
	})
	v.InFunction("do_ecmd", func(v *graph.Verbs) {
		v.Cut("(if (&& (! oldbuf) (!= eap nullptr)) (block (call set_forced_fenc eap)))", 1, "editing a file taking ++enc")
	})

	v.InFunction("mb_init", func(v *graph.Verbs) {
		ncFold(v, "(== p_enc nullptr)", "mb_init without an encoding", false, -1)
		ncFold(v, `(!= (call strcmp (cast (ptr char) (paren p_enc)) (cast (ptr char) (paren "utf-8"))) 0)`,
			"mb_init refusing another encoding", false, -1)
		if v.Failed() {
			return
		}
		var stores []*graph.Node
		for _, s := range v.Find("(= ?p ?f)") {
			if v.Editor().Item(s) == s && !s.Kids[1].IsList() && mbName.MatchString(s.Kids[1].Atom) &&
				!s.Kids[2].IsList() && utfName.MatchString(s.Kids[2].Atom) {
				stores = append(stores, s)
			}
		}
		if len(stores) != 10 {
			v.Die("mb_init pointing the mb_* functions at UTF-8 -- expected 10, matched %d", len(stores))
			return
		}
		for _, s := range stores {
			if err := v.Editor().Delete(s); err != nil {
				v.Die("mb_init pointing the mb_* functions at UTF-8 -- %v", err)
				return
			}
		}
		v.Say("mb_init pointing the mb_* functions at UTF-8")
		v.Cut("(= (. vimconv vc_type) CONV_NONE)", 1, "mb_init clearing a conversion")
		v.Cut("(call convert_setup (addr vimconv) nullptr nullptr)", 1, "mb_init setting up no conversion")
	})

	// THE BRANCH FOLDED ABOVE WAS TAKEN, ONCE.  common_init_1() calls
	// mb_init() before any option exists, and p_enc NULL sent that call to the
	// 1s and back; set_init_1() makes the real call later.  Without the branch
	// the first call ran on into init_chartab() with no curbuf, and the editor
	// crashed before its first command.
	v.InFunction("common_init_1", func(v *graph.Verbs) {
		v.ReplaceC("(cast void (call mb_init))", "for (int i = 0; i < 256; ++i)\n{\n    mb_bytelen_tab[i] = 1;\n}\n", 1,
			"startup filling the byte lengths before any option exists")
	})
	v.InFunction("set_options_default", func(v *graph.Verbs) {
		v.DropOperand("(|| (== opt_flags 0) (paren (!= (. (index options i) var) (cast (ptr char_u) (addr p_enc)))))", 1,
			"setting defaults skipping 'encoding'")
	})

	v.InFunction("utf_find_illegal", func(v *graph.Verbs) {
		ncFold(v, "(!= (. vimconv vc_type) CONV_NONE)", ":ga-style search converting a line first", false, -1)
		v.KeepThen("(== (. vimconv vc_type) CONV_NONE)", 1, "the illegal byte found in the line itself")
		v.Cut("(= (. vimconv vc_type) CONV_NONE)", 1, "utf_find_illegal clearing a conversion")
		v.Cut("(call vim_free tofree)", 1, "utf_find_illegal freeing a converted copy")
		v.Cut("(call convert_setup (addr vimconv) nullptr nullptr)", 1, "utf_find_illegal ending no conversion")
		// `theend:` ends the function now: the null statement the text's
		// print puts after it
		if !v.Failed() {
			if _, err := v.Editor().EndLabels(v.Scope()); err != nil {
				v.Die("utf_find_illegal ending no conversion -- %v", err)
			}
		}
	})
	v.InFunction("ui_write", func(v *graph.Verbs) {
		ncFold(v, "(!= (. output_conv vc_type) CONV_NONE)", "output converted for the terminal", false, 2)
	})
	v.InFunction("fill_input_buf", func(v *graph.Verbs) {
		v.Rewrite("(/ ?x (. input_conv vc_factor))", "(paren ?x)", 1, "input read in room for a conversion to grow")
		ncFold(v, "(!= (. input_conv vc_type) CONV_NONE)", "input converted from the terminal", false, -1)
		ncFold(v, "(!= rest nullptr)", "input left over from a conversion", false, -1)
		v.Cut("(= unconverted 0)", 1, "nothing left unconverted")
	})
	if v.Failed() {
		return v.Done()
	}

	// The ten pointers: every call through one is a call of its function,
	// then the pointers go.  A use that is not a call is refused, as the
	// text refused a name left after its calls.
	var defs []*graph.Node
	for _, f := range e.Graph().Forms {
		if f.Is("def") && e.Live(f) {
			if nm := graph.DeclName(f); mbName.MatchString(nm) {
				if val := f.Kids[len(f.Kids)-1]; !val.IsList() && latinName.MatchString(val.Atom) {
					defs = append(defs, f)
				}
			}
		}
	}
	if len(defs) != 10 {
		return fmt.Errorf("noconv: the mb_* function pointers -- expected 10, matched %d", len(defs))
	}
	calls := 0
	for _, p := range noconvPointers {
		var d *graph.Node
		for _, x := range defs {
			if graph.DeclName(x) == p.ptr {
				d = x
			}
		}
		if d == nil {
			return fmt.Errorf("noconv: %s is not one of the pointers", p.ptr)
		}
		for _, u := range e.Uses(d) {
			callee := u
			if up := e.Parent(u); up != nil && up.Is("deref") {
				callee = up
			}
			call := e.Parent(callee)
			if call == nil || !call.Is("call") || call.Kids[1] != callee {
				return fmt.Errorf("noconv: %s is still named after its calls were made direct", p.ptr)
			}
			with, err := e.Build(call, p.fn, nil)
			if err == nil {
				err = e.Replace(callee, with...)
			}
			if err != nil {
				return fmt.Errorf("noconv: %s -- %v", p.ptr, err)
			}
			// the call's type is the function's result, as it was the
			// pointer's: typed again, not left cleared
			e.Rederive(with[0])
			calls++
		}
	}
	for _, d := range defs {
		if err := e.Delete(d); err != nil {
			return fmt.Errorf("noconv: the mb_* function pointers -- %v", err)
		}
	}
	v.Say("the mb_* function pointers")
	v.Sayf("%d calls through an mb_* pointer are direct calls", calls)

	// mb_tail_off went with maketitle(), its one caller, at phase 1
	// (whim2a, the reform's D8)

	// expand_argopt() and get_argopt_name(), the completion of ++ arguments
	// and their values, went with every completion context but files at
	// phase 4 (whim4f, phase 4f's program, which runs before this phase now)

	v.Say("a file is read and written as the UTF-8 bytes it holds")
	return v.Done()
}
