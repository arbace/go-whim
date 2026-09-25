// The host: everything the editor core asks of an operating system, from the
// Go runtime and standard library.  This is the Go port of the part of
// whim-vim.c from its first #include to its end (.tmp/tx/chunks/host.c):
// vim's own printf (vim_snprintf), the terminal, the clock, input with a
// timeout, the signals, output, the allocator, and the launcher main().
//
// Deviations from the C are marked DEVIATION where they happen; the
// important ones are about signals, which Go cannot run asynchronously on the
// thread running the core:
//
//   - The C handlers for SIGWINCH/SIGCONT, SIGTSTP and SIGINT only set a flag.
//     Here a goroutine receives the signal from os/signal and sets the same
//     flag (atomically), then writes one byte to a wake-up pipe so a blocking
//     select in the host returns, as the C select/read/nanosleep returns with
//     EINTR.  The flags are read only by the host, as in the C.
//   - SIGHUP and SIGTERM run the core's deathtrap() in the C, at whatever
//     point the core is.  Here the goroutine queues the signal and deathtrap()
//     runs on the main goroutine at the next point the host is entered to
//     wait, read or sleep (musl_wait_for_input, musl_read_input, musl_delay).
//     vim's long operations poll for input (ui_breakcheck), so those points
//     are reached; a core that loops without ever polling would not die of
//     SIGTERM/SIGHUP, where the C would.
//   - host_raise() of a signal the host catches runs that handler's effect
//     directly (the C kill(getpid()) runs the handler before kill returns).
package main

import (
	"os"
	"os/signal"
	"reflect"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// ---------------------------------------------------------------------------
// vim_snprintf: vim's printf (vim_vsnprintf / vim_vsnprintf_typval).
//
// The C va_list is the Go variadic slice and an index into it: va_arg reads
// args[ap] and advances; va_copy(ap, ap_start) resets the index to 0.  Each
// argument is converted to the type the C va_arg names (hostVaInt & co.).
//
// vim_vsnprintf_typval is only ever called with tvs == NULL (by
// vim_vsnprintf), so the typval parameter is dropped and every `tvs !=
// nullptr` is false: get_unsigned_int clamps instead of reporting overflow,
// and the "too many arguments" check never runs.
// ---------------------------------------------------------------------------

const TMP_LEN = 350

const (
	TYPE_UNKNOWN = -1 + iota
	TYPE_INT
	TYPE_LONGINT
	TYPE_LONGLONGINT
	TYPE_UNSIGNEDINT
	TYPE_UNSIGNEDLONGINT
	TYPE_UNSIGNEDLONGLONGINT
	TYPE_POINTER
	TYPE_PERCENT
	TYPE_CHAR
	TYPE_STRING
)

const MAX_ALLOWED_STRING_WIDTH = 1048576

var (
	typename_unknown                               = S("unknown")
	typename_int                                   = S("int")
	typename_longint                               = S("long int")
	typename_longlongint                           = S("long long int")
	typename_unsignedint                           = S("unsigned int")
	typename_unsignedlongint                       = S("unsigned long int")
	typename_unsignedlonglongint                   = S("unsigned long long int")
	typename_pointer                               = S("pointer")
	typename_percent                               = S("percent")
	typename_char                                  = S("char")
	typename_string                                = S("string")
	e_cannot_mix_positional_and_non_positional_str = S("E1500: Cannot mix positional and non-positional arguments: %s")
	e_fmt_arg_nr_unused_str                        = S("E1501: format argument %d unused in $-style format: %s")
	e_positional_num_field_spec_reused_str_str     = S("E1502: Positional argument %d used as field width reused as different type: %s/%s")
	e_positional_arg_num_type_inconsistent_str_str = S("E1504: Positional argument %d type used inconsistently: %s/%s")
	e_invalid_format_specifier_str                 = S("E1505: Invalid format specifier: %s")
	e_aptypes_is_null_nr_str                       = S("E1507: Internal error: ap_types or ap_types[idx] is NULL: %d: %s")
)

// hostVa is a va_list: the arguments and the index of the next one.
type hostVa struct {
	args []any
	i    int
}

func (ap *hostVa) next() any {
	if ap.i >= len(ap.args) {
		panic("vim_snprintf: va_arg past the last argument")
	}
	a := ap.args[ap.i]
	ap.i++
	return a
}

// hostVaInt reads any Go integer as C would read it had it been passed at its
// own width and read back at 64 bits: signed types sign-extend, unsigned ones
// zero-extend.  The caller truncates to the width its va_arg names.
func hostVaInt(a any) int64 {
	switch x := a.(type) {
	case int32:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	case uint32:
		return int64(x)
	case uint64:
		return int64(x)
	case byte:
		return int64(x)
	case int8:
		return int64(x)
	case int16:
		return int64(x)
	case uint16:
		return int64(x)
	case uint:
		return int64(x)
	case uintptr:
		return int64(x)
	case bool:
		if x {
			return 1
		}
		return 0
	}
	panic("vim_snprintf: an integer conversion was given " + reflect.TypeOf(a).String())
}

func (ap *hostVa) vaInt() int32    { return int32(hostVaInt(ap.next())) }
func (ap *hostVa) vaUint() uint32  { return uint32(hostVaInt(ap.next())) }
func (ap *hostVa) vaLong() int64   { return hostVaInt(ap.next()) }
func (ap *hostVa) vaUlong() uint64 { return uint64(hostVaInt(ap.next())) }
func (ap *hostVa) vaSkip()         { ap.next() }
func (ap *hostVa) vaString() Ptr[byte] {
	switch x := ap.next().(type) {
	case nil:
		return Ptr[byte]{}
	case Ptr[byte]:
		return x
	case string:
		b := Mk[byte](len(x) + 1)
		copy(b.Slice(len(x)), x)
		return b
	default:
		panic("vim_snprintf: %s was given " + reflect.TypeOf(x).String())
	}
}

// vaPointer is va_arg(ap, void *) as the address it would print.  A Go
// allocation's address is not the C one, so %p prints an address no C run
// would: the core uses %p for nothing a user sees.
func (ap *hostVa) vaPointer() uint64 {
	a := ap.next()
	if a == nil {
		return 0
	}
	if n, ok := a.(interface{ Nil() bool }); ok && n.Nil() {
		return 0
	}
	v := reflect.ValueOf(a)
	switch v.Kind() {
	case reflect.Pointer, reflect.Func, reflect.Map, reflect.Slice, reflect.Chan, reflect.UnsafePointer:
		return uint64(v.Pointer())
	case reflect.Struct:
		// a Ptr[T]: its base plus its offset in elements
		if v.NumField() == 3 && v.Field(0).Kind() == reflect.Pointer {
			return uint64(v.Field(0).Pointer()) + uint64(v.Field(2).Int())*uint64(v.Field(0).Type().Elem().Size())
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return uint64(hostVaInt(a))
	}
	return 0
}

func hostIsDigit(c byte) bool { return uint32(c)-'0' < 10 }

func hostStrlen(s Ptr[byte]) usize {
	var n usize
	for s.At(int(n)) != NUL {
		n++
	}
	return n
}

func hostStrchr(s Ptr[byte], c byte) Ptr[byte] {
	for {
		if s.Get() == c {
			return s
		}
		if s.Get() == NUL {
			return Ptr[byte]{}
		}
		s = s.Add(1)
	}
}

func musl_memchr(src Ptr[byte], c int32, n usize) Ptr[byte] {
	s := src
	ch := byte(c)
	for ; n != 0 && s.Get() != ch; s, n = s.Add(1), n-1 {
	}
	if n != 0 {
		return s
	}
	return Ptr[byte]{}
}

func musl_fmtbase(spec byte) int32 {
	if spec == 'o' {
		return 8
	}
	if spec == 'x' || spec == 'X' {
		return 16
	}
	return 10
}

func musl_fmtnum(dest Ptr[byte], v uint64, base int32, upper int32, isneg int32) int32 {
	var digits [24]byte
	var n, out int32
	var d int32

	if isneg != 0 {
		v = ^v + 1
	}
	for {
		d = int32(v % uint64(base))
		if d < 10 {
			digits[n] = byte('0' + d)
		} else if upper != 0 {
			digits[n] = byte('A' + d - 10)
		} else {
			digits[n] = byte('a' + d - 10)
		}
		n++
		v /= uint64(base)
		if v == 0 {
			break
		}
	}
	if isneg != 0 {
		dest.Set(int(out), '-')
		out++
	}
	for i := int32(0); i < n; i++ {
		dest.Set(int(out), digits[n-1-i])
		out++
	}
	dest.Set(int(out), 0)
	return out
}

func musl_fmtptr(dest Ptr[byte], v uint64) int32 {
	dest.Set(0, '0')
	dest.Set(1, 'x')
	for i := 0; i < 16; i++ {
		d := int32((v >> (60 - 4*uint(i))) & 0xf)
		if d < 10 {
			dest.Set(2+i, byte('0'+d))
		} else {
			dest.Set(2+i, byte('a'+d-10))
		}
	}
	dest.Set(18, 0)
	return 18
}

func vim_snprintf(p0 Ptr[byte], p1 usize, p2 Ptr[byte], args ...any) int32 {
	return vim_vsnprintf_typval(p0, p1, p2, args)
}

func format_typeof(type_ Ptr[byte]) int32 {
	var length_modifier byte
	var fmt_spec byte

	if type_.Get() == 'h' || type_.Get() == 'l' {
		length_modifier = type_.Get()
		type_ = type_.Add(1)
		if length_modifier == 'l' && type_.Get() == 'l' {
			length_modifier = 'L'
			type_ = type_.Add(1)
		}
	}
	fmt_spec = type_.Get()

	switch fmt_spec {
	case 'i':
		fmt_spec = 'd'
	case '*':
		fmt_spec = 'd'
		length_modifier = 'h'
	case 'D':
		fmt_spec = 'd'
		length_modifier = 'l'
	case 'U':
		fmt_spec = 'u'
		length_modifier = 'l'
	case 'O':
		fmt_spec = 'o'
		length_modifier = 'l'
	}

	switch fmt_spec {
	case '%':
		return TYPE_PERCENT
	case 'c':
		return TYPE_CHAR
	case 's', 'S':
		return TYPE_STRING
	case 'd', 'u', 'b', 'B', 'o', 'x', 'X', 'p':
		if fmt_spec == 'p' {
			return TYPE_POINTER
		} else if fmt_spec == 'b' || fmt_spec == 'B' {
			return TYPE_UNSIGNEDLONGLONGINT
		} else if fmt_spec == 'd' {
			switch length_modifier {
			case 0, 'h':
				return TYPE_INT
			case 'l':
				return TYPE_LONGINT
			case 'L':
				return TYPE_LONGLONGINT
			}
		} else {
			switch length_modifier {
			case 0, 'h':
				return TYPE_UNSIGNEDINT
			case 'l':
				return TYPE_UNSIGNEDLONGINT
			case 'L':
				return TYPE_UNSIGNEDLONGLONGINT
			}
		}
	}
	return TYPE_UNKNOWN
}

func format_typename(type_ Ptr[byte]) Ptr[byte] {
	switch format_typeof(type_) {
	case TYPE_INT:
		return typename_int
	case TYPE_LONGINT:
		return typename_longint
	case TYPE_LONGLONGINT:
		return typename_longlongint
	case TYPE_UNSIGNEDINT:
		return typename_unsignedint
	case TYPE_UNSIGNEDLONGINT:
		return typename_unsignedlongint
	case TYPE_UNSIGNEDLONGLONGINT:
		return typename_unsignedlonglongint
	case TYPE_POINTER:
		return gettext_(typename_pointer)
	case TYPE_PERCENT:
		return gettext_(typename_percent)
	case TYPE_CHAR:
		return typename_char
	case TYPE_STRING:
		return gettext_(typename_string)
	}
	return gettext_(typename_unknown)
}

// hostFmtError is the vim_snprintf-into-IObuff-then-emsg pair every format
// error makes.
func hostFmtError(msg Ptr[byte], args ...any) {
	vim_snprintf(IObuff, emsg_iobuff_room(), gettext_(msg), args...)
	emsg(iobuff_or(gettext_(msg)))
}

// adjust_types: ap_types is a C array of `const char *` into fmt; here a Go
// slice of them, nil for the C NULL array, Ptr{} for a NULL entry.
func adjust_types(ap_types *[]Ptr[byte], arg int32, num_posarg *int32, type_ Ptr[byte]) int32 {
	if arg <= 0 {
		hostFmtError(e_invalid_format_specifier_str, type_)
		return FAIL
	}

	if *ap_types == nil || *num_posarg < arg {
		new_types := make([]Ptr[byte], arg)
		if *ap_types != nil {
			copy(new_types, (*ap_types)[:*num_posarg])
		}
		for idx := *num_posarg; idx < arg; idx++ {
			new_types[idx] = Ptr[byte]{}
		}
		*ap_types = new_types
		*num_posarg = arg
	}

	if !(*ap_types)[arg-1].Nil() {
		if (*ap_types)[arg-1].At(0) == '*' || type_.At(0) == '*' {
			pt := type_
			if pt.At(0) == '*' {
				pt = (*ap_types)[arg-1]
			}
			if pt.At(0) != '*' {
				switch pt.At(0) {
				case 'd', 'i':
				default:
					vim_snprintf(IObuff, emsg_iobuff_room(), gettext_(e_positional_num_field_spec_reused_str_str), arg, format_typename((*ap_types)[arg-1]), format_typename(type_))
					emsg(iobuff_or(gettext_(e_positional_num_field_spec_reused_str_str)))
					return FAIL
				}
			}
		} else {
			if format_typeof(type_) != format_typeof((*ap_types)[arg-1]) {
				vim_snprintf(IObuff, emsg_iobuff_room(), gettext_(e_positional_arg_num_type_inconsistent_str_str), arg, format_typename(type_), format_typename((*ap_types)[arg-1]))
				emsg(iobuff_or(gettext_(e_positional_arg_num_type_inconsistent_str_str)))
				return FAIL
			}
		}
	}

	(*ap_types)[arg-1] = type_
	return OK
}

func format_overflow_error(pstart Ptr[byte]) {
	p := pstart
	for hostIsDigit(p.Get()) {
		p = p.Add(1)
	}
	arglen := usize(p.Sub(pstart))
	argcopy := Alloc(int(arglen + 1))
	Memmove(argcopy, pstart, int(arglen))
	vim_snprintf(IObuff, emsg_iobuff_room(), gettext_(e_val_too_large), argcopy)
	emsg(iobuff_or(gettext_(e_val_too_large)))
}

func get_unsigned_int(pstart Ptr[byte], p *Ptr[byte], uj *uint32, overflow_err int32) int32 {
	*uj = uint32(p.Get()) - '0'
	*p = p.Add(1)

	for hostIsDigit(p.Get()) && *uj < MAX_ALLOWED_STRING_WIDTH {
		*uj = 10**uj + uint32(p.Get()-'0')
		*p = p.Add(1)
	}

	if *uj > MAX_ALLOWED_STRING_WIDTH {
		if overflow_err != 0 {
			format_overflow_error(pstart)
			return FAIL
		}
		*uj = MAX_ALLOWED_STRING_WIDTH
	}
	return OK
}

// hostTvs is `tvs != nullptr`: always false in this program.
const hostTvs = 0

func parse_fmt_types(ap_types *[]Ptr[byte], num_posarg *int32, fmt Ptr[byte]) int32 {
	p := fmt
	var arg Ptr[byte]
	var any_pos, any_arg int32

	fail := func() int32 {
		*ap_types = nil
		*num_posarg = 0
		return FAIL
	}
	// CHECK_POS_ARG
	mixed := func() bool {
		if any_pos != 0 && any_arg != 0 {
			hostFmtError(e_cannot_mix_positional_and_non_positional_str, fmt)
			return true
		}
		return false
	}

	if p.Nil() {
		return OK
	}

	for p.Get() != NUL {
		if p.Get() != '%' {
			q := hostStrchr(p.Add(1), '%')
			var n usize
			if q.Nil() {
				n = hostStrlen(p)
			} else {
				n = usize(q.Sub(p))
			}
			p = p.Add(int(n))
		} else {
			var length_modifier byte
			var pos_arg int32 = -1
			var ptype Ptr[byte]
			pstart := p.Add(1)

			p = p.Add(1)
			ptype = p

			for hostIsDigit(ptype.Get()) {
				ptype = ptype.Add(1)
			}

			if ptype.Get() == '$' {
				if p.Get() == '0' {
					hostFmtError(e_invalid_format_specifier_str, fmt)
					return fail()
				}
				var uj uint32
				if get_unsigned_int(pstart, &p, &uj, hostTvs) == FAIL {
					return fail()
				}
				pos_arg = int32(uj)
				any_pos = 1
				if mixed() {
					return fail()
				}
				p = p.Add(1)
			}

			for p.Get() == '0' || p.Get() == '-' || p.Get() == '+' || p.Get() == ' ' || p.Get() == '#' || p.Get() == '\'' {
				p = p.Add(1)
			}

			arg = p
			if arg.Get() == '*' {
				p = p.Add(1)
				if hostIsDigit(p.Get()) {
					var uj uint32
					if get_unsigned_int(arg.Add(1), &p, &uj, hostTvs) == FAIL {
						return fail()
					}
					if p.Get() != '$' {
						hostFmtError(e_invalid_format_specifier_str, fmt)
						return fail()
					}
					p = p.Add(1)
					any_pos = 1
					if mixed() {
						return fail()
					}
					if adjust_types(ap_types, int32(uj), num_posarg, arg) == FAIL {
						return fail()
					}
				} else {
					any_arg = 1
					if mixed() {
						return fail()
					}
				}
			} else if hostIsDigit(p.Get()) {
				digstart := p
				var uj uint32
				if get_unsigned_int(digstart, &p, &uj, hostTvs) == FAIL {
					return fail()
				}
				if p.Get() == '$' {
					hostFmtError(e_invalid_format_specifier_str, fmt)
					return fail()
				}
			}

			if p.Get() == '.' {
				p = p.Add(1)
				arg = p
				if arg.Get() == '*' {
					p = p.Add(1)
					if hostIsDigit(p.Get()) {
						var uj uint32
						if get_unsigned_int(arg.Add(1), &p, &uj, hostTvs) == FAIL {
							return fail()
						}
						if p.Get() == '$' {
							any_pos = 1
							if mixed() {
								return fail()
							}
							p = p.Add(1)
							if adjust_types(ap_types, int32(uj), num_posarg, arg) == FAIL {
								return fail()
							}
						} else {
							hostFmtError(e_invalid_format_specifier_str, fmt)
							return fail()
						}
					} else {
						any_arg = 1
						if mixed() {
							return fail()
						}
					}
				} else if hostIsDigit(p.Get()) {
					digstart := p
					var uj uint32
					if get_unsigned_int(digstart, &p, &uj, hostTvs) == FAIL {
						return fail()
					}
					if p.Get() == '$' {
						hostFmtError(e_invalid_format_specifier_str, fmt)
						return fail()
					}
				}
			}

			if pos_arg != -1 {
				any_pos = 1
				if mixed() {
					return fail()
				}
				ptype = p
			}

			if p.Get() == 'h' || p.Get() == 'l' {
				length_modifier = p.Get()
				p = p.Add(1)
				if length_modifier == 'l' && p.Get() == 'l' {
					p = p.Add(1)
				}
			}

			switch p.Get() {
			case 'i', '*', 'd', 'u', 'o', 'D', 'U', 'O', 'x', 'X', 'b', 'B', 'c', 's', 'S', 'p':
				if pos_arg != -1 {
					if adjust_types(ap_types, pos_arg, num_posarg, ptype) == FAIL {
						return fail()
					}
				} else {
					any_arg = 1
					if mixed() {
						return fail()
					}
				}
			default:
				if pos_arg != -1 {
					hostFmtError(e_cannot_mix_positional_and_non_positional_str, fmt)
					return fail()
				}
			}

			if p.Get() != NUL {
				p = p.Add(1)
			}
		}
	}

	for arg_idx := int32(0); arg_idx < *num_posarg; arg_idx++ {
		if (*ap_types)[arg_idx].Nil() {
			hostFmtError(e_fmt_arg_nr_unused_str, arg_idx+1, fmt)
			return fail()
		}
	}
	return OK
}

func skip_to_arg(ap_types []Ptr[byte], ap *hostVa, arg_idx *int32, arg_cur *int32, fmt Ptr[byte]) {
	var arg_min int32

	if *arg_cur+1 == *arg_idx {
		*arg_cur++
		*arg_idx++
		return
	}

	if *arg_cur >= *arg_idx {
		ap.i = 0 // va_end(*ap); va_copy(*ap, ap_start)
	} else {
		arg_min = *arg_cur
	}

	for *arg_cur = arg_min; *arg_cur < *arg_idx-1; *arg_cur++ {
		// DEVIATION (bounds): C would read past the end of ap_types here;
		// an index past it is treated as a NULL entry.
		if ap_types == nil || int(*arg_cur) >= len(ap_types) || ap_types[*arg_cur].Nil() {
			vim_snprintf(IObuff, emsg_iobuff_room(), e_aptypes_is_null_nr_str, *arg_cur, fmt)
			iemsg(iobuff_or(e_aptypes_is_null_nr_str))
			return
		}
		p := ap_types[*arg_cur]
		switch format_typeof(p) {
		case TYPE_PERCENT, TYPE_UNKNOWN:
		default:
			// every other type is one va_arg, whatever its width
			ap.vaSkip()
		}
	}

	*arg_cur++
	*arg_idx++
}

func vim_vsnprintf_typval(str Ptr[byte], str_m usize, fmt Ptr[byte], args []any) int32 {
	var str_l usize
	p := fmt
	var arg_cur int32
	var num_posarg int32
	var arg_idx int32 = 1
	var ap_types []Ptr[byte]

	if parse_fmt_types(&ap_types, &num_posarg, fmt) == FAIL {
		return 0
	}

	ap := &hostVa{args: args}

	if p.Nil() {
		p = S("")
	}
	for p.Get() != NUL {
		if p.Get() != '%' {
			q := hostStrchr(p.Add(1), '%')
			var n usize
			if q.Nil() {
				n = hostStrlen(p)
			} else {
				n = usize(q.Sub(p))
			}
			if str_l < str_m {
				avail := str_m - str_l
				m := n
				if n > avail {
					m = avail
				}
				Memmove(str.Add(int(str_l)), p, int(m))
			}
			p = p.Add(int(n))
			str_l += n
			continue
		}

		var min_field_width usize
		var precision usize
		var zero_padding int32
		var precision_specified int32
		var justify_left int32
		var alternate_form int32
		var force_sign int32
		var space_for_positive int32 = 1
		var length_modifier byte
		var str_arg Ptr[byte]
		var str_arg_l usize
		var number_of_zeros_to_pad usize
		var zero_padding_insertion_ind usize
		var fmt_spec byte
		var pos_arg int32 = -1
		var ptype Ptr[byte]

		p = p.Add(1)
		ptype = p
		for hostIsDigit(ptype.Get()) {
			ptype = ptype.Add(1)
		}

		if ptype.Get() == '$' {
			digstart := p
			var uj uint32
			if get_unsigned_int(digstart, &p, &uj, hostTvs) == FAIL {
				return int32(str_l)
			}
			pos_arg = int32(uj)
			p = p.Add(1)
		}

		for p.Get() == '0' || p.Get() == '-' || p.Get() == '+' || p.Get() == ' ' || p.Get() == '#' || p.Get() == '\'' {
			switch p.Get() {
			case '0':
				zero_padding = 1
			case '-':
				justify_left = 1
			case '+':
				force_sign = 1
				space_for_positive = 0
			case ' ':
				force_sign = 1
			case '#':
				alternate_form = 1
			case '\'':
			}
			p = p.Add(1)
		}

		if p.Get() == '*' {
			var j int32
			digstart := p.Add(1)
			p = p.Add(1)
			if hostIsDigit(p.Get()) {
				var uj uint32
				if get_unsigned_int(digstart, &p, &uj, hostTvs) == FAIL {
					return int32(str_l)
				}
				arg_idx = int32(uj)
				p = p.Add(1)
			}
			skip_to_arg(ap_types, ap, &arg_idx, &arg_cur, fmt)
			j = ap.vaInt()
			if j > MAX_ALLOWED_STRING_WIDTH {
				j = MAX_ALLOWED_STRING_WIDTH
			}
			if j >= 0 {
				min_field_width = usize(j)
			} else {
				min_field_width = usize(-j)
				justify_left = 1
			}
		} else if hostIsDigit(p.Get()) {
			digstart := p
			var uj uint32
			if get_unsigned_int(digstart, &p, &uj, hostTvs) == FAIL {
				return int32(str_l)
			}
			min_field_width = usize(uj)
		}

		if p.Get() == '.' {
			p = p.Add(1)
			precision_specified = 1
			if hostIsDigit(p.Get()) {
				digstart := p
				var uj uint32
				if get_unsigned_int(digstart, &p, &uj, hostTvs) == FAIL {
					return int32(str_l)
				}
				precision = usize(uj)
			} else if p.Get() == '*' {
				var j int32
				digstart := p
				p = p.Add(1)
				if hostIsDigit(p.Get()) {
					var uj uint32
					if get_unsigned_int(digstart, &p, &uj, hostTvs) == FAIL {
						return int32(str_l)
					}
					arg_idx = int32(uj)
					p = p.Add(1)
				}
				skip_to_arg(ap_types, ap, &arg_idx, &arg_cur, fmt)
				j = ap.vaInt()
				if j > MAX_ALLOWED_STRING_WIDTH {
					j = MAX_ALLOWED_STRING_WIDTH
				}
				if j >= 0 {
					precision = usize(j)
				} else {
					precision_specified = 0
					precision = 0
				}
			}
		}

		if p.Get() == 'h' || p.Get() == 'l' {
			length_modifier = p.Get()
			p = p.Add(1)
			if length_modifier == 'l' && p.Get() == 'l' {
				length_modifier = 'L'
				p = p.Add(1)
			}
		}
		fmt_spec = p.Get()

		switch fmt_spec {
		case 'i':
			fmt_spec = 'd'
		case 'D':
			fmt_spec = 'd'
			length_modifier = 'l'
		case 'U':
			fmt_spec = 'u'
			length_modifier = 'l'
		case 'O':
			fmt_spec = 'o'
			length_modifier = 'l'
		}

		if pos_arg != -1 {
			arg_idx = pos_arg
		}

		switch fmt_spec {
		case '%', 'c', 's', 'S':
			str_arg_l = 1
			switch fmt_spec {
			case '%':
				str_arg = p
			case 'c':
				skip_to_arg(ap_types, ap, &arg_idx, &arg_cur, fmt)
				j := ap.vaInt()
				uchar_arg := Mk[byte](1)
				uchar_arg.Put(byte(j))
				str_arg = uchar_arg
			case 's', 'S':
				skip_to_arg(ap_types, ap, &arg_idx, &arg_cur, fmt)
				str_arg = ap.vaString()
				if str_arg.Nil() {
					str_arg = S("[NULL]")
					str_arg_l = 6
				} else if precision_specified == 0 {
					str_arg_l = hostStrlen(str_arg)
				} else if precision == 0 {
					str_arg_l = 0
				} else {
					lim := precision
					if lim > 0x7fffffff {
						lim = 0x7fffffff
					}
					q := musl_memchr(str_arg, 0, lim)
					if q.Nil() {
						str_arg_l = precision
					} else {
						str_arg_l = usize(q.Sub(str_arg))
					}
				}
				if fmt_spec == 'S' {
					var i usize
					var cell int32
					p1 := str_arg
					for ; p1.Get() != 0; p1 = p1.Add(int(utfc_ptr2len(p1))) {
						cell = utf_ptr2cells(p1)
						if precision_specified != 0 && i+usize(cell) > precision {
							break
						}
						i += usize(cell)
					}
					str_arg_l = usize(p1.Sub(str_arg))
					if min_field_width != 0 {
						min_field_width += str_arg_l - i
					}
				}
			}

		case 'd', 'u', 'b', 'B', 'o', 'x', 'X', 'p':
			var arg_sign int32
			var int_arg int32
			var uint_arg uint32
			var long_arg int64
			var ulong_arg uint64
			var llong_arg varnumber_T
			var ullong_arg uvarnumber_T
			var bin_arg uvarnumber_T
			var ptr_arg uint64
			tmp := Mk[byte](TMP_LEN)

			if fmt_spec == 'p' {
				length_modifier = 0
				skip_to_arg(ap_types, ap, &arg_idx, &arg_cur, fmt)
				ptr_arg = ap.vaPointer()
				if ptr_arg != 0 {
					arg_sign = 1
				}
			} else if fmt_spec == 'b' || fmt_spec == 'B' {
				skip_to_arg(ap_types, ap, &arg_idx, &arg_cur, fmt)
				bin_arg = ap.vaUlong()
				if bin_arg != 0 {
					arg_sign = 1
				}
			} else if fmt_spec == 'd' {
				switch length_modifier {
				case 0, 'h':
					skip_to_arg(ap_types, ap, &arg_idx, &arg_cur, fmt)
					int_arg = ap.vaInt()
					if int_arg > 0 {
						arg_sign = 1
					} else if int_arg < 0 {
						arg_sign = -1
					}
				case 'l':
					skip_to_arg(ap_types, ap, &arg_idx, &arg_cur, fmt)
					long_arg = ap.vaLong()
					if long_arg > 0 {
						arg_sign = 1
					} else if long_arg < 0 {
						arg_sign = -1
					}
				case 'L':
					skip_to_arg(ap_types, ap, &arg_idx, &arg_cur, fmt)
					llong_arg = ap.vaLong()
					if llong_arg > 0 {
						arg_sign = 1
					} else if llong_arg < 0 {
						arg_sign = -1
					}
				}
			} else {
				switch length_modifier {
				case 0, 'h':
					skip_to_arg(ap_types, ap, &arg_idx, &arg_cur, fmt)
					uint_arg = ap.vaUint()
					if uint_arg != 0 {
						arg_sign = 1
					}
				case 'l':
					skip_to_arg(ap_types, ap, &arg_idx, &arg_cur, fmt)
					ulong_arg = ap.vaUlong()
					if ulong_arg != 0 {
						arg_sign = 1
					}
				case 'L':
					skip_to_arg(ap_types, ap, &arg_idx, &arg_cur, fmt)
					ullong_arg = ap.vaUlong()
					if ullong_arg != 0 {
						arg_sign = 1
					}
				}
			}

			str_arg = tmp
			str_arg_l = 0

			if precision_specified != 0 {
				zero_padding = 0
			}
			if fmt_spec == 'd' {
				if force_sign != 0 && arg_sign >= 0 {
					if space_for_positive != 0 {
						tmp.Set(int(str_arg_l), ' ')
					} else {
						tmp.Set(int(str_arg_l), '+')
					}
					str_arg_l++
				}
			} else if alternate_form != 0 {
				if arg_sign != 0 && (fmt_spec == 'b' || fmt_spec == 'B' || fmt_spec == 'x' || fmt_spec == 'X') {
					tmp.Set(int(str_arg_l), '0')
					str_arg_l++
					tmp.Set(int(str_arg_l), fmt_spec)
					str_arg_l++
				}
			}

			zero_padding_insertion_ind = str_arg_l
			if precision_specified == 0 {
				precision = 1
			}
			if precision == 0 && arg_sign == 0 {
				// the C leaves this branch empty: no digits for a zero
			} else {
				at := tmp.Add(int(str_arg_l))
				if fmt_spec == 'p' {
					str_arg_l += usize(musl_fmtptr(at, ptr_arg))
				} else if fmt_spec == 'b' || fmt_spec == 'B' {
					var b [64]byte
					var b_l usize
					bn := bin_arg
					for {
						b_l++
						b[usize(len(b))-b_l] = byte('0' + (bn & 0x1))
						bn >>= 1
						if bn == 0 {
							break
						}
					}
					for k := usize(0); k < b_l; k++ {
						at.Set(int(k), b[usize(len(b))-b_l+k])
					}
					str_arg_l += b_l
				} else if fmt_spec == 'd' {
					switch length_modifier {
					case 0:
						str_arg_l += usize(musl_fmtnum(at, uint64(int64(int_arg)), 10, 0, B2i(int_arg < 0)))
					case 'h':
						str_arg_l += usize(musl_fmtnum(at, uint64(int64(int16(int_arg))), 10, 0, B2i(int16(int_arg) < 0)))
					case 'l':
						str_arg_l += usize(musl_fmtnum(at, uint64(long_arg), 10, 0, B2i(long_arg < 0)))
					case 'L':
						str_arg_l += usize(musl_fmtnum(at, uint64(llong_arg), 10, 0, B2i(llong_arg < 0)))
					}
				} else {
					switch length_modifier {
					case 0:
						str_arg_l += usize(musl_fmtnum(at, uint64(uint_arg), musl_fmtbase(fmt_spec), B2i(fmt_spec == 'X'), 0))
					case 'h':
						str_arg_l += usize(musl_fmtnum(at, uint64(uint16(uint_arg)), musl_fmtbase(fmt_spec), B2i(fmt_spec == 'X'), 0))
					case 'l':
						str_arg_l += usize(musl_fmtnum(at, ulong_arg, musl_fmtbase(fmt_spec), B2i(fmt_spec == 'X'), 0))
					case 'L':
						str_arg_l += usize(musl_fmtnum(at, ullong_arg, musl_fmtbase(fmt_spec), B2i(fmt_spec == 'X'), 0))
					}
				}

				if zero_padding_insertion_ind < str_arg_l && tmp.At(int(zero_padding_insertion_ind)) == '-' {
					zero_padding_insertion_ind++
				}
				if zero_padding_insertion_ind+1 < str_arg_l && tmp.At(int(zero_padding_insertion_ind)) == '0' && (tmp.At(int(zero_padding_insertion_ind+1)) == 'x' || tmp.At(int(zero_padding_insertion_ind+1)) == 'X') {
					zero_padding_insertion_ind += 2
				}
			}

			{
				num_of_digits := str_arg_l - zero_padding_insertion_ind
				if alternate_form != 0 && fmt_spec == 'o' && !(zero_padding_insertion_ind < str_arg_l && tmp.At(int(zero_padding_insertion_ind)) == '0') {
					if precision_specified == 0 || precision < num_of_digits+1 {
						precision = num_of_digits + 1
					}
				}
				if num_of_digits < precision {
					number_of_zeros_to_pad = precision - num_of_digits
				}
			}
			if justify_left == 0 && zero_padding != 0 {
				n := int32(min_field_width - (str_arg_l + number_of_zeros_to_pad))
				if n > 0 {
					number_of_zeros_to_pad += usize(n)
				}
			}

		default:
			zero_padding = 0
			justify_left = 1
			min_field_width = 0
			str_arg = p
			str_arg_l = 0
			if p.Get() != NUL {
				str_arg_l++
			}
		}

		if p.Get() != NUL {
			p = p.Add(1)
		}

		if justify_left == 0 {
			pn := int32(min_field_width - (str_arg_l + number_of_zeros_to_pad))
			if pn > 0 {
				if str_l < str_m {
					avail := str_m - str_l
					m := usize(pn)
					if m > avail {
						m = avail
					}
					c := int32(' ')
					if zero_padding != 0 {
						c = '0'
					}
					Memset(str.Add(int(str_l)), c, int(m))
				}
				str_l += usize(pn)
			}
		}

		if number_of_zeros_to_pad == 0 {
			zero_padding_insertion_ind = 0
		} else {
			zn := int32(zero_padding_insertion_ind)
			if zn > 0 {
				if str_l < str_m {
					avail := str_m - str_l
					m := usize(zn)
					if m > avail {
						m = avail
					}
					Memmove(str.Add(int(str_l)), str_arg, int(m))
				}
				str_l += usize(zn)
			}
			zn = int32(number_of_zeros_to_pad)
			if zn > 0 {
				if str_l < str_m {
					avail := str_m - str_l
					m := usize(zn)
					if m > avail {
						m = avail
					}
					Memset(str.Add(int(str_l)), '0', int(m))
				}
				str_l += usize(zn)
			}
		}

		{
			sn := int32(str_arg_l - zero_padding_insertion_ind)
			if sn > 0 {
				if str_l < str_m {
					avail := str_m - str_l
					m := usize(sn)
					if m > avail {
						m = avail
					}
					Memmove(str.Add(int(str_l)), str_arg.Add(int(zero_padding_insertion_ind)), int(m))
				}
				str_l += usize(sn)
			}
		}

		if justify_left != 0 {
			pn := int32(min_field_width - (str_arg_l + number_of_zeros_to_pad))
			if pn > 0 {
				if str_l < str_m {
					avail := str_m - str_l
					m := usize(pn)
					if m > avail {
						m = avail
					}
					Memset(str.Add(int(str_l)), ' ', int(m))
				}
				str_l += usize(pn)
			}
		}
	}

	if str_m > 0 {
		k := str_m - 1
		if str_l <= str_m-1 {
			k = str_l
		}
		str.Set(int(k), NUL)
	}

	return int32(str_l)
}

// ---------------------------------------------------------------------------
// The terminal, the clock, input and the signals.
// ---------------------------------------------------------------------------

// XTABS (TABDLY) is not in package syscall; Linux's value.
const hostXTABS = 0x1800

var (
	host_winch_pending atomic.Bool
	host_tstp_pending  atomic.Bool
	host_int_pending   atomic.Bool
	host_tty_saved     syscall.Termios
	host_tty_valid     int32
	host_tty_raw       int32
	host_now_base      int64
	host_now_based     int32

	// the Go replacements for asynchronous handlers
	host_caught   bool           // musl_host_init has run
	host_sigch    chan os.Signal // signals the host catches
	host_wake_r   = -1           // the read end of the wake-up pipe
	host_wake_w   = -1           // its write end
	host_dying_mu sync.Mutex
	host_dying    []int32 // SIGHUP/SIGTERM received, for deathtrap
)

func host_ioctl(fd int, req uintptr, arg unsafe.Pointer) syscall.Errno {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(arg))
	return e
}

func host_tcgetattr(fd int, t *syscall.Termios) bool {
	return host_ioctl(fd, syscall.TCGETS, unsafe.Pointer(t)) == 0
}

func host_tty_set(raw int32, sleep int32) {
	var tnew syscall.Termios
	n := 10

	if host_tty_valid == 0 {
		if !host_tcgetattr(0, &host_tty_saved) {
			return
		}
		host_tty_valid = TRUE
	}
	tnew = host_tty_saved
	if raw != 0 {
		tnew.Iflag &^= syscall.ICRNL | syscall.IXON
		tnew.Lflag &^= syscall.ICANON | syscall.ECHO | syscall.ISIG | syscall.ECHOE | syscall.IEXTEN
		tnew.Oflag &^= syscall.ONLCR | hostXTABS
		tnew.Cc[syscall.VMIN] = 1
		tnew.Cc[syscall.VTIME] = 0
	} else if sleep != 0 {
		tnew.Lflag &^= syscall.ICANON | syscall.ECHO
		tnew.Cc[syscall.VMIN] = 1
		tnew.Cc[syscall.VTIME] = 0
	}
	for {
		e := host_ioctl(0, syscall.TCSETS, unsafe.Pointer(&tnew)) // tcsetattr(0, TCSANOW)
		if !(e != 0 && e == syscall.EINTR && n > 0) {
			break
		}
		n--
	}
}

// host_signals is the goroutine that stands for the C handlers: it sets the
// flag a handler sets (or queues SIGHUP/SIGTERM for deathtrap), then wakes a
// host select that is waiting, as the signal's EINTR would.
func host_signals() {
	for s := range host_sigch {
		sig := s.(syscall.Signal)
		switch sig {
		case syscall.SIGWINCH, syscall.SIGCONT:
			host_winch_pending.Store(true)
		case syscall.SIGTSTP:
			host_tstp_pending.Store(true)
		case syscall.SIGINT:
			host_int_pending.Store(true)
		case syscall.SIGHUP, syscall.SIGTERM:
			host_dying_mu.Lock()
			host_dying = append(host_dying, int32(sig))
			host_dying_mu.Unlock()
		}
		if host_wake_w >= 0 {
			syscall.Write(host_wake_w, []byte{0})
		}
	}
}

// host_drain empties the wake-up pipe: the flags, not the pipe, say what
// arrived; the pipe only ends a wait.  Drained before the flags are read, so
// a signal after the read still wakes the wait that follows.
func host_drain() {
	if host_wake_r < 0 {
		return
	}
	var b [64]byte
	for {
		n, err := syscall.Read(host_wake_r, b[:])
		if n <= 0 || err != nil {
			return
		}
	}
}

// host_deliver runs deathtrap for each SIGHUP/SIGTERM received, on the main
// goroutine -- where the C handler would have run at the signal.
func host_deliver() {
	for {
		host_dying_mu.Lock()
		if len(host_dying) == 0 {
			host_dying_mu.Unlock()
			return
		}
		sig := host_dying[0]
		host_dying = host_dying[1:]
		host_dying_mu.Unlock()
		deathtrap(sig)
	}
}

func hostFdSet(s *syscall.FdSet, fd int) {
	s.Bits[fd/64] |= 1 << (uint(fd) % 64)
}

func hostFdIsSet(s *syscall.FdSet, fd int) bool {
	return s.Bits[fd/64]&(1<<(uint(fd)%64)) != 0
}

func musl_host_init() {
	if host_sigch == nil {
		var fds [2]int
		if syscall.Pipe2(fds[:], syscall.O_NONBLOCK|syscall.O_CLOEXEC) == nil {
			host_wake_r, host_wake_w = fds[0], fds[1]
		}
		host_sigch = make(chan os.Signal, 64)
		go host_signals()
	}
	signal.Notify(host_sigch, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGWINCH,
		syscall.SIGCONT, syscall.SIGTSTP, syscall.SIGINT)
	signal.Ignore(syscall.SIGPIPE, syscall.SIGALRM)
	host_caught = true
}

func musl_get_winsize(rows *int32, cols *int32) int32 {
	var ws struct{ row, col, xpixel, ypixel uint16 }

	if host_ioctl(1, syscall.TIOCGWINSZ, unsafe.Pointer(&ws)) != 0 {
		return FAIL
	}
	if ws.row <= 0 || ws.col <= 0 {
		return FAIL
	}
	*rows = int32(ws.row)
	*cols = int32(ws.col)
	return OK
}

func musl_term_start() {
	host_tty_raw = TRUE
	host_tty_set(TRUE, FALSE)
}

func musl_term_stop() {
	host_tty_raw = FALSE
	host_tty_set(FALSE, FALSE)
}

func musl_tty_keys(fd int32, bs *int32, intr *int32, cr *int32, nlcr *int32) int32 {
	var keys syscall.Termios

	if !host_tcgetattr(int(fd), &keys) {
		return FAIL
	}
	*bs = int32(keys.Cc[syscall.VERASE])
	*intr = int32(keys.Cc[syscall.VINTR])
	*cr = B2i(keys.Iflag&syscall.ICRNL != 0)
	*nlcr = B2i(keys.Oflag&syscall.ONLCR != 0)
	return OK
}

func musl_now_ms() int64 {
	t := time.Now()
	sec := t.Unix()
	usec := int64(t.Nanosecond() / 1000)
	if host_now_based == 0 {
		host_now_based = TRUE
		host_now_base = sec
	}
	return (sec-host_now_base)*1000 + usec/1000
}

func host_time() int64 {
	return time.Now().Unix()
}

// host_sleep is nanosleep(ms): it ends early when a caught signal arrives, as
// nanosleep does with EINTR.  A negative time does not sleep (nanosleep's
// EINVAL).
func host_sleep(ms int64) {
	if ms < 0 {
		return
	}
	if host_wake_r < 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return
	}
	host_drain()
	tv := syscall.Timeval{Sec: ms / 1000, Usec: (ms % 1000) * 1000}
	for {
		var r syscall.FdSet
		hostFdSet(&r, host_wake_r)
		// Linux's select leaves the time remaining in tv, so an EINTR from
		// the Go runtime's own signals resumes the same sleep.
		_, err := syscall.Select(host_wake_r+1, &r, nil, nil, &tv)
		if err == syscall.EINTR {
			continue
		}
		return
	}
}

func musl_delay(ms int64, interruptible int32) {
	relax := interruptible != 0 && host_tty_raw != 0 && ms > 500

	host_deliver()
	if relax {
		host_tty_set(FALSE, TRUE)
	}
	host_sleep(ms)
	if relax {
		host_tty_set(TRUE, FALSE)
	}
	host_deliver()
}

func musl_wait_for_input(ms int64) int32 {
	var tv syscall.Timeval
	var tvp *syscall.Timeval

	if ms >= 0 {
		tv.Sec = ms / 1000
		tv.Usec = (ms % 1000) * 1000
		tvp = &tv
	}
	for {
		host_drain()
		host_deliver()
		if host_winch_pending.Load() || host_tstp_pending.Load() || host_int_pending.Load() {
			return 1
		}
		var rfds syscall.FdSet
		nfd := 1
		hostFdSet(&rfds, 0)
		if host_wake_r >= 0 {
			hostFdSet(&rfds, host_wake_r)
			nfd = host_wake_r + 1
		}
		ret, err := syscall.Select(nfd, &rfds, nil, nil, tvp)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			return 0
		}
		if ret > 0 && hostFdIsSet(&rfds, 0) {
			return 1
		}
		if ret > 0 {
			// only the wake-up pipe: the C select returned EINTR
			continue
		}
		return 0
	}
}

func musl_read_input(buf Ptr[byte], len_ int32) int32 {
	host_drain()
	host_deliver()
	if host_int_pending.Load() {
		host_int_pending.Store(false)
		if len_ >= 1 {
			buf.Set(0, 3)
			return 1
		}
	}
	if host_winch_pending.Load() {
		var rows, cols int32

		host_winch_pending.Store(false)
		if musl_get_winsize(&rows, &cols) == OK && len_ >= 32 {
			return vim_snprintf(buf, usize(len_), S("\033[48;%d;%d;0;0t"), rows, cols)
		}
	}
	if host_tstp_pending.Load() {
		host_tstp_pending.Store(false)
		if len_ >= 5 {
			Memmove(buf, S("\033[?1z"), 5)
			return 5
		}
	}
	if len_ < 0 {
		return -1 // read(2) of (usize)len: EFAULT
	}
	if len_ == 0 {
		return 0
	}
	// DEVIATION: read(0) in the C is interrupted by a caught signal and
	// returns -1 (EINTR).  A Go read would not be, so wait first for either
	// input or the wake-up pipe, and return -1 when the pipe won.
	if host_wake_r >= 0 {
		for {
			var rfds syscall.FdSet
			hostFdSet(&rfds, 0)
			hostFdSet(&rfds, host_wake_r)
			ret, err := syscall.Select(host_wake_r+1, &rfds, nil, nil, nil)
			if err == syscall.EINTR {
				continue
			}
			if err == nil && ret > 0 && !hostFdIsSet(&rfds, 0) {
				host_drain()
				host_deliver()
				return -1
			}
			break
		}
	}
	for {
		n, err := syscall.Read(0, buf.Slice(int(len_)))
		if err == syscall.EINTR {
			continue // only the Go runtime's own signals get here
		}
		if err != nil {
			return -1
		}
		return int32(n)
	}
}

func host_raise(sig int32) {
	// DEVIATION: the C kill(getpid(), sig) runs the installed handler before
	// kill returns; here a caught signal's handler effect runs directly.
	if host_caught {
		switch syscall.Signal(sig) {
		case syscall.SIGHUP, syscall.SIGTERM:
			deathtrap(sig)
			return
		case syscall.SIGWINCH, syscall.SIGCONT:
			host_winch_pending.Store(true)
			return
		case syscall.SIGTSTP:
			host_tstp_pending.Store(true)
			return
		case syscall.SIGINT:
			host_int_pending.Store(true)
			return
		case syscall.SIGPIPE, syscall.SIGALRM:
			return
		}
	}
	syscall.Kill(syscall.Getpid(), syscall.Signal(sig))
}

// musl_suspend stops the process group with SIGTSTP at its default action.
// Go's signal.Reset would leave the runtime's handler installed, which
// swallows SIGTSTP, so the kernel action is set to SIG_DFL with
// rt_sigaction directly and the runtime's own restored after.
func musl_suspend() {
	var old, dfl [4]uint64 // struct kernel_sigaction: handler, flags, restorer, mask

	_, _, e := syscall.RawSyscall6(syscall.SYS_RT_SIGACTION, uintptr(syscall.SIGTSTP), 0, uintptr(unsafe.Pointer(&old)), 8, 0, 0)
	if e != 0 {
		// DEVIATION (fallback only): stop with SIGSTOP instead
		syscall.Kill(0, syscall.SIGSTOP)
		return
	}
	dfl = old
	dfl[0] = 0 // SIG_DFL
	syscall.RawSyscall6(syscall.SYS_RT_SIGACTION, uintptr(syscall.SIGTSTP), uintptr(unsafe.Pointer(&dfl)), 0, 8, 0, 0)
	syscall.Kill(0, syscall.SIGTSTP)
	syscall.RawSyscall6(syscall.SYS_RT_SIGACTION, uintptr(syscall.SIGTSTP), uintptr(unsafe.Pointer(&old)), 0, 8, 0, 0)
}

// host_exit is the C longjmp back to main, which returns the code: here the
// process simply exits with it.
func host_exit(r int32) {
	os.Exit(int(r))
}

func host_message(msg Ptr[byte], len_ int32, err int32) {
	n := len_
	var off int32

	if n < 0 {
		n = int32(hostStrlen(msg))
	}
	fd := 1
	if err != 0 {
		fd = 2
	}
	for off < n {
		w, e := syscall.Write(fd, msg.Add(int(off)).Slice(int(n-off)))
		if e == syscall.EINTR {
			continue
		}
		if w <= 0 || e != nil {
			return
		}
		off += int32(w)
	}
}

// The C host allocates from a static 1 GiB arena and never frees; the
// garbage collector is the allocator here, but the arena's accounting (and
// its exhaustion message) are kept.
const HOST_ARENA_BYTES = 1024 * 1024 * 1024

var host_arena_used usize

func host_arena_exhausted(n usize) {
	m := "whim-vim: host arena exhausted: " + hostUtoa(HOST_ARENA_BYTES) + " bytes, " +
		hostUtoa(host_arena_used) + " used, request " + hostUtoa(n) + "\n"
	b := Mk[byte](len(m) + 1)
	copy(b.Slice(len(m)), m)
	host_message(b, int32(len(m)), TRUE)
	host_exit(1)
}

func hostUtoa(v usize) string {
	var d [24]byte
	i := 24
	if v == 0 {
		i--
		d[i] = '0'
	}
	for v > 0 {
		i--
		d[i] = byte('0' + v%10)
		v /= 10
	}
	return string(d[i:])
}

// host_alloc returns n zeroed bytes, a Ptr[byte] as `any`.
func host_alloc(n usize) any {
	want := (n + 15) &^ usize(15) // alignof(max_align_t) is 16
	if want < n || want > HOST_ARENA_BYTES-host_arena_used {
		host_arena_exhausted(n)
	}
	host_arena_used += want
	return Alloc(int(n))
}

func host_write(s Ptr[byte], len_ int32) int32 {
	if len_ < 0 {
		return -1
	}
	if len_ == 0 {
		return 0
	}
	for {
		n, err := syscall.Write(1, s.Slice(int(len_)))
		if err == syscall.EINTR {
			continue // the Go runtime's own signals
		}
		if err != nil {
			return -1
		}
		return int32(n)
	}
}

// main is the launcher: argv as C builds it (NUL-terminated strings,
// argv[argc] NULL), and vim_main's result as the exit status.
func main() {
	argc := len(os.Args)
	argv := make([]Ptr[byte], argc+1)
	for i, a := range os.Args {
		argv[i] = View([]byte(a + "\x00"))
	}
	os.Exit(int(vim_main(int32(argc), argv)))
}
