package p096

// The multi-line literals internal/phase/096/edit.go matches on, EXTRACTED from the phase's
// heredoc by tools/gocmp/genlits.py rather than retyped.  They carry BLANK
// LINES, which a filtered read of a phase program does not show, and a
// literal that is 90%% right matches nothing.  See that tool for the
// measurement.
const (
	w96lit1  = "    if ((!(State & (MODE_INSERT | MODE_CMDLINE)) || arrow_used) && scriptin[curscript] == NULL)\n"
	w96lit2  = "    if (!(State & (MODE_INSERT | MODE_CMDLINE)) || arrow_used)\n"
	w96lit3  = " && scriptin[curscript] == NULL"
	w96lit4  = "    script_char = -1;\n    while (scriptin[curscript] != NULL && script_char < 0)\n    {\n        if (got_int || (script_char = getc(scriptin[curscript])) < 0)\n        {\n            closescript();\n            if (got_int)\n            {\n                retesc = TRUE;\n            }\n            else\n            {\n                return -1;\n            }\n        }\n        else\n        {\n            buf[0] = script_char;\n            len = 1;\n        }\n    }\n"
	w96lit5  = "            return retesc;\n"
	w96lit6  = "            return FALSE;\n"
	w96lit7  = "    int retesc = FALSE;\n"
	w96lit8  = "    int script_char;\n"
	w96lit9  = "                    redir_write(p, -1);\n"
	w96lit10 = "                redir_write((char_u *)s, -1);\n"
	w96lit11 = "    redir_write((char_u *)str, maxlen);\n"
	w96lit12 = "static void redir_write(char_u *s, int maxlen);\n"
	w96lit13 = "    int did_return = FALSE;\n"
	w96lit14 = "        did_return = TRUE;\n"
	w96lit15 = "static int redir_off = FALSE;\n"
	w96lit16 = "static void ui_write(char_u *s, int len, int console);\n"
	w96lit17 = "static void ui_write(char_u *s, int len);\n"
	w96lit18 = "ui_write(char_u *s, int len, int console __attribute__((unused)))\n{\n    mch_write(s, len);\n    if (console && s[len - 1] == '\\n')\n    {\n        vim_fsync(1);\n    }\n}\n"
	w96lit19 = "ui_write(char_u *s, int len)\n{\n    mch_write(s, len);\n}\n"
	w96lit20 = "    ui_write(out_buf, len, FALSE);\n"
	w96lit21 = "    ui_write(out_buf, len);\n"
)
