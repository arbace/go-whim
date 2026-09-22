package p140

// Whim phase 140, the check -- highlight groups are found in their array.
// See phase/140/edit.go, and GOALS.md.
//
// phase/140/check.go proves the scan finds what the table found, requires
// the hash table gone, and probes group lookup, a taken-back group and a link.

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/check"
	"github.com/arbace/go-whim/internal/cutil"
	"github.com/arbace/go-whim/internal/harness"
)

func init() { check.Register("whim140", Check) }

// Whim140 is phase 140's check: highlight groups are found in their array.
//
//  1. WHY THE SCAN FINDS WHAT THE TABLE FOUND, on the input: syn_add_group()
//     has one call, in syn_check_group(), taken only when syn_name2id_len()
//     returned 0 for the name -- so no two groups share an upper-cased name;
//     the table's key for a group is its sg_name_u, the same pointer; and the
//     id stored beside the key is highlight_ga.ga_len + 1 just before the
//     length goes up -- the group's index + 1.  highlight_ht is the only table.
//  2. THE CUT: syn_name2id_len() and syn_unadd_group() are the bodies this
//     phase writes, and the hash table -- hashtab_T, hashitem_T, hash_init,
//     hash_find, hash_add, hash_remove, hash_removed, hlname_T, highlight_ht --
//     has no mention.
//  3. THE GATE, the libc surface unchanged.
//  4. THE PROBES: a new group looked up in another case, a group a failed :hi
//     added and took back, and a link; each the same on both binaries, each
//     CONTROL moves.
func Check(w io.Writer, args []string) error {
	c, err := check.NewCore(w, args, "whim140", "hlname")
	if err != nil {
		return err
	}
	r := c.R
	blank := string(cutil.Blank([]byte(c.Old)))
	if n := len(regexp.MustCompile(`\bsyn_add_group\(`).FindAllString(blank, -1)); n != 3 {
		r.Bad("syn_add_group is named %d times on the input: its prototype, definition and one call are what this phase depends on", n)
	}
	if !strings.Contains(c.Old, "    id = syn_name2id_len(pp, len);\n    if (id == 0)\n    {\n        name = vim_strnsave(pp, len);\n        if (name == nullptr)\n        {\n            return 0;\n        }\n        id = syn_add_group(name);\n    }\n") {
		r.Bad("syn_add_group() is not called only when the name was not found")
	}
	for _, s := range []string{
		"        hn->hn_id = highlight_ga.ga_len + 1;\n        name_up = hn->hn_key;\n",
		"[highlight_ga.ga_len].sg_name_u = name_up;\n    hash_add(&highlight_ht, name_up, \"highlight\");\n    ++highlight_ga.ga_len;\n",
	} {
		if strings.Count(c.Old, s) != 1 {
			r.Bad("the input does not store the key and id this phase depends on: %q", s)
		}
	}
	if ts := regexp.MustCompile(`(?m)^static hashtab_T\s+(\w+);`).FindAllStringSubmatch(c.Old, -1); len(ts) != 1 || ts[0][1] != "highlight_ht" {
		r.Bad("highlight_ht is not the only hash table: %v", ts)
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("on the input a group is added only when its name was not found, its key is its sg_name_u and its id its index + 1, and highlight_ht is the only table: a scan of the array finds what the table found")

	lo, lc, lf, _ := cutil.Body([]byte(c.New), "syn_name2id_len")
	if !lf || c.New[lo:lc+1] != "{\n"+W140LookupBody+"\n}" {
		r.Bad("syn_name2id_len()'s body is not the one this phase writes")
	}
	uo, uc, uf, _ := cutil.Body([]byte(c.New), "syn_unadd_group")
	if !uf || c.New[uo:uc+1] != "{\n    --highlight_ga.ga_len;\n}" {
		r.Bad("syn_unadd_group()'s body is not the one this phase writes")
	}
	for _, n := range []string{"hashtab_T", "hashitem_T", "hash_init", "hash_find", "hash_add", "hash_remove", "hash_removed", "hash_lookup", "hlname_T", "highlight_ht", "highlight_ht_inited"} {
		if k := check.Word(c.New, n); k != 0 {
			r.Bad("%s has %d mentions left", n, k)
		}
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("the lookup scans highlight_ga, a failed group is the length going down, and the hash table and hlname_T are gone (the file lost %d lines)",
		check.CountLines([]byte(c.Old))-check.CountLines([]byte(c.New)))
	if err := c.Gate(true); err != nil {
		return err
	}

	ob, nb := c.Bins()
	seed := []byte("iabc\x1b")
	probes := []struct{ What, Keys, ctl string }{
		{"a new group, found in another case", ":hi MyGrp ctermfg=1\r:hi mygrp\r", ":hi MyGrp ctermfg=2\r:hi mygrp\r"},
		{"a group a failed :hi added and took back", ":hi BadGrp foo=bar\r:hi BadGrp\r", ":hi BadGrp ctermfg=1\r:hi BadGrp\r"},
		{"a link", ":hi link LnA Search\r:hi LnA\r", ":hi link LnA Visual\r:hi LnA\r"},
	}
	for _, pr := range probes {
		keys := [][]byte{seed, []byte(pr.Keys), []byte(":q!\r")}
		s1, _, e1 := check.Stream(ob, keys, nil)
		s2, _, e2 := check.Stream(nb, keys, nil)
		s3, _, e3 := check.Stream(nb, [][]byte{seed, []byte(pr.ctl), []byte(":q!\r")}, nil)
		if e1 != nil || e2 != nil || e3 != nil {
			r.Say("a probe did not run: %v %v %v", e1, e2, e3)
			return harness.ErrReported
		}
		if s1 != s2 {
			r.Bad("%s is written differently on the two binaries", pr.What)
		}
		if s3 == s2 {
			r.Bad("the CONTROL for %s did not move", pr.What)
		}
	}
	if err := r.Done(); err != nil {
		return err
	}
	r.Say("PROBE: a new group found in another case, a group a failed :hi took back, and a link write the same bytes on both binaries; each CONTROL moves")
	return nil
}
