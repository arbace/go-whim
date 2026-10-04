package graph

import "strings"

// A lineHunk is one change between two texts as lines: the old text's lines
// [OA, OB) are the new text's lines [NA, NB).  OA == OB is an insertion
// before old line OA; NA == NB a deletion.
type lineHunk struct{ OA, OB, NA, NB int }

// splitLines is s as lines, each with its newline; a last line without one
// is a line too.
func splitLines(s string) []string {
	var out []string
	for len(s) > 0 {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			out = append(out, s)
			break
		}
		out = append(out, s[:i+1])
		s = s[i+1:]
	}
	return out
}

// diffLines is a shortest edit script from a to b (Myers' greedy
// algorithm), as hunks in order.  The common head and tail are taken off
// first, so its cost is in what differs.
func diffLines(a, b []string) []lineHunk {
	ids := map[string]int32{}
	id := func(ls []string) []int32 {
		out := make([]int32, len(ls))
		for i, l := range ls {
			k, ok := ids[l]
			if !ok {
				k = int32(len(ids))
				ids[l] = k
			}
			out[i] = k
		}
		return out
	}
	x, y := id(a), id(b)
	lo := 0
	for lo < len(x) && lo < len(y) && x[lo] == y[lo] {
		lo++
	}
	hx, hy := len(x), len(y)
	for hx > lo && hy > lo && x[hx-1] == y[hy-1] {
		hx--
		hy--
	}
	x, y = x[lo:hx], y[lo:hy]
	n, m := len(x), len(y)
	if n == 0 && m == 0 {
		return nil
	}
	if n == 0 || m == 0 {
		return []lineHunk{{lo, lo + n, lo, lo + m}}
	}
	max := n + m
	off := max
	v := make([]int, 2*max+2)
	var trace [][]int
	var dEnd int
search:
	for d := 0; d <= max; d++ {
		trace = append(trace, append([]int(nil), v[off-d:off+d+2]...))
		for k := -d; k <= d; k += 2 {
			var i int
			if k == -d || k != d && v[off+k-1] < v[off+k+1] {
				i = v[off+k+1]
			} else {
				i = v[off+k-1] + 1
			}
			j := i - k
			for i < n && j < m && x[i] == y[j] {
				i++
				j++
			}
			v[off+k] = i
			if i >= n && j >= m {
				dEnd = d
				break search
			}
		}
	}
	// back from the end: each round is one edit, then a run of matches
	var hunks []lineHunk
	add := func(oa, ob, na, nb int) {
		if k := len(hunks) - 1; k >= 0 && hunks[k].OA == ob && hunks[k].NA == nb {
			hunks[k].OA, hunks[k].NA = oa, na
			return
		}
		hunks = append(hunks, lineHunk{oa, ob, na, nb})
	}
	i, j := n, m
	for d := dEnd; d > 0; d-- {
		vd := trace[d] // v as it was before round d: index k+d is diagonal k
		at := func(k int) int { return vd[k+d] }
		k := i - j
		var pk int
		if k == -d || k != d && at(k-1) < at(k+1) {
			pk = k + 1 // an insertion: down from diagonal k+1
		} else {
			pk = k - 1 // a deletion: right from diagonal k-1
		}
		pi := at(pk)
		pj := pi - pk
		if pk == k+1 {
			add(pi, pi, pj, pj+1)
		} else {
			add(pi, pi+1, pj, pj)
		}
		i, j = pi, pj
	}
	// hunks were made back to front
	for a, z := 0, len(hunks)-1; a < z; a, z = a+1, z-1 {
		hunks[a], hunks[z] = hunks[z], hunks[a]
	}
	for k := range hunks {
		hunks[k].OA += lo
		hunks[k].OB += lo
		hunks[k].NA += lo
		hunks[k].NB += lo
	}
	return hunks
}
