package core

import (
	"math/big"
	"testing"
)

func TestBoxedPrimitiveInfoIsolation(t *testing.T) {
	for _, original := range []Object{boxBoolean(false), boxBoolean(true), boxInt(0), boxInt(255), boxInt(256), boxInt(2016), boxInt(2047), boxInt(2048), boxInt(-1), boxChar('a'), boxChar('λ')} {
		info := &ObjectInfo{}
		located := original.WithInfo(info)
		_, isInt := original.(Int) // go-whim: an Int keeps no position (object.go)
		if (located.GetInfo() != info) != isInt || original.GetInfo() != nil || !located.Equals(original) {
			t.Fatalf("boxing changed source-info isolation for %s", original.ToString(true))
		}
	}
	if boxInt(42).(Int).I != 42 || boxChar('λ').(Char).Ch != 'λ' {
		t.Fatal("boxing changed primitive representation")
	}
}

func TestIntEqualityFastPathCompatibility(t *testing.T) {
	for _, other := range []Object{Int{I: 1}, Int{I: 2}, MakeBigInt(big.NewInt(1)), Double{D: 1}, MakeString("1"), NIL} {
		x := Int{I: 1} // go-whim: an Int keeps no spelling
		if x.Equals(other) != equalsNumbers(x, other) {
			t.Fatalf("integer equality changed for %T", other)
		}
	}
}
