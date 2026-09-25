// The C string functions the core calls, written in Go on Go's own byte
// functions instead of transpiled from musl byte by byte.  Their contracts are
// musl's exactly -- a comparison returns the difference of the first bytes
// that differ, not only its sign, and strchr finds the terminator when asked
// for 0 -- and libc_test.go holds each to the transpiled original.
//
// A C string is the bytes before the first NUL.  One with no NUL in its
// allocation is a read past the end in C; here it is a panic, as the
// transpiled loop's out-of-range index was.
package editor

import "bytes"

// cstring is the C string at p, without its terminator.
func cstring(p Ptr[byte]) []byte {
	s := p.slice()[p.i:]
	n := bytes.IndexByte(s, 0)
	if n < 0 {
		panic("a C string with no terminator")
	}
	return s[:n]
}

// at is s[i], or the terminator's 0 past its end.
func at(s []byte, i int) int32 {
	if i < len(s) {
		return int32(s[i])
	}
	return 0
}

// same is how many leading bytes a and b share, up to limit.
func same(a, b []byte, limit int) int {
	i := 0
	for i < limit && i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	return i
}

func musl_strlen(s Ptr[byte]) usize { return usize(len(cstring(s))) }

func musl_strcpy(dest Ptr[byte], src Ptr[byte]) Ptr[byte] {
	s := cstring(src)
	d := dest.Slice(len(s) + 1)
	copy(d, s)
	d[len(s)] = 0
	return dest
}

// musl_strncpy copies at most n bytes and pads what is left of n with NULs.
func musl_strncpy(dest Ptr[byte], src Ptr[byte], n usize) Ptr[byte] {
	d := dest.Slice(int(n))
	clear(d[copy(d, cstring(src)):])
	return dest
}

func musl_strcat(dest Ptr[byte], src Ptr[byte]) Ptr[byte] {
	musl_strcpy(dest.Add(len(cstring(dest))), src)
	return dest
}

func musl_strcmp(l Ptr[byte], r Ptr[byte]) int32 {
	a, b := cstring(l), cstring(r)
	i := same(a, b, len(a)+1)
	return at(a, i) - at(b, i)
}

// musl_strncmp compares at most n bytes: the first n-1 that agree decide
// nothing, and the n-th's difference is the answer.
func musl_strncmp(ls Ptr[byte], rs Ptr[byte], n usize) int32 {
	if n == 0 {
		return 0
	}
	a, b := cstring(ls), cstring(rs)
	i := same(a, b, int(n)-1)
	return at(a, i) - at(b, i)
}

// musl_strchr finds byte(c), the terminator included: asked for 0, it is
// the end of the string.
func musl_strchr(s Ptr[byte], c int32) Ptr[byte] {
	str := cstring(s)
	if byte(c) == 0 {
		return s.Add(len(str))
	}
	if i := bytes.IndexByte(str, byte(c)); i >= 0 {
		return s.Add(i)
	}
	return Ptr[byte]{}
}

func musl_strstr(h Ptr[byte], n Ptr[byte]) Ptr[byte] {
	if i := bytes.Index(cstring(h), cstring(n)); i >= 0 {
		return h.Add(i)
	}
	return Ptr[byte]{}
}

// musl_strpbrk finds the first byte of s that is in b.  A byte set rather
// than bytes.IndexAny, which reads its set as UTF-8.
func musl_strpbrk(s Ptr[byte], b Ptr[byte]) Ptr[byte] {
	var in [256]bool
	for _, c := range cstring(b) {
		in[c] = true
	}
	for i, c := range cstring(s) {
		if in[c] {
			return s.Add(i)
		}
	}
	return Ptr[byte]{}
}
