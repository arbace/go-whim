package p022

// The multi-line literals /root/.claude/jobs/107d6bd5/tmp/w70.sh matches on, EXTRACTED from the phase's
// heredoc by tools/gocmp/genlits.py rather than retyped.  They carry BLANK
// LINES, which a filtered read of a phase program does not show, and a
// literal that is 90%% right matches nothing.  See that tool for the
// measurement.
const (
	w22OldOpen   = "        if (fnum)\n        {\n            buf = buflist_findnr(fnum);\n        }\n        else\n        {\n            buf = buflist_new(ffname, sfname, 0L, BLN_CURBUF | ((flags & ECMD_SET_HELP) ? 0 : BLN_LISTED));\n            if (oldwin != nullptr)\n            {\n                oldwin = curwin;\n            }\n            set_bufref(&old_curbuf, curbuf);\n        }\n        if (buf == nullptr)\n        {\n            goto theend;\n        }\n"
	w22NewOpen   = "        if (ffname != nullptr)\n        {\n            char_u *new_ffname = ffname;\n            char_u *new_sfname = sfname;\n            stat_T st;\n            fname_expand(curbuf, &new_ffname, &new_sfname);\n            if (new_ffname == nullptr)\n            {\n                goto theend;\n            }\n            if (stat(((char *)new_ffname), (&st)) < 0)\n            {\n                st.st_dev = (dev_t)-1;\n            }\n            new_sfname = vim_strsave(new_sfname);\n            if (new_sfname == nullptr)\n            {\n                vim_free(new_ffname);\n                goto theend;\n            }\n            if (curbuf->b_sfname != curbuf->b_ffname)\n            {\n                vim_free(curbuf->b_sfname);\n            }\n            vim_free(curbuf->b_ffname);\n            curbuf->b_ffname = new_ffname;\n            curbuf->b_sfname = new_sfname;\n            curbuf->b_fname = new_sfname;\n            if (st.st_dev == (dev_t)-1)\n            {\n                curbuf->b_dev_valid = false;\n            }\n            else\n            {\n                curbuf->b_dev_valid = true;\n                curbuf->b_dev = st.st_dev;\n                curbuf->b_ino = st.st_ino;\n            }\n            curbuf->b_shortname = false;\n            status_redraw_all();\n        }\n        if (oldwin != nullptr)\n        {\n            oldwin = curwin;\n        }\n        set_bufref(&old_curbuf, curbuf);\n        buf = curbuf;\n"
	w22OldOldbuf = "        if (buf->b_ml.ml_mfp == nullptr)\n        {\n            oldbuf = FALSE;\n        }\n        else\n        {\n            oldbuf = TRUE;\n            set_bufref(&bufref, buf);\n            if (!bufref_valid(&bufref) || curbuf != old_curbuf.br_buf)\n            {\n                goto theend;\n            }\n        }\n"
	w22lit2      = "        oldbuf = FALSE;\n"
	w22lit3      = "    if (!other_file && !oldbuf)\n"
	w22lit4      = "    if (!oldbuf)\n"
	w22lit5      = "            swap_exists_action = SEA_DIALOG;\n            curbuf->b_flags |= BF_CHECK_RO;\n"
	w22lit6      = "            curbuf->b_flags |= BF_CHECK_RO;\n"
	w22lit7      = "\n            if (swap_exists_action == SEA_QUIT)\n            {\n                retval = FAIL;\n            }\n            handle_swap_exists(&old_curbuf);\n"
)
