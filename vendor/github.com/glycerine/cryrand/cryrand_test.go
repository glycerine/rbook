package cryrand

import (
	"bytes"
	"fmt"
	"testing"
)

func BenchmarkNew(b *testing.B) {
	b.Run("CryptoRandInt64", func(b *testing.B) {
		b.SetBytes(8)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			CryptoRandInt64()
		}
	})

	b.Run("MathRandInt64", func(b *testing.B) {
		b.SetBytes(8)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			MathRandInt64()
		}
	})

}

func TestMathRandSomeNeg(t *testing.T) {
	pos := 0
	neg := 0
	n := 1000
	for i := 0; i < n; i++ {
		x := MathRandInt64()
		if x > 0 {
			pos++
		} else if x < 0 {
			neg++
		}
	}
	fmt.Printf("pos:%v, neg:%v\n",
		float64(pos)/float64(n),
		float64(neg)/float64(n))
}

func TestCryptoRandSomeNeg(t *testing.T) {
	pos := 0
	neg := 0
	n := 1000
	for i := 0; i < n; i++ {
		x := CryptoRandInt64()
		if x > 0 {
			pos++
		} else if x < 0 {
			neg++
		}
	}
	fmt.Printf("pos:%v, neg:%v\n",
		float64(pos)/float64(n),
		float64(neg)/float64(n))
}

func TestBytes(t *testing.T) {
	ma := MathRandBytes(80)
	fmt.Printf("MathRandBytes: %x\n", ma)
	cr := CryptoRandBytes(80)
	fmt.Printf("CryptoRandBytes: %x\n", cr)
	if 0 == bytes.Compare(ma, cr) {
		panic("man, something is seriously wrong here.")
	}

	for i := 0; i <= 32; i++ {
		s := MathRandHexString(i)
		if len(s) != i {
			panic("wrong len")
		}
		fmt.Printf("MathRandHexString(%v): '%s' (len %v)\n", i, s, len(s))
	}
}
