package main

import (
	"bytes"
	"math/rand"
	"testing"
)

// cstr is a buffer holding s, its NUL and room to spare, as a Ptr at off.
func cstr(s []byte, room, off int) (Ptr[byte], []byte) {
	buf := make([]byte, off+len(s)+1+room)
	copy(buf[off:], s)
	return View(buf).Add(off), buf
}

func randStr(r *rand.Rand, alphabet []byte) []byte {
	n := r.Intn(12)
	s := make([]byte, n)
	for i := range s {
		s[i] = alphabet[r.Intn(len(alphabet))]
	}
	return s
}

// alphabets: a small one so strings collide, and every nonzero byte.
func alphabets() [][]byte {
	all := make([]byte, 255)
	for i := range all {
		all[i] = byte(i + 1)
	}
	return [][]byte{[]byte("ab"), []byte("aAbB\xe9\xff"), all}
}

func off(p, base Ptr[byte]) int {
	if p.Nil() {
		return -1
	}
	return p.Sub(base)
}

func TestLibcAgainstTranspiled(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for iter := 0; iter < 20000; iter++ {
		al := alphabets()[iter%3]
		a, b := randStr(r, al), randStr(r, al)
		if iter%5 == 0 && len(a) > 2 {
			b = append(append([]byte{}, a[:r.Intn(len(a))]...), b...) // a shared prefix
		}
		pa, _ := cstr(a, 32, r.Intn(4))
		pb, _ := cstr(b, 32, r.Intn(4))
		n := usize(r.Intn(14))

		if g, w := musl_strlen(pa), oracle_strlen(pa); g != w {
			t.Fatalf("strlen(%q) = %d, want %d", a, g, w)
		}
		if g, w := musl_strcmp(pa, pb), oracle_strcmp(pa, pb); g != w {
			t.Fatalf("strcmp(%q, %q) = %d, want %d", a, b, g, w)
		}
		if g, w := musl_strncmp(pa, pb, n), oracle_strncmp(pa, pb, n); g != w {
			t.Fatalf("strncmp(%q, %q, %d) = %d, want %d", a, b, n, g, w)
		}
		c := int32(al[r.Intn(len(al))])
		if iter%7 == 0 {
			c = 0
		}
		if g, w := off(musl_strchr(pa, c), pa), off(oracle_strchr(pa, c), pa); g != w {
			t.Fatalf("strchr(%q, %d) at %d, want %d", a, c, g, w)
		}
		if g, w := off(musl_strstr(pa, pb), pa), off(oracle_strstr(pa, pb), pa); g != w {
			t.Fatalf("strstr(%q, %q) at %d, want %d", a, b, g, w)
		}
		if g, w := off(musl_strpbrk(pa, pb), pa), off(oracle_strpbrk(pa, pb), pa); g != w {
			t.Fatalf("strpbrk(%q, %q) at %d, want %d", a, b, g, w)
		}
		// the copies: the same destination bytes, every one of the buffer
		junk := randStr(r, al)
		d1, buf1 := cstr(junk, 40, 1)
		d2, buf2 := cstr(junk, 40, 1)
		musl_strcpy(d1, pb)
		oracle_strcpy(d2, pb)
		if !bytes.Equal(buf1, buf2) {
			t.Fatalf("strcpy of %q: %q, want %q", b, buf1, buf2)
		}
		musl_strcat(d1, pa)
		oracle_strcat(d2, pa)
		if !bytes.Equal(buf1, buf2) {
			t.Fatalf("strcat of %q: %q, want %q", a, buf1, buf2)
		}
		musl_strncpy(d1, pb, n)
		oracle_strncpy(d2, pb, n)
		if !bytes.Equal(buf1, buf2) {
			t.Fatalf("strncpy(%q, %d): %q, want %q", b, n, buf1, buf2)
		}
	}
}
