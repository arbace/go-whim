package p021

// The multi-line C literals whim21 matches on, EXTRACTED from the phase's
// heredoc rather than retyped.
//
// whim3d is why.  An OP_FUNCTION block transcribed by hand had lost the blank
// lines between its statements, and a literal that is 90% right matches
// nothing -- `occurs 0 times, expected 1`.  These carry blank lines too, and a
// filtered read of the phase program does not show them.  Generating is what
// the Go session did for muslctype's 230-line C driver, for the same reason:
// retyping is where a changed byte becomes invisible to the compile AND to
// the comparison.
const (
	w21lit1  = "    return OK;\n"
	w21lit2  = "    return 0;\n"
	w21lit3  = "                case ADDR_ARGUMENTS:\n                    if (((curwin)->w_alist->al_ga.ga_len) == 0)\n                    {\n                        eap->line1 = eap->line2 = 0;\n                    }\n                    else\n                    {\n                        eap->line1 = 1;\n                        eap->line2 = ((curwin)->w_alist->al_ga.ga_len);\n                    }\n                    break;\n"
	w21lit4  = "                case ADDR_ARGUMENTS:\n                    eap->line1 = eap->line2 = 0;\n                    break;\n"
	w21lit5  = "    case ADDR_ARGUMENTS:\n        if (((curwin)->w_alist->al_ga.ga_len) == 0)\n        {\n            eap->line1 = eap->line2 = 0;\n        }\n        else\n        {\n            eap->line2 = ((curwin)->w_alist->al_ga.ga_len);\n        }\n        break;\n"
	w21lit6  = "    case ADDR_ARGUMENTS:\n        eap->line1 = eap->line2 = 0;\n        break;\n"
	w21lit7  = "    case ADDR_ARGUMENTS:\n        lnum = curwin->w_arg_idx + 1;\n        if (lnum > ((curwin)->w_alist->al_ga.ga_len))\n        {\n            lnum = ((curwin)->w_alist->al_ga.ga_len);\n        }\n        break;\n"
	w21lit8  = "    case ADDR_ARGUMENTS:\n        lnum = 0;\n        break;\n"
	w21lit12 = "        case ADDR_ARGUMENTS:\n            if (eap->line2 > ((curwin)->w_alist->al_ga.ga_len) + (!((curwin)->w_alist->al_ga.ga_len)))\n            {\n                return _(e_invalid_range);\n            }\n            break;\n"
	w21lit13 = "        case ADDR_ARGUMENTS:\n            break;\n"
	w21lit16 = "                result = arg_all();\n                resultbuf = result;\n"
	w21lit17 = "                result = (char_u *)\"\";\n                resultbuf = nullptr;\n"
)
