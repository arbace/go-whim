package p053

// The multi-line literals internal/phase/053/edit.go matches on, EXTRACTED from the phase's
// heredoc by tools/gocmp/genlits.py rather than retyped.  They carry BLANK
// LINES, which a filtered read of a phase program does not show, and a
// literal that is 90%% right matches nothing.  See that tool for the
// measurement.
const (
	w53lit4  = "    if (hp->bh_hashitem.mhi_key != 1)\n    {\n        iemsg(e_didnt_get_block_nr_one);\n"
	w53lit5  = "    if (hp->bh_hashitem.mhi_key != 0)\n    {\n        iemsg(e_didnt_get_block_nr_zero);\n"
	w53lit6  = "    pp->pb_pointer[0].pe_bnum = 2;\n"
	w53lit7  = "    pp->pb_pointer[0].pe_bnum = 1;\n"
	w53lit8  = "    if (hp->bh_hashitem.mhi_key != 2)\n    {\n        iemsg(e_didnt_get_block_nr_two);\n"
	w53lit9  = "    bnum = 1;\n    page_count = 1;\n"
	w53lit10 = "    bnum = 0;\n    page_count = 1;\n"
	w53lit11 = "                if (hp->bh_hashitem.mhi_key != 1)\n"
	w53lit12 = "                if (hp->bh_hashitem.mhi_key != 0)\n"
	w53lit13 = "        if (negative)\n        {\n            hp->bh_hashitem.mhi_key = mfp->mf_blocknr_min--;\n            mfp->mf_neg_count++;\n        }\n        else\n        {\n            hp->bh_hashitem.mhi_key = mfp->mf_blocknr_max;\n            mfp->mf_blocknr_max += page_count;\n        }\n"
	w53lit14 = "        hp->bh_hashitem.mhi_key = mfp->mf_blocknr_max;\n        mfp->mf_blocknr_max += page_count;\n"
	w53lit15 = "        buf->b_ml.ml_flags |= ML_LOCKED_DIRTY;\n        if (!(flags & ML_APPEND_NEW))\n        {\n            buf->b_ml.ml_flags |= ML_LOCKED_POS;\n        }\n"
	w53lit16 = "        if (lines_moved || in_left)\n        {\n            buf->b_ml.ml_flags |= ML_LOCKED_DIRTY;\n        }\n        if (!(flags & ML_APPEND_NEW) && db_idx >= 0 && in_left)\n        {\n            buf->b_ml.ml_flags |= ML_LOCKED_POS;\n        }\n"
	w53lit17 = "        mf_put(mfp, buf->b_ml.ml_locked, buf->b_ml.ml_flags & ML_LOCKED_DIRTY, buf->b_ml.ml_flags & ML_LOCKED_POS);\n"
	w53lit18 = "        mf_put(buf->b_ml.ml_locked);\n"
	w53lit19 = "mf_put(memfile_T *mfp, bhdr_T *hp, int dirty, int infile)\n{\n    int flags;\n    flags = hp->bh_flags;\n    if ((flags & BH_LOCKED) == 0)\n    {\n        iemsg(e_block_was_not_locked);\n    }\n    flags &= ~BH_LOCKED;\n    if (dirty)\n    {\n        flags |= BH_DIRTY;\n        if (mfp->mf_dirty != MF_DIRTY_YES_NOSYNC)\n        {\n            mfp->mf_dirty = MF_DIRTY_YES;\n        }\n    }\n    hp->bh_flags = flags;\n    if (infile)\n    {\n        mf_trans_add(mfp, hp);\n    }\n}"
	w53lit20 = "mf_put(bhdr_T *hp)\n{\n    if ((hp->bh_flags & BH_LOCKED) == 0)\n    {\n        iemsg(e_block_was_not_locked);\n    }\n    hp->bh_flags &= ~BH_LOCKED;\n}"
	w53lit21 = "                if (bnum < 0)\n                {\n                    bnum2 = mf_trans_del(mfp, bnum);\n                    if (bnum != bnum2)\n                    {\n                        bnum = bnum2;\n                        pp->pb_pointer[idx].pe_bnum = bnum;\n                        dirty = TRUE;\n                    }\n                }\n"
	w53lit23 = "static int mf_trans_add(memfile_T *, bhdr_T *);\n"
	w53lit24 = "    if (hp->bh_hashitem.mhi_key < 0)\n    {\n        vim_free(hp);\n        mfp->mf_neg_count--;\n    }\n    else\n    {\n        mf_ins_free(mfp, hp);\n    }\n"
	w53lit25 = "    mf_ins_free(mfp, hp);\n"
	w53lit26 = "    mf_hash_init(&mfp->mf_trans);\n"
	w53lit27 = "    mfp->mf_blocknr_min = -1;\n"
	w53lit28 = "    mfp->mf_neg_count = 0;\n"
	w53lit29 = "    mf_hash_free_all(&mfp->mf_trans);\n"
	w53lit33 = "    if (curbuf->b_ml.ml_mfp != nullptr)\n    {\n        curbuf->b_ml.ml_mfp->mf_dirty = MF_DIRTY_YES_NOSYNC;\n    }\n"
	w53lit34 = "    if (curbuf->b_ml.ml_mfp != nullptr && curbuf->b_ml.ml_mfp->mf_dirty == MF_DIRTY_YES_NOSYNC)\n    {\n        curbuf->b_ml.ml_mfp->mf_dirty = MF_DIRTY_YES;\n    }\n"
	w53lit35 = "    mfp->mf_used_last = nullptr;\n    mfp->mf_dirty = MF_DIRTY_NO;\n"
	w53lit36 = "    mfp->mf_used_last = nullptr;\n"
	w53lit37 = "    hp->bh_flags = BH_LOCKED | BH_DIRTY;\n    mfp->mf_dirty = MF_DIRTY_YES;\n"
	w53lit38 = "    hp->bh_flags = BH_LOCKED;\n"
	w53lit39 = "    mfdirty_T mf_dirty;\n"
	w53lit40 = "typedef enum\n{\n    MF_DIRTY_NO = 0,\n    MF_DIRTY_YES,\n    MF_DIRTY_YES_NOSYNC,\n} mfdirty_T;\n\n"
	w53lit42 = "        if (db_idx < 0)\n        {\n            lnum_left = lnum + 1;\n            lnum_right = 0;\n        }\n        else\n        {\n            lnum_left = 0;\n            if (in_left)\n            {\n                lnum_right = lnum + 2;\n            }\n            else\n            {\n                lnum_right = lnum + 1;\n            }\n        }\n"
	w53lit45 = "            lnum_left = 0;\n            lnum_right = 0;\n"
)
