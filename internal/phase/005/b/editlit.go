package p005b

// The multi-line literals /root/.claude/jobs/107d6bd5/tmp/w72.sh matches on, EXTRACTED from the phase's
// heredoc by tools/gocmp/genlits.py rather than retyped.  They carry BLANK
// LINES, which a filtered read of a phase program does not show, and a
// literal that is 90%% right matches nothing.  See that tool for the
// measurement.
const (
	w5blit3  = "        win = (curwin->w_buffer == buf) ? curwin : nullptr;\n"
	w5blit4  = "        if (curwin->w_buffer == buf)\n        {\n            can_unload = FALSE;\n        }\n"
	w5blit5  = "    return 1;\n"
	w5blit6  = "        if (curwin->w_buffer != nullptr && buf_valid(curwin->w_buffer))\n        {\n            buf = curwin->w_buffer;\n            if ( ((buf)->b_ct_di.di_tv.vval.v_number)  != -1)\n            {\n                bufref_T bufref;\n\n                set_bufref(&bufref, buf);\n                apply_autocmds(EVENT_BUFWINLEAVE, buf->b_fname, buf->b_fname, FALSE, buf);\n                if (bufref_valid(&bufref))\n                {\n                     ((buf)->b_ct_di.di_tv.vval.v_number)  = -1;\n                }\n            }\n        }\n"
	w5blit7  = "    ++autocmd_no_enter;\n    ++autocmd_no_leave;\n\n    curbuf = curwin->w_buffer;\n    if (curbuf->b_ml.ml_mfp == nullptr)\n    {\n        (void)open_buffer(FALSE, nullptr, 0);\n    }\n    ui_breakcheck();\n    if (got_int)\n    {\n        (void)vgetc();\n    }\n\n    curbuf = curwin->w_buffer;\n    --autocmd_no_enter;\n    --autocmd_no_leave;\n"
	w5blit8  = "    return win != nullptr && win == curwin;\n"
	w5blit9  = "    return (curwin->w_id == id) ? curwin : nullptr;\n"
	w5blit10 = "    return tpc == curtab;\n"
	w5blit11 = "    curwin = win_alloc(nullptr, FALSE);\n    if (curwin == nullptr)\n    {\n        return FAIL;\n    }\n    curbuf = buflist_new(nullptr, nullptr, 1L, BLN_LISTED);\n    if (curbuf == nullptr)\n    {\n        return FAIL;\n    }\n    curwin->w_buffer = curbuf;\n    curbuf->b_nwindows = 1;\n    curwin_init();\n\n    new_frame(curwin);\n    if (curwin->w_frame == nullptr)\n    {\n        return FAIL;\n    }\n    topframe = curwin->w_frame;\n    topframe->fr_width =  Columns ;\n    topframe->fr_height = Rows - p_ch;\n\n    return OK;\n"
	w5blit12 = "    if (win_alloc_firstwin(nullptr) == FAIL)\n    {\n        return FAIL;\n    }\n\n    curtab = alloc_tabpage();\n    if (curtab == nullptr)\n    {\n        return FAIL;\n    }\n    unuse_tabpage(curtab);\n\n    return OK;\n"
	w5blit13 = "    tp->tp_topframe = topframe;\n    tp->tp_curwin = curwin;\n"
	w5blit14 = "    redraw_win_later(wp, UPD_NOT_VALID);\n    wp->w_redr_status = true;\n    redraw_cmdline = TRUE;\n"
	w5blit16 = "    if (wp->w_next != nullptr || wp->w_status_height)\n"
	w5blit17 = "    if (wp->w_status_height)\n"
	w5blit18 = "        else if (wp->w_next)\n        {\n            return FAIL;\n        }\n"
	w5blit19 = "        if (did_delete)\n        {\n            wp->w_redr_status = true;\n            win_rest_invalid(((wp)->w_next));\n        }\n"
	w5blit20 = "        if (did_delete)\n        {\n            wp->w_redr_status = true;\n        }\n"
	w5blit21 = "    if (wp->w_next || wp->w_status_height || cmdline_row < Rows - 1)\n"
	w5blit22 = "    if (wp->w_status_height || cmdline_row < Rows - 1)\n"
	w5blit23 = "            wp->w_redr_status = true;\n            win_rest_invalid(wp->w_next);\n"
	w5blit24 = "            wp->w_redr_status = true;\n"
	w5blit25 = "    if (wp->w_next != nullptr && p_tf)\n    {\n        return FAIL;\n    }\n"
)
