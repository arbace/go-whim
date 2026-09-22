// The C runtime the transpilation writes against: a pointer that can walk,
// allocation that the garbage collector owns, and the handful of libc
// functions whose C signature is `void *` -- which a Go program writes
// generically rather than byte by byte.
//
// THIS IS THE ONE FILE THAT USES unsafe, and only to make Ptr a comparable
// value: a C pointer is compared with == far more often than it is walked,
// and a Ptr that held a slice could not be.  Everything unsafe here is the
// arithmetic of an index into a Go allocation the Ptr keeps alive.
package main

import (
	"reflect"
	"unsafe"
)

// Ptr is a C pointer to T that may walk: the allocation's first element, its
// length, and an offset into it.  The zero Ptr is NULL.  Two Ptr are == when
// they point at the same element of the same allocation, as in C.
type Ptr[T any] struct {
	base *T
	n    int
	i    int
}

// Mk allocates n zeroed elements (C: alloc_clear, calloc, a local array).
func Mk[T any](n int) Ptr[T] {
	if n < 1 {
		n = 1
	}
	s := make([]T, n)
	return Ptr[T]{&s[0], n, 0}
}

// View is a Ptr over the elements of a Go slice, sharing them.
func View[T any](s []T) Ptr[T] {
	if len(s) == 0 {
		return Mk[T](1)
	}
	return Ptr[T]{&s[0], len(s), 0}
}

// Addr is &x as a Ptr: a one-element allocation that is x itself.
func Addr[T any](x *T) Ptr[T] { return Ptr[T]{x, 1, 0} }

func (p Ptr[T]) slice() []T {
	if p.base == nil {
		panic("NULL pointer dereference")
	}
	return unsafe.Slice(p.base, p.n)
}

// Nil is p == NULL.
func (p Ptr[T]) Nil() bool { return p.base == nil }

// At is p[k].
func (p Ptr[T]) At(k int) T { return p.slice()[p.i+k] }

// Get is *p.
func (p Ptr[T]) Get() T { return p.slice()[p.i] }

// Set is p[k] = v.
func (p Ptr[T]) Set(k int, v T) { p.slice()[p.i+k] = v }

// Put is *p = v.
func (p Ptr[T]) Put(v T) { p.slice()[p.i] = v }

// Ref is &p[k], a Go pointer to one element (C: p->field is p.Ref(0).field).
func (p Ptr[T]) Ref(k int) *T { return &p.slice()[p.i+k] }

// P is &p[0]: struct access through a walking pointer, p.P().field.
func (p Ptr[T]) P() *T { return &p.slice()[p.i] }

// Add is p + k (C permits one past the end; so does this).
func (p Ptr[T]) Add(k int) Ptr[T] {
	if p.base == nil {
		if k == 0 {
			return p
		}
		panic("arithmetic on NULL")
	}
	return Ptr[T]{p.base, p.n, p.i + k}
}

// Sub is p - q, both into the same allocation.
func (p Ptr[T]) Sub(q Ptr[T]) int {
	if p.base != q.base {
		if p.base == nil || q.base == nil {
			panic("pointer difference with NULL")
		}
		// two views of one allocation, or two allocations: the addresses decide
		var z T
		sz := int(unsafe.Sizeof(z))
		if sz == 0 {
			sz = 1
		}
		return (int(uintptr(unsafe.Pointer(p.base)))-int(uintptr(unsafe.Pointer(q.base))))/sz + p.i - q.i
	}
	return p.i - q.i
}

// Lt, Le, Gt, Ge order two pointers into one allocation.
func (p Ptr[T]) Lt(q Ptr[T]) bool { return p.Sub(q) < 0 }
func (p Ptr[T]) Le(q Ptr[T]) bool { return p.Sub(q) <= 0 }
func (p Ptr[T]) Gt(q Ptr[T]) bool { return p.Sub(q) > 0 }
func (p Ptr[T]) Ge(q Ptr[T]) bool { return p.Sub(q) >= 0 }

// Len is how many elements remain from p to the allocation's end.
func (p Ptr[T]) Len() int { return p.n - p.i }

// Slice is the n elements from p, sharing them.
func (p Ptr[T]) Slice(n int) []T { return p.slice()[p.i : p.i+n] }

// Eq and Ne are for the places where == is awkward to write.
func (p Ptr[T]) Eq(q Ptr[T]) bool { return p == q }

// --- strings ----------------------------------------------------------------

var literals = map[string]Ptr[byte]{}

// S is a C string literal: NUL-terminated, one allocation per distinct text,
// so the same literal twice is the same pointer as it usually is in C.
func S(s string) Ptr[byte] {
	if p, ok := literals[s]; ok {
		return p
	}
	b := make([]byte, len(s)+1)
	copy(b, s)
	p := Ptr[byte]{&b[0], len(b), 0}
	literals[s] = p
	return p
}

// GoString is the NUL-terminated bytes at p as a Go string (for the host).
func GoString(p Ptr[byte]) string {
	if p.Nil() {
		return ""
	}
	s := p.slice()[p.i:]
	for k, c := range s {
		if c == 0 {
			return string(s[:k])
		}
	}
	return string(s)
}

// --- allocation: the garbage collector is the host's allocator ---------------

// Alloc is alloc(n) and lalloc(n) for bytes.  A C allocation of a struct is
// new(T) in Go, and of n elements of T is Mk[T](n); only raw bytes come here.
func Alloc(n int) Ptr[byte] { return Mk[byte](n) }

// Realloc is vim_realloc(p, n): a new allocation holding the old elements.
func Realloc[T any](p Ptr[T], n int) Ptr[T] {
	q := Mk[T](n)
	if !p.Nil() {
		m := p.Len()
		if m > n {
			m = n
		}
		copy(q.slice()[:m], p.slice()[p.i:p.i+m])
	}
	return q
}

// --- the void * libc functions, in elements and not in bytes ---------------

// Memmove is memmove and memcpy: n elements from src to dst.
func Memmove[T any](dst, src Ptr[T], n int) Ptr[T] {
	if n > 0 {
		copy(dst.slice()[dst.i:dst.i+n], src.slice()[src.i:src.i+n])
	}
	return dst
}

// Memset is memset of bytes.
func Memset(p Ptr[byte], c int32, n int) Ptr[byte] {
	s := p.slice()[p.i : p.i+n]
	for k := range s {
		s[k] = byte(c)
	}
	return p
}

// Zero is memset(p, 0, n * sizeof(T)) for any element type.
func Zero[T any](p Ptr[T], n int) {
	var z T
	s := p.slice()[p.i : p.i+n]
	for k := range s {
		s[k] = z
	}
}

// Memcmp is memcmp of bytes.
func Memcmp(a, b Ptr[byte], n int) int32 {
	x, y := a.slice()[a.i:a.i+n], b.slice()[b.i:b.i+n]
	for k := 0; k < n; k++ {
		if x[k] != y[k] {
			if x[k] < y[k] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// --- the C conversions Go makes explicit ---------------------------------------

// B2i is a C comparison or logical operator's int result.
func B2i(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// --- growarray: garray_T.ga_data holds a Ptr[T] as `any` -----------------------

// GaData is ((T *)gap->ga_data): the growarray's storage as Ptr[T].  The type
// is only known where the C code casts, so storage is made here, lazily, at
// the first typed access, as large as ga_grow has asked for (ga_maxlen); and
// grown here when ga_grow has asked for more since.
func GaData[T any](gap *S_growarray) Ptr[T] {
	want := int(gap.ga_maxlen)
	if want < 1 {
		want = 1
	}
	switch d := gap.ga_data.(type) {
	case nil:
		p := Mk[T](want)
		gap.ga_data = p
		return p
	case Ptr[T]:
		if d.Nil() {
			p := Mk[T](want)
			gap.ga_data = p
			return p
		}
		if d.Len() < want {
			d = Realloc(d, want)
			gap.ga_data = d
		}
		return d
	default:
		panic("growarray used as two element types")
	}
}

// GaGrowTo is what ga_grow does to the storage: if it exists (and so has a
// type), make it hold n elements; if it does not, GaData will when first used.
func GaGrowTo(gap *S_growarray, n int) {
	if gap.ga_data == nil {
		return
	}
	v := reflect.ValueOf(gap.ga_data)
	if m := v.MethodByName("GrowTo"); m.IsValid() {
		gap.ga_data = m.Call([]reflect.Value{reflect.ValueOf(n)})[0].Interface()
	}
}

// GrowTo is Realloc for GaGrowTo, reachable through reflection.
func (p Ptr[T]) GrowTo(n int) Ptr[T] {
	if p.Nil() {
		return Mk[T](n)
	}
	if p.Len() >= n {
		return p
	}
	return Realloc(p, n)
}

// --- container_of: a key recovered as the struct it is inside -------------------
