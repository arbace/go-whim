package p091

// The multi-line literals internal/phase/091/edit.go matches on, EXTRACTED from the phase's
// heredoc by tools/gocmp/genlits.py rather than retyped.  They carry BLANK
// LINES, which a filtered read of a phase program does not show, and a
// literal that is 90%% right matches nothing.  See that tool for the
// measurement.
const (
	w91Head = "    if (cap->nchar == 'f')\n    {\n        nv_gotofile(cap);\n    }\n    else\n    {\n"
	w91lit1 = "    case 'f':\n    case 'F':\n        nv_gotofile(cap);\n        break;\n"
	w91lit2 = "    if (ea.argt & EX_ARGOPT)\n    {\n        while (ea.arg[0] == '+' && ea.arg[1] == '+')\n        {\n            if (getargopt(&ea) == FAIL)\n            {\n                errormsg = _(e_invalid_argument);\n                goto doend;\n            }\n        }\n    }\n"
	w91lit4 = "    }\n"
)
