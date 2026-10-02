//! vim's own printf, `vim_snprintf`, from the host half of whim-vim.c: the
//! core calls it for every message it formats, and it needs nothing of an
//! operating system, so it is the library's and not the Host's.  A hand port
//! of the C -- `musl_memchr`, `musl_fmtbase`, `musl_fmtnum`, `musl_fmtptr`,
//! `vim_snprintf`/`vim_vsnprintf`/`vim_vsnprintf_typval`, `format_typeof`,
//! `format_typename`, `adjust_types`, `get_unsigned_int`, `parse_fmt_types`
//! and `skip_to_arg` -- on the C's memory: the format and the result are
//! bytes at raw addresses, as in editor/format.go's and caprice's
//! Caprice.Printf ports, whose structure it follows.
//!
//! The C's va_list is the call's `[VArg]` and an index into it: `va_arg`
//! reads `args[ap]` and advances, `va_copy(ap, ap_start)` sets the index back
//! to 0.  An argument is read at the type the C's `va_arg` names (`VArg`'s
//! `bits()` truncated to it, or `ptr()`); every C type here takes one slot of
//! the list, so skipping an argument of any type is advancing the index.
//!
//! What the C leaves out of reach is left out here too: `get_unsigned_int`
//! is only ever called with `overflow_err` FALSE, so it clamps and never
//! reports, and `format_overflow_error`, which only that report calls, is
//! not ported.  The format errors (E1500-E1507) are written into `IObuff`
//! by this function and given to the core's `emsg`/`iemsg`, as the C does.
//!
//! DEVIATION: `va_arg` past the last argument -- undefined in C, a read of
//! whatever the stack holds -- reads 0 here (a null pointer for `%s`, which
//! prints `[NULL]`), so that no format can make this function panic.
//!
//! DEVIATION (invisible): the positional arguments' types, `ap_types`, are a
//! `Vec` of the C's pointers into the format, where the C allocates them on
//! its arena with `alloc_clear`/`host_alloc`; so the C's out-of-memory return
//! from `adjust_types` cannot happen.

use crate::editor::{self, Editor};
use crate::rt::VArg;
use core::ffi::c_void;

const TMP_LEN: usize = 350;

const TYPE_UNKNOWN: i32 = -1;
const TYPE_INT: i32 = 0;
const TYPE_LONGINT: i32 = 1;
const TYPE_LONGLONGINT: i32 = 2;
const TYPE_UNSIGNEDINT: i32 = 3;
const TYPE_UNSIGNEDLONGINT: i32 = 4;
const TYPE_UNSIGNEDLONGLONGINT: i32 = 5;
const TYPE_POINTER: i32 = 6;
const TYPE_PERCENT: i32 = 7;
const TYPE_CHAR: i32 = 8;
const TYPE_STRING: i32 = 9;

const MAX_ALLOWED_STRING_WIDTH: u32 = 1048576;

static TYPENAME_UNKNOWN: &[u8] = b"unknown\0";
static TYPENAME_INT: &[u8] = b"int\0";
static TYPENAME_LONGINT: &[u8] = b"long int\0";
static TYPENAME_LONGLONGINT: &[u8] = b"long long int\0";
static TYPENAME_UNSIGNEDINT: &[u8] = b"unsigned int\0";
static TYPENAME_UNSIGNEDLONGINT: &[u8] = b"unsigned long int\0";
static TYPENAME_UNSIGNEDLONGLONGINT: &[u8] = b"unsigned long long int\0";
static TYPENAME_POINTER: &[u8] = b"pointer\0";
static TYPENAME_PERCENT: &[u8] = b"percent\0";
static TYPENAME_CHAR: &[u8] = b"char\0";
static TYPENAME_STRING: &[u8] = b"string\0";

static E_CANNOT_MIX_POSITIONAL_AND_NON_POSITIONAL_STR: &[u8] =
    b"E1500: Cannot mix positional and non-positional arguments: %s\0";
static E_FMT_ARG_NR_UNUSED_STR: &[u8] = b"E1501: format argument %d unused in $-style format: %s\0";
static E_POSITIONAL_NUM_FIELD_SPEC_REUSED_STR_STR: &[u8] =
    b"E1502: Positional argument %d used as field width reused as different type: %s/%s\0";
static E_POSITIONAL_ARG_NUM_TYPE_INCONSISTENT_STR_STR: &[u8] =
    b"E1504: Positional argument %d type used inconsistently: %s/%s\0";
static E_INVALID_FORMAT_SPECIFIER_STR: &[u8] = b"E1505: Invalid format specifier: %s\0";
static E_APTYPES_IS_NULL_NR_STR: &[u8] = b"E1507: Internal error: ap_types or ap_types[idx] is NULL: %d: %s\0";

static NULL_STR: &[u8] = b"[NULL]\0";
static EMPTY_STR: &[u8] = b"\0";

/// A static C string of this file's, as the `char *` the C's arrays decay to
/// (never written through).
fn cs(s: &'static [u8]) -> *mut i8 {
    s.as_ptr() as *mut i8
}

/// `*p`, the byte at p.
#[inline(always)]
unsafe fn at(p: *const u8) -> u8 {
    *p
}

/// `(unsigned)c - '0' < 10`
#[inline(always)]
fn is_digit(c: u8) -> bool {
    c.wrapping_sub(b'0') < 10
}

unsafe fn c_strlen(s: *const u8) -> u64 {
    let mut n = 0u64;
    while *s.add(n as usize) != 0 {
        n += 1;
    }
    n
}

/// strchr(s, c): the first c at or after s, or null (c is never NUL here).
unsafe fn c_strchr(mut s: *const u8, c: u8) -> *const u8 {
    loop {
        if *s == c {
            return s;
        }
        if *s == 0 {
            return core::ptr::null();
        }
        s = s.add(1);
    }
}

/// `min(n, avail)` bytes of src at dest: the C's guarded memmove.
unsafe fn put_bytes(str: *mut u8, str_l: u64, str_m: u64, src: *const u8, n: u64) {
    if str_l < str_m {
        let avail = str_m - str_l;
        core::ptr::copy(src, str.add(str_l as usize), n.min(avail) as usize);
    }
}

/// `min(n, avail)` bytes c at dest: the C's guarded memset.
unsafe fn put_fill(str: *mut u8, str_l: u64, str_m: u64, c: u8, n: u64) {
    if str_l < str_m {
        let avail = str_m - str_l;
        core::ptr::write_bytes(str.add(str_l as usize), c, n.min(avail) as usize);
    }
}

unsafe fn musl_memchr(src: *const u8, c: i32, mut n: u64) -> *const u8 {
    let mut s = src;
    let ch = c as u8;
    while n != 0 && *s != ch {
        s = s.add(1);
        n -= 1;
    }
    if n != 0 {
        s
    } else {
        core::ptr::null()
    }
}

fn musl_fmtbase(spec: u8) -> u64 {
    if spec == b'o' {
        return 8;
    }
    if spec == b'x' || spec == b'X' {
        return 16;
    }
    10
}

unsafe fn musl_fmtnum(dest: *mut u8, mut v: u64, base: u64, upper: bool, isneg: bool) -> u64 {
    let mut digits = [0u8; 24];
    let mut n = 0usize;
    let mut out = 0usize;
    if isneg {
        v = (!v).wrapping_add(1);
    }
    loop {
        let d = (v % base) as u8;
        digits[n] = if d < 10 {
            b'0' + d
        } else if upper {
            b'A' + d - 10
        } else {
            b'a' + d - 10
        };
        n += 1;
        v /= base;
        if v == 0 {
            break;
        }
    }
    if isneg {
        *dest.add(out) = b'-';
        out += 1;
    }
    for i in 0..n {
        *dest.add(out) = digits[n - 1 - i];
        out += 1;
    }
    *dest.add(out) = 0;
    out as u64
}

unsafe fn musl_fmtptr(dest: *mut u8, v: u64) -> u64 {
    *dest = b'0';
    *dest.add(1) = b'x';
    for i in 0..16 {
        let d = ((v >> (60 - 4 * i)) & 0xf) as u8;
        *dest.add(2 + i) = if d < 10 { b'0' + d } else { b'a' + d - 10 };
    }
    *dest.add(18) = 0;
    18
}

/// The va_list: the arguments and the index of the next one.
struct Va<'a> {
    args: &'a [VArg],
    i: usize,
}

impl Va<'_> {
    /// va_arg: the next argument (0 past the last: see DEVIATION above).
    fn next(&mut self) -> VArg {
        let a = self.args.get(self.i).copied().unwrap_or(VArg::I(0));
        self.i += 1;
        a
    }
    fn int(&mut self) -> i32 {
        self.next().bits() as i32
    }
    fn uint(&mut self) -> u32 {
        self.next().bits() as u32
    }
    fn long(&mut self) -> i64 {
        self.next().bits() as i64
    }
    fn ulong(&mut self) -> u64 {
        self.next().bits()
    }
    fn string(&mut self) -> *const u8 {
        self.next().ptr()
    }
    /// va_arg(ap, void *), as its address.
    fn pointer(&mut self) -> u64 {
        self.next().bits()
    }
}

unsafe fn format_typeof(mut ty: *const u8) -> i32 {
    let mut length_modifier = 0u8;
    if at(ty) == b'h' || at(ty) == b'l' {
        length_modifier = at(ty);
        ty = ty.add(1);
        if length_modifier == b'l' && at(ty) == b'l' {
            length_modifier = b'L';
            ty = ty.add(1);
        }
    }
    let mut fmt_spec = at(ty);
    match fmt_spec {
        b'i' => fmt_spec = b'd',
        b'*' => {
            fmt_spec = b'd';
            length_modifier = b'h';
        }
        b'D' => {
            fmt_spec = b'd';
            length_modifier = b'l';
        }
        b'U' => {
            fmt_spec = b'u';
            length_modifier = b'l';
        }
        b'O' => {
            fmt_spec = b'o';
            length_modifier = b'l';
        }
        _ => {}
    }
    match fmt_spec {
        b'%' => return TYPE_PERCENT,
        b'c' => return TYPE_CHAR,
        b's' | b'S' => return TYPE_STRING,
        b'd' | b'u' | b'b' | b'B' | b'o' | b'x' | b'X' | b'p' => {
            if fmt_spec == b'p' {
                return TYPE_POINTER;
            } else if fmt_spec == b'b' || fmt_spec == b'B' {
                return TYPE_UNSIGNEDLONGLONGINT;
            } else if fmt_spec == b'd' {
                match length_modifier {
                    0 | b'h' => return TYPE_INT,
                    b'l' => return TYPE_LONGINT,
                    b'L' => return TYPE_LONGLONGINT,
                    _ => {}
                }
            } else {
                match length_modifier {
                    0 | b'h' => return TYPE_UNSIGNEDINT,
                    b'l' => return TYPE_UNSIGNEDLONGINT,
                    b'L' => return TYPE_UNSIGNEDLONGLONGINT,
                    _ => {}
                }
            }
        }
        _ => {}
    }
    TYPE_UNKNOWN
}

unsafe fn format_typename(ty: *const u8) -> *mut i8 {
    cs(match format_typeof(ty) {
        TYPE_INT => TYPENAME_INT,
        TYPE_LONGINT => TYPENAME_LONGINT,
        TYPE_LONGLONGINT => TYPENAME_LONGLONGINT,
        TYPE_UNSIGNEDINT => TYPENAME_UNSIGNEDINT,
        TYPE_UNSIGNEDLONGINT => TYPENAME_UNSIGNEDLONGINT,
        TYPE_UNSIGNEDLONGLONGINT => TYPENAME_UNSIGNEDLONGLONGINT,
        TYPE_POINTER => TYPENAME_POINTER,
        TYPE_PERCENT => TYPENAME_PERCENT,
        TYPE_CHAR => TYPENAME_CHAR,
        TYPE_STRING => TYPENAME_STRING,
        _ => TYPENAME_UNKNOWN,
    })
}

/// `vim_snprintf((char *)IObuff, emsg_iobuff_room(), e, args...);
/// emsg(iobuff_or(e));` -- or iemsg, for skip_to_arg's internal error.
unsafe fn format_error(ed: *mut Editor, e: &'static [u8], args: &[VArg], internal: bool) {
    let room = editor::emsg_iobuff_room(ed);
    vim_snprintf(ed, (*ed).IObuff as *mut i8, room, cs(e), args);
    let s = editor::iobuff_or(ed, cs(e));
    if internal {
        editor::iemsg(ed, s);
    } else {
        editor::emsg(ed, s);
    }
}

/// The positional arguments' types: pointers into the format, null where
/// none was seen yet (the C's `ap_types`, `num_posarg` its length).
type ApTypes = Vec<*const u8>;

unsafe fn adjust_types(ed: *mut Editor, ap_types: &mut ApTypes, arg: i32, ty: *const u8) -> bool {
    if arg <= 0 {
        format_error(ed, E_INVALID_FORMAT_SPECIFIER_STR, &[VArg::P(ty as *const c_void)], false);
        return false;
    }
    let arg = arg as usize;
    if ap_types.len() < arg {
        ap_types.resize(arg, core::ptr::null());
    }
    let prev = ap_types[arg - 1];
    if !prev.is_null() {
        if at(prev) == b'*' || at(ty) == b'*' {
            let mut pt = ty;
            if at(pt) == b'*' {
                pt = prev;
            }
            if at(pt) != b'*' {
                match at(pt) {
                    b'd' | b'i' => {}
                    _ => {
                        format_error(
                            ed,
                            E_POSITIONAL_NUM_FIELD_SPEC_REUSED_STR_STR,
                            &[
                                VArg::I(arg as i64),
                                VArg::P(format_typename(prev) as *const c_void),
                                VArg::P(format_typename(ty) as *const c_void),
                            ],
                            false,
                        );
                        return false;
                    }
                }
            }
        } else if format_typeof(ty) != format_typeof(prev) {
            format_error(
                ed,
                E_POSITIONAL_ARG_NUM_TYPE_INCONSISTENT_STR_STR,
                &[
                    VArg::I(arg as i64),
                    VArg::P(format_typename(ty) as *const c_void),
                    VArg::P(format_typename(prev) as *const c_void),
                ],
                false,
            );
            return false;
        }
    }
    ap_types[arg - 1] = ty;
    true
}

/// get_unsigned_int with overflow_err FALSE, its only use: the digits at
/// *p while the value is under the limit, clamped to it.
unsafe fn get_unsigned_int(p: &mut *const u8) -> u32 {
    let mut uj = (at(*p) - b'0') as u32;
    *p = p.add(1);
    while is_digit(at(*p)) && uj < MAX_ALLOWED_STRING_WIDTH {
        uj = 10 * uj + (at(*p) - b'0') as u32;
        *p = p.add(1);
    }
    if uj > MAX_ALLOWED_STRING_WIDTH {
        uj = MAX_ALLOWED_STRING_WIDTH;
    }
    uj
}

/// The C's `CHECK_POS_ARG`: an error when positional and non-positional
/// arguments were both seen.
unsafe fn mixed(ed: *mut Editor, any_pos: bool, any_arg: bool, fmt: *const u8) -> bool {
    if any_pos && any_arg {
        format_error(ed, E_CANNOT_MIX_POSITIONAL_AND_NON_POSITIONAL_STR, &[VArg::P(fmt as *const c_void)], false);
        return true;
    }
    false
}

/// parse_fmt_types: the positional arguments' types, or None after an
/// error has been given (the C's FAIL, its `ap_types` freed).
unsafe fn parse_fmt_types(ed: *mut Editor, fmt: *const u8) -> Option<ApTypes> {
    let mut ap_types: ApTypes = Vec::new();
    let mut p = fmt;
    let mut any_pos = false;
    let mut any_arg = false;
    if p.is_null() {
        return Some(ap_types);
    }
    let invalid = |ed: *mut Editor| {
        format_error(ed, E_INVALID_FORMAT_SPECIFIER_STR, &[VArg::P(fmt as *const c_void)], false);
    };
    while at(p) != 0 {
        if at(p) != b'%' {
            let q = c_strchr(p.add(1), b'%');
            let n = if q.is_null() { c_strlen(p) } else { q.offset_from(p) as u64 };
            p = p.add(n as usize);
        } else {
            let mut pos_arg: i32 = -1;
            p = p.add(1);
            let mut ptype = p;
            while is_digit(at(ptype)) {
                ptype = ptype.add(1);
            }
            if at(ptype) == b'$' {
                if at(p) == b'0' {
                    invalid(ed);
                    return None;
                }
                let uj = get_unsigned_int(&mut p);
                pos_arg = uj as i32;
                any_pos = true;
                if mixed(ed, any_pos, any_arg, fmt) {
                    return None;
                }
                p = p.add(1);
            }
            while matches!(at(p), b'0' | b'-' | b'+' | b' ' | b'#' | b'\'') {
                p = p.add(1);
            }
            let mut arg = p;
            if at(arg) == b'*' {
                p = p.add(1);
                if is_digit(at(p)) {
                    let uj = get_unsigned_int(&mut p);
                    if at(p) != b'$' {
                        invalid(ed);
                        return None;
                    }
                    p = p.add(1);
                    any_pos = true;
                    if mixed(ed, any_pos, any_arg, fmt) {
                        return None;
                    }
                    if !adjust_types(ed, &mut ap_types, uj as i32, arg) {
                        return None;
                    }
                } else {
                    any_arg = true;
                    if mixed(ed, any_pos, any_arg, fmt) {
                        return None;
                    }
                }
            } else if is_digit(at(p)) {
                get_unsigned_int(&mut p);
                if at(p) == b'$' {
                    invalid(ed);
                    return None;
                }
            }
            if at(p) == b'.' {
                p = p.add(1);
                arg = p;
                if at(arg) == b'*' {
                    p = p.add(1);
                    if is_digit(at(p)) {
                        let uj = get_unsigned_int(&mut p);
                        if at(p) == b'$' {
                            any_pos = true;
                            if mixed(ed, any_pos, any_arg, fmt) {
                                return None;
                            }
                            p = p.add(1);
                            if !adjust_types(ed, &mut ap_types, uj as i32, arg) {
                                return None;
                            }
                        } else {
                            invalid(ed);
                            return None;
                        }
                    } else {
                        any_arg = true;
                        if mixed(ed, any_pos, any_arg, fmt) {
                            return None;
                        }
                    }
                } else if is_digit(at(p)) {
                    get_unsigned_int(&mut p);
                    if at(p) == b'$' {
                        invalid(ed);
                        return None;
                    }
                }
            }
            if pos_arg != -1 {
                any_pos = true;
                if mixed(ed, any_pos, any_arg, fmt) {
                    return None;
                }
                ptype = p;
            }
            if at(p) == b'h' || at(p) == b'l' {
                let length_modifier = at(p);
                p = p.add(1);
                if length_modifier == b'l' && at(p) == b'l' {
                    p = p.add(1);
                }
            }
            match at(p) {
                b'i' | b'*' | b'd' | b'u' | b'o' | b'D' | b'U' | b'O' | b'x' | b'X' | b'b' | b'B' | b'c' | b's'
                | b'S' | b'p' => {
                    if pos_arg != -1 {
                        if !adjust_types(ed, &mut ap_types, pos_arg, ptype) {
                            return None;
                        }
                    } else {
                        any_arg = true;
                        if mixed(ed, any_pos, any_arg, fmt) {
                            return None;
                        }
                    }
                }
                _ => {
                    if pos_arg != -1 {
                        format_error(
                            ed,
                            E_CANNOT_MIX_POSITIONAL_AND_NON_POSITIONAL_STR,
                            &[VArg::P(fmt as *const c_void)],
                            false,
                        );
                        return None;
                    }
                }
            }
            if at(p) != 0 {
                p = p.add(1);
            }
        }
    }
    for (arg_idx, t) in ap_types.iter().enumerate() {
        if t.is_null() {
            format_error(
                ed,
                E_FMT_ARG_NR_UNUSED_STR,
                &[VArg::I(arg_idx as i64 + 1), VArg::P(fmt as *const c_void)],
                false,
            );
            return None;
        }
    }
    Some(ap_types)
}

/// skip_to_arg: ap at argument arg_idx (1-based), arg_cur the one before it,
/// arg_idx then the one after.
unsafe fn skip_to_arg(
    ed: *mut Editor,
    ap_types: &ApTypes,
    ap: &mut Va,
    arg_idx: &mut i32,
    arg_cur: &mut i32,
    fmt: *const u8,
) {
    let mut arg_min = 0;
    if *arg_cur + 1 == *arg_idx {
        *arg_cur += 1;
        *arg_idx += 1;
        return;
    }
    if *arg_cur >= *arg_idx {
        ap.i = 0;
    } else {
        arg_min = *arg_cur;
    }
    *arg_cur = arg_min;
    while *arg_cur < *arg_idx - 1 {
        let t = ap_types.get(*arg_cur as usize).copied().unwrap_or(core::ptr::null());
        if t.is_null() {
            format_error(
                ed,
                E_APTYPES_IS_NULL_NR_STR,
                &[VArg::I(*arg_cur as i64), VArg::P(fmt as *const c_void)],
                true,
            );
            return;
        }
        match format_typeof(t) {
            TYPE_PERCENT | TYPE_UNKNOWN => {}
            _ => {
                ap.next();
            }
        }
        *arg_cur += 1;
    }
    *arg_cur += 1;
    *arg_idx += 1;
}

/// vim_snprintf: fmt formatted with args into str, at most str_m bytes with
/// its NUL; the length the whole would have had.
///
/// # Safety
/// `ed` is an editor; `str` holds `str_m` bytes (null when 0); `fmt` is null
/// or a C string; every `%s` argument is null or a C string.
pub unsafe fn vim_snprintf(ed: *mut Editor, str: *mut i8, str_m: u64, fmt: *mut i8, args: &[VArg]) -> i32 {
    let str = str as *mut u8;
    let fmt = fmt as *const u8;
    let mut str_l: u64 = 0;
    let mut p = fmt;
    let mut arg_cur: i32 = 0;
    let mut arg_idx: i32 = 1;
    let mut ap = Va { args, i: 0 };
    let ap_types = match parse_fmt_types(ed, fmt) {
        Some(t) => t,
        None => return 0,
    };
    if p.is_null() {
        p = EMPTY_STR.as_ptr();
    }
    let mut tmp = [0u8; TMP_LEN];
    let mut uchar_arg = [0u8; 1];
    while at(p) != 0 {
        if at(p) != b'%' {
            let q = c_strchr(p.add(1), b'%');
            let n = if q.is_null() { c_strlen(p) } else { q.offset_from(p) as u64 };
            put_bytes(str, str_l, str_m, p, n);
            p = p.add(n as usize);
            str_l = str_l.wrapping_add(n);
            continue;
        }
        let mut min_field_width: u64 = 0;
        let mut precision: u64 = 0;
        let mut zero_padding = false;
        let mut precision_specified = false;
        let mut justify_left = false;
        let mut alternate_form = false;
        let mut force_sign = false;
        let mut space_for_positive = true;
        let mut length_modifier = 0u8;
        let str_arg: *const u8;
        let str_arg_l: u64;
        let mut number_of_zeros_to_pad: u64 = 0;
        let mut zero_padding_insertion_ind: u64 = 0;
        let mut pos_arg: i32 = -1;
        p = p.add(1);
        let mut ptype = p;
        while is_digit(at(ptype)) {
            ptype = ptype.add(1);
        }
        if at(ptype) == b'$' {
            pos_arg = get_unsigned_int(&mut p) as i32;
            p = p.add(1);
        }
        while matches!(at(p), b'0' | b'-' | b'+' | b' ' | b'#' | b'\'') {
            match at(p) {
                b'0' => zero_padding = true,
                b'-' => justify_left = true,
                b'+' => {
                    force_sign = true;
                    space_for_positive = false;
                }
                b' ' => force_sign = true,
                b'#' => alternate_form = true,
                _ => {}
            }
            p = p.add(1);
        }
        if at(p) == b'*' {
            p = p.add(1);
            if is_digit(at(p)) {
                arg_idx = get_unsigned_int(&mut p) as i32;
                p = p.add(1);
            }
            skip_to_arg(ed, &ap_types, &mut ap, &mut arg_idx, &mut arg_cur, fmt);
            let mut j = ap.int();
            if j > MAX_ALLOWED_STRING_WIDTH as i32 {
                j = MAX_ALLOWED_STRING_WIDTH as i32;
            }
            if j >= 0 {
                min_field_width = j as u64;
            } else {
                min_field_width = j.wrapping_neg() as i64 as u64;
                justify_left = true;
            }
        } else if is_digit(at(p)) {
            min_field_width = get_unsigned_int(&mut p) as u64;
        }
        if at(p) == b'.' {
            p = p.add(1);
            precision_specified = true;
            if is_digit(at(p)) {
                precision = get_unsigned_int(&mut p) as u64;
            } else if at(p) == b'*' {
                p = p.add(1);
                if is_digit(at(p)) {
                    arg_idx = get_unsigned_int(&mut p) as i32;
                    p = p.add(1);
                }
                skip_to_arg(ed, &ap_types, &mut ap, &mut arg_idx, &mut arg_cur, fmt);
                let mut j = ap.int();
                if j > MAX_ALLOWED_STRING_WIDTH as i32 {
                    j = MAX_ALLOWED_STRING_WIDTH as i32;
                }
                if j >= 0 {
                    precision = j as u64;
                } else {
                    precision_specified = false;
                    precision = 0;
                }
            }
        }
        if at(p) == b'h' || at(p) == b'l' {
            length_modifier = at(p);
            p = p.add(1);
            if length_modifier == b'l' && at(p) == b'l' {
                length_modifier = b'L';
                p = p.add(1);
            }
        }
        let mut fmt_spec = at(p);
        match fmt_spec {
            b'i' => fmt_spec = b'd',
            b'D' => {
                fmt_spec = b'd';
                length_modifier = b'l';
            }
            b'U' => {
                fmt_spec = b'u';
                length_modifier = b'l';
            }
            b'O' => {
                fmt_spec = b'o';
                length_modifier = b'l';
            }
            _ => {}
        }
        if pos_arg != -1 {
            arg_idx = pos_arg;
        }
        match fmt_spec {
            b'%' | b'c' | b's' | b'S' => {
                let mut l: u64 = 1;
                match fmt_spec {
                    b'%' => str_arg = p,
                    b'c' => {
                        skip_to_arg(ed, &ap_types, &mut ap, &mut arg_idx, &mut arg_cur, fmt);
                        uchar_arg[0] = ap.int() as u8;
                        str_arg = uchar_arg.as_ptr();
                    }
                    _ => {
                        skip_to_arg(ed, &ap_types, &mut ap, &mut arg_idx, &mut arg_cur, fmt);
                        let s = ap.string();
                        if s.is_null() {
                            str_arg = NULL_STR.as_ptr();
                            l = 6;
                        } else {
                            str_arg = s;
                            if !precision_specified {
                                l = c_strlen(s);
                            } else if precision == 0 {
                                l = 0;
                            } else {
                                let q = musl_memchr(s, 0, precision.min(0x7fffffff));
                                l = if q.is_null() { precision } else { q.offset_from(s) as u64 };
                            }
                        }
                        if fmt_spec == b'S' {
                            let mut i: u64 = 0;
                            let mut p1 = str_arg as *mut u8;
                            while *p1 != 0 {
                                let cell = editor::utf_ptr2cells(ed, p1);
                                if precision_specified && i.wrapping_add(cell as i64 as u64) > precision {
                                    break;
                                }
                                i = i.wrapping_add(cell as i64 as u64);
                                p1 = p1.offset(editor::utfc_ptr2len(ed, p1) as isize);
                            }
                            l = p1.offset_from(str_arg) as u64;
                            if min_field_width != 0 {
                                min_field_width = min_field_width.wrapping_add(l.wrapping_sub(i));
                            }
                        }
                    }
                }
                str_arg_l = l;
            }
            b'd' | b'u' | b'b' | b'B' | b'o' | b'x' | b'X' | b'p' => {
                let mut arg_sign = 0;
                let mut int_arg: i32 = 0;
                let mut uint_arg: u32 = 0;
                let mut long_arg: i64 = 0;
                let mut ulong_arg: u64 = 0;
                let mut llong_arg: i64 = 0;
                let mut ullong_arg: u64 = 0;
                let mut bin_arg: u64 = 0;
                let mut ptr_arg: u64 = 0;
                if fmt_spec == b'p' {
                    length_modifier = 0;
                    skip_to_arg(ed, &ap_types, &mut ap, &mut arg_idx, &mut arg_cur, fmt);
                    ptr_arg = ap.pointer();
                    if ptr_arg != 0 {
                        arg_sign = 1;
                    }
                } else if fmt_spec == b'b' || fmt_spec == b'B' {
                    skip_to_arg(ed, &ap_types, &mut ap, &mut arg_idx, &mut arg_cur, fmt);
                    bin_arg = ap.ulong();
                    if bin_arg != 0 {
                        arg_sign = 1;
                    }
                } else if fmt_spec == b'd' {
                    match length_modifier {
                        0 | b'h' => {
                            skip_to_arg(ed, &ap_types, &mut ap, &mut arg_idx, &mut arg_cur, fmt);
                            int_arg = ap.int();
                            arg_sign = int_arg.signum();
                        }
                        b'l' => {
                            skip_to_arg(ed, &ap_types, &mut ap, &mut arg_idx, &mut arg_cur, fmt);
                            long_arg = ap.long();
                            arg_sign = long_arg.signum() as i32;
                        }
                        _ => {
                            skip_to_arg(ed, &ap_types, &mut ap, &mut arg_idx, &mut arg_cur, fmt);
                            llong_arg = ap.long();
                            arg_sign = llong_arg.signum() as i32;
                        }
                    }
                } else {
                    match length_modifier {
                        0 | b'h' => {
                            skip_to_arg(ed, &ap_types, &mut ap, &mut arg_idx, &mut arg_cur, fmt);
                            uint_arg = ap.uint();
                            if uint_arg != 0 {
                                arg_sign = 1;
                            }
                        }
                        b'l' => {
                            skip_to_arg(ed, &ap_types, &mut ap, &mut arg_idx, &mut arg_cur, fmt);
                            ulong_arg = ap.ulong();
                            if ulong_arg != 0 {
                                arg_sign = 1;
                            }
                        }
                        _ => {
                            skip_to_arg(ed, &ap_types, &mut ap, &mut arg_idx, &mut arg_cur, fmt);
                            ullong_arg = ap.ulong();
                            if ullong_arg != 0 {
                                arg_sign = 1;
                            }
                        }
                    }
                }
                let t = tmp.as_mut_ptr();
                let mut l: u64 = 0;
                if precision_specified {
                    zero_padding = false;
                }
                if fmt_spec == b'd' {
                    if force_sign && arg_sign >= 0 {
                        tmp[l as usize] = if space_for_positive { b' ' } else { b'+' };
                        l += 1;
                    }
                } else if alternate_form
                    && arg_sign != 0
                    && (fmt_spec == b'b' || fmt_spec == b'B' || fmt_spec == b'x' || fmt_spec == b'X')
                {
                    tmp[l as usize] = b'0';
                    tmp[l as usize + 1] = fmt_spec;
                    l += 2;
                }
                zero_padding_insertion_ind = l;
                if !precision_specified {
                    precision = 1;
                }
                if !(precision == 0 && arg_sign == 0) {
                    let d = t.add(l as usize);
                    if fmt_spec == b'p' {
                        l += musl_fmtptr(d, ptr_arg);
                    } else if fmt_spec == b'b' || fmt_spec == b'B' {
                        let mut b = [0u8; 64];
                        let mut b_l = 0usize;
                        let mut bn = bin_arg;
                        loop {
                            b_l += 1;
                            b[64 - b_l] = b'0' + (bn & 0x1) as u8;
                            bn >>= 1;
                            if bn == 0 {
                                break;
                            }
                        }
                        core::ptr::copy_nonoverlapping(b.as_ptr().add(64 - b_l), d, b_l);
                        l += b_l as u64;
                    } else if fmt_spec == b'd' {
                        l += match length_modifier {
                            0 => musl_fmtnum(d, int_arg as i64 as u64, 10, false, int_arg < 0),
                            b'h' => musl_fmtnum(d, int_arg as i16 as i64 as u64, 10, false, (int_arg as i16) < 0),
                            b'l' => musl_fmtnum(d, long_arg as u64, 10, false, long_arg < 0),
                            _ => musl_fmtnum(d, llong_arg as u64, 10, false, llong_arg < 0),
                        };
                    } else {
                        let base = musl_fmtbase(fmt_spec);
                        let upper = fmt_spec == b'X';
                        l += match length_modifier {
                            0 => musl_fmtnum(d, uint_arg as u64, base, upper, false),
                            b'h' => musl_fmtnum(d, uint_arg as u16 as u64, base, upper, false),
                            b'l' => musl_fmtnum(d, ulong_arg, base, upper, false),
                            _ => musl_fmtnum(d, ullong_arg, base, upper, false),
                        };
                    }
                    let z = zero_padding_insertion_ind as usize;
                    if zero_padding_insertion_ind < l && tmp[z] == b'-' {
                        zero_padding_insertion_ind += 1;
                    }
                    let z = zero_padding_insertion_ind as usize;
                    if zero_padding_insertion_ind + 1 < l && tmp[z] == b'0' && (tmp[z + 1] == b'x' || tmp[z + 1] == b'X') {
                        zero_padding_insertion_ind += 2;
                    }
                }
                {
                    let num_of_digits = l - zero_padding_insertion_ind;
                    if alternate_form
                        && fmt_spec == b'o'
                        && !(zero_padding_insertion_ind < l && tmp[zero_padding_insertion_ind as usize] == b'0')
                        && (!precision_specified || precision < num_of_digits + 1)
                    {
                        precision = num_of_digits + 1;
                    }
                    if num_of_digits < precision {
                        number_of_zeros_to_pad = precision - num_of_digits;
                    }
                }
                if !justify_left && zero_padding {
                    let n = min_field_width.wrapping_sub(l.wrapping_add(number_of_zeros_to_pad)) as i32;
                    if n > 0 {
                        number_of_zeros_to_pad = number_of_zeros_to_pad.wrapping_add(n as u64);
                    }
                }
                str_arg = t;
                str_arg_l = l;
            }
            _ => {
                zero_padding = false;
                justify_left = true;
                min_field_width = 0;
                str_arg = p;
                str_arg_l = if at(p) != 0 { 1 } else { 0 };
            }
        }
        if at(p) != 0 {
            p = p.add(1);
        }
        if !justify_left {
            let pn = min_field_width.wrapping_sub(str_arg_l.wrapping_add(number_of_zeros_to_pad)) as i32;
            if pn > 0 {
                put_fill(str, str_l, str_m, if zero_padding { b'0' } else { b' ' }, pn as u64);
                str_l = str_l.wrapping_add(pn as u64);
            }
        }
        if number_of_zeros_to_pad == 0 {
            zero_padding_insertion_ind = 0;
        } else {
            let zn = zero_padding_insertion_ind as i32;
            if zn > 0 {
                put_bytes(str, str_l, str_m, str_arg, zn as u64);
                str_l = str_l.wrapping_add(zn as u64);
            }
            let zn = number_of_zeros_to_pad as i32;
            if zn > 0 {
                put_fill(str, str_l, str_m, b'0', zn as u64);
                str_l = str_l.wrapping_add(zn as u64);
            }
        }
        {
            let sn = str_arg_l.wrapping_sub(zero_padding_insertion_ind) as i32;
            if sn > 0 {
                put_bytes(str, str_l, str_m, str_arg.add(zero_padding_insertion_ind as usize), sn as u64);
                str_l = str_l.wrapping_add(sn as u64);
            }
        }
        if justify_left {
            let pn = min_field_width.wrapping_sub(str_arg_l.wrapping_add(number_of_zeros_to_pad)) as i32;
            if pn > 0 {
                put_fill(str, str_l, str_m, b' ', pn as u64);
                str_l = str_l.wrapping_add(pn as u64);
            }
        }
    }
    if str_m > 0 {
        *str.add(if str_l < str_m { str_l } else { str_m - 1 } as usize) = 0;
    }
    str_l as i32
}
