package togo

// rs_range.go is what the Rust backend knows of an integer value's range
// (doc/RUST-IDIOMS.md, item 3): enough to write `c as i32 - 48` where
// `(c as i32).wrapping_sub(48)` was -- an operation whose result provably
// fits its type is Rust's plain operator, which in a debug build would
// panic where C's would overflow; the rest stay `wrapping_*`.
//
// A value's range is its type's -- a u8 is 0..=255, a bool converted 0..=1
// -- narrowed where the backend sees more: a constant is itself, a
// conversion that keeps every value keeps the range, `x & 0x7f` is
// 0..=0x7f, `x % 10` of a value not negative 0..=9, and an operation on two
// known ranges has the range of its results.  Nothing is followed through
// a variable: a local's range is its type's.  Division and remainder of
// unsigned values are plain (they cannot overflow), and of signed ones when
// the divisor cannot be -1 or the dividend cannot be the type's least.

import (
	"math"
	"math/big"
)

// typeRange is the range of a Rust integer type, when an int64 holds it.
func typeRange(ty string) (lo, hi int64, ok bool) {
	switch ty {
	case "bool":
		return 0, 1, true
	case "i8":
		return math.MinInt8, math.MaxInt8, true
	case "u8":
		return 0, math.MaxUint8, true
	case "i16":
		return math.MinInt16, math.MaxInt16, true
	case "u16":
		return 0, math.MaxUint16, true
	case "i32":
		return math.MinInt32, math.MaxInt32, true
	case "u32":
		return 0, math.MaxUint32, true
	case "i64", "isize":
		return math.MinInt64, math.MaxInt64, true
	}
	return 0, 0, false // u64 and usize: past an int64
}

// rangeOf is v's range, when it is known.
func rangeOf(v rv) (lo, hi int64, ok bool) {
	if v.konst && !v.null {
		if (v.ty == "u64" || v.ty == "usize") && v.kv < 0 {
			return 0, 0, false
		}
		return v.kv, v.kv, true
	}
	if v.ranged {
		return v.lo, v.hi, true
	}
	return typeRange(v.ty)
}

// withRange is v with its range narrowed to lo..hi, when that says more
// than its type.
func withRange(v rv, lo, hi int64) rv {
	if tlo, thi, ok := typeRange(v.ty); ok && lo <= tlo && hi >= thi {
		return v
	}
	v.ranged, v.lo, v.hi = true, lo, hi
	return v
}

// rangeFits says lo..hi is within ty's values.
func rangeFits(lo, hi int64, ty string) bool {
	if ty == "u64" || ty == "usize" {
		return lo >= 0
	}
	tlo, thi, ok := typeRange(ty)
	return ok && lo >= tlo && hi <= thi
}

// arithRange is the range of a op b for every a in alo..ahi and b in
// blo..bhi, computed exactly, and whether it fits an int64.
func arithRange(op string, alo, ahi, blo, bhi int64) (lo, hi int64, ok bool) {
	var cands []*big.Int
	A := []*big.Int{big.NewInt(alo), big.NewInt(ahi)}
	B := []*big.Int{big.NewInt(blo), big.NewInt(bhi)}
	for _, a := range A {
		for _, b := range B {
			z := new(big.Int)
			switch op {
			case "+":
				z.Add(a, b)
			case "-":
				z.Sub(a, b)
			case "*":
				z.Mul(a, b)
			default:
				return 0, 0, false
			}
			cands = append(cands, z)
		}
	}
	mn, mx := cands[0], cands[0]
	for _, c := range cands[1:] {
		if c.Cmp(mn) < 0 {
			mn = c
		}
		if c.Cmp(mx) > 0 {
			mx = c
		}
	}
	if !mn.IsInt64() || !mx.IsInt64() {
		return 0, 0, false
	}
	return mn.Int64(), mx.Int64(), true
}

// plainArith says whether a op b in ty is Rust's plain operator: no
// operand's value can make it overflow (a and b already of ty).  ranged
// says lo..hi is the result's range; without it the result's is its type's.
func plainArith(op string, a, b rv, ty string) (plain bool, lo, hi int64, ranged bool) {
	alo, ahi, ok1 := rangeOf(a)
	blo, bhi, ok2 := rangeOf(b)
	switch op {
	case "/", "%":
		if !isSigned(ty) {
			// unsigned: it cannot overflow
			switch {
			case ok2 && blo > 0 && op == "%":
				return true, 0, bhi - 1, true
			case ok1 && ok2 && blo > 0:
				return true, alo / bhi, ahi / blo, true
			}
			return true, 0, 0, false
		}
		tlo, _, _ := typeRange(ty)
		if !(ok2 && (blo > -1 || bhi < -1)) && !(ok1 && alo > tlo) {
			return false, 0, 0, false // MIN / -1 overflows
		}
		switch {
		case ok2 && blo > 0 && op == "%" && ok1 && alo >= 0:
			return true, 0, bhi - 1, true
		case ok2 && blo > 0 && op == "%":
			return true, -(bhi - 1), bhi - 1, true
		case ok1 && ok2 && blo > 0:
			return true, min(alo/blo, alo/bhi), max(ahi/blo, ahi/bhi), true
		}
		return true, 0, 0, false
	}
	if !ok1 || !ok2 {
		return false, 0, 0, false
	}
	lo, hi, ok := arithRange(op, alo, ahi, blo, bhi)
	if !ok || !rangeFits(lo, hi, ty) {
		return false, 0, 0, false
	}
	return true, lo, hi, true
}

// bitRange is the range of a op b for & | ^ of ranges known, where it is
// narrower than the type's: of values not negative.
func bitRange(op string, a, b rv) (lo, hi int64, ok bool) {
	alo, ahi, ok1 := rangeOf(a)
	blo, bhi, ok2 := rangeOf(b)
	switch op {
	case "&":
		switch {
		case ok1 && ok2 && alo >= 0 && blo >= 0:
			return 0, min(ahi, bhi), true
		case ok1 && alo >= 0:
			return 0, ahi, true
		case ok2 && blo >= 0:
			return 0, bhi, true
		}
	case "|", "^":
		if ok1 && ok2 && alo >= 0 && blo >= 0 {
			m := max(ahi, bhi)
			p := int64(1)
			for p <= m && p < math.MaxInt64/2 {
				p <<= 1
			}
			return 0, p - 1, true
		}
	}
	return 0, 0, false
}
