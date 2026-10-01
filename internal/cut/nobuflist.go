package cut

import (
	"fmt"
	"io"
	"regexp"
)

var (
	ecmdAddAlt = regexp.MustCompile(`\bECMD_(ADDBUF|ALTBUF)\b`)
)

// NoBufList leaves only :bnext and :bprevious walking the buffer list.
func NoBufList(text []byte, w io.Writer) ([]byte, error) {
	e := ed{"nobuflist", w}
	var err error

	text, err = e.inFunction(text, "ex_edit", func(s []byte) ([]byte, error) {
		return e.literal(s, "eap->cmdidx != CMD_badd && eap->cmdidx != CMD_balt && ", "",
			":edit asking whether it was :badd or :balt", 1)
	})
	if err != nil {
		return nil, err
	}

	text, err = e.inFunction(text, "do_exedit", func(s []byte) ([]byte, error) {
		s, err := e.foldAlways(s,
			`^[ \t]*if \(eap->cmdidx != CMD_balt && eap->cmdidx != CMD_badd\)$`,
			"do_exedit setting the pcmark for :badd and :balt")
		if err != nil {
			return nil, err
		}
		return e.literal(s, " + (eap->cmdidx == CMD_badd ? ECMD_ADDBUF : 0) + "+
			"(eap->cmdidx == CMD_balt ? ECMD_ALTBUF : 0)", "",
			"do_exedit passing ECMD_ADDBUF and ECMD_ALTBUF", 1)
	})
	if err != nil {
		return nil, err
	}

	text, err = e.inFunction(text, "do_ecmd", func(s []byte) ([]byte, error) {
		s, err := e.foldNever(s, `^[ \t]*if \(\(flags & \(ECMD_ADDBUF \| ECMD_ALTBUF\)\) `+
			`&& \(ffname == NULL \|\| \*ffname == NUL\)\)$`,
			"do_ecmd adding a buffer with no name")
		if err != nil {
			return nil, err
		}
		s, err = e.literal(s, "(ECMD_HIDE | ECMD_ADDBUF | ECMD_ALTBUF)", "(ECMD_HIDE)",
			"do_ecmd sparing an added buffer the changed check", 1)
		if err != nil {
			return nil, err
		}
		s, err = e.foldAlways(s, `^[ \t]*if \(!\(flags & \(ECMD_ADDBUF \| ECMD_ALTBUF\)\)\)$`,
			"do_ecmd keeping the alternate file")
		if err != nil {
			return nil, err
		}
		s, err = e.foldNever(s, `^[ \t]*if \(flags & \(ECMD_ADDBUF \| ECMD_ALTBUF\)\)$`,
			"do_ecmd adding a buffer without editing it")
		if err != nil {
			return nil, err
		}
		return e.literal(s, "(flags & (ECMD_ADDBUF | ECMD_ALTBUF)) || ", "",
			"do_ecmd stopping after an added buffer", 1)
	})
	if err != nil {
		return nil, err
	}

	text, err = e.inFunction(text, "do_one_cmd", func(s []byte) ([]byte, error) {
		return e.foldNever(s, `^[ \t]*if \(ea\.cmdidx == CMD_bdelete \|\| `+
			`ea\.cmdidx == CMD_bwipeout \|\| ea\.cmdidx == CMD_bunload\)$`,
			"do_one_cmd reading a buffer list argument")
	})
	if err != nil {
		return nil, err
	}

	// do_buffer_ext's unloading and goto_buffer's :bNext died with the buffer
	// commands, retired at phase 1 (exfront, the reform's D2)

	text, err = e.inFunction(text, "set_context_by_cmdname", func(s []byte) ([]byte, error) {
		s, err := e.subOnce(s, `^[ \t]*case CMD_bufdo:\n`, "completion for :bufdo")
		if err != nil {
			return nil, err
		}
		s, err = e.subOnce(s, `^[ \t]*case CMD_bdelete:\n[ \t]*case CMD_bwipeout:\n`+
			`[ \t]*case CMD_bunload:\n`+
			`[ \t]*while \(\(xp->xp_pattern = vim_strchr\(arg, ' '\)\) != NULL\)\n`+
			`[ \t]*\{\n[ \t]*arg = xp->xp_pattern \+ 1;\n[ \t]*\}\n`+
			`[ \t]*__attribute__\(\(fallthrough\)\);\n`,
			"completion for :bdelete, :bwipeout and :bunload")
		if err != nil {
			return nil, err
		}
		return e.subOnce(s, `^[ \t]*case CMD_buffer:\n`, "completion for :buffer")
	})
	if err != nil {
		return nil, err
	}

	// Only the flags can be counted here.  ex_listdo() and ex_bunload() still
	// name CMD_bufdo and CMD_bdelete, and have no row now: the sweep takes
	// them, and whim41 counts the command names after it.
	if n := len(ecmdAddAlt.FindAll(text, -1)); n != 2 {
		return nil, fmt.Errorf("nobuflist: ECMD_ADDBUF or ECMD_ALTBUF outside their "+
			"enumerators -- %d mentions, expected 2", n)
	}

	e.say("only :bnext and :bprevious walk the buffer list")
	return text, nil
}
