package p087

// The multi-line literals phase/087/edit.go matches on, EXTRACTED from the phase's
// heredoc by tools/gocmp/genlits.py rather than retyped.  They carry BLANK
// LINES, which a filtered read of a phase program does not show, and a
// literal that is 90%% right matches nothing.  See that tool for the
// measurement.
const (
	w87lit3  = "    {'Q', nv_exmode, NV_NCW, 0},\n"
	w87lit4  = "    {'Q', nv_error, NV_NCW, 0},\n"
	w87lit5  = "    case 'Q':\n        if (!check_text_locked(cap->oap) && !checkclearopq(oap))\n        {\n            do_exmode(TRUE);\n        }\n        break;\n"
	w87lit6  = "                exmode_active = EXMODE_NORMAL;\n"
	w87lit7  = "                exmode_active = EXMODE_VIM;\n"
	w87lit8  = "                if (exmode_active)\n                {\n                    silent_mode = TRUE;\n                }\n                else\n                {\n                    mainerr(ME_UNKNOWN_OPTION, (char_u *)argv[0]);\n                }\n"
	w87lit9  = "                exmode_active = 0;\n"
	w87lit10 = "\n    int exmode_was = exmode_active;\n"
	w87lit12 = " || exmode_active"
	w87lit13 = "\n    check_tty();\n"
	w87lit14 = "\n    int save_silent = silent_mode;\n"
	w87lit15 = "\n    silent_mode = FALSE;\n"
	w87lit16 = "\n    silent_mode = save_silent;\n"
	w87lit17 = "\nstatic int ex_pressedreturn = FALSE;\n"
	w87lit18 = "\n                ex_pressedreturn = TRUE;\n"
	w87lit19 = "\nstatic int ex_no_reprint = FALSE;\n"
	w87lit20 = "\n    ex_no_reprint = TRUE;\n"
	w87lit21 = "\n        ex_no_reprint = TRUE;\n"
	w87lit22 = "\nstatic int ex_exitval = 0;\n"
	w87lit23 = "\n        ex_exitval = 1;\n"
	w87lit24 = "\n    volatile int previous_got_int = FALSE;\n"
	w87lit25 = "\n            previous_got_int = TRUE;\n"
	w87lit26 = "        else\n        {\n            previous_got_int = FALSE;\n        }\n"
	w87lit27 = "\n    int use_plus_cmd = FALSE;\n"
	w87lit28 = "\nstatic void main_loop(int cmdwin, int noexmode);\n"
	w87lit29 = "\nstatic void main_loop(int cmdwin);\n"
	w87lit30 = "main_loop(int cmdwin, int noexmode)\n"
	w87lit31 = "main_loop(int cmdwin)\n"
	w87lit32 = "\n    main_loop(FALSE, FALSE);\n"
	w87lit33 = "\n    main_loop(FALSE);\n"
	w87lit34 = "\ntheend:\n    current_oap = prev_oap;\n"
	w87lit35 = "\n    current_oap = prev_oap;\n"
)
