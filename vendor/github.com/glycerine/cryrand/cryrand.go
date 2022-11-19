package cryrand

import (
	"encoding/binary"

	cr "crypto/rand"
	mr "math/rand"
)

var mathRandSrc *mr.Rand

func init() {
	seed := CryptoRandInt64()
	mathRandSrc = mr.New(mr.NewSource(seed))
}

func SeedOurMathRandSrc(seed int64) {
	mathRandSrc.Seed(seed)
}

func MathRandInt64() int64 {
	// generate one rand for the sign, xor with a 2nd.
	return (mathRandSrc.Int63() << 1) ^ mathRandSrc.Int63()
}

// Use crypto/rand to get an random int64.
func CryptoRandInt64() int64 {
	b := make([]byte, 8)
	_, err := cr.Read(b)
	if err != nil {
		panic(err)
	}
	r := int64(binary.LittleEndian.Uint64(b))
	return r
}

func CryptoRandBytes(n int) []byte {
	b := make([]byte, n)
	_, err := cr.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

var enc16 string = "0123456789abcdef"
var e16 []rune = []rune(enc16)

// nibble must be between 0 and 15 inclusive.
func encode16(nibble byte) rune {
	return e16[nibble]
}

func MathRandHexString(n int) string {
	by := MathRandBytes(n/2 + 1)

	m := len(by)
	p := m - 1
	t := p * 2
	res := make([]rune, m*2)
	for i := 0; i < m; i++ {
		r := byte(by[p-i])
		res[t-i*2] = encode16(r >> 4)
		res[t-i*2+1] = encode16(r & 0x0F)
	}

	// we could be 1 byte larger than need, so
	// truncate here
	s := string(res)[:n]
	//fmt.Printf("source bytes: %x\n", by)
	//fmt.Printf("hexstring conversion: %s\n", s)
	return s
}

func MathRandBytes(n int) []byte {
	b := make([]byte, n)
	_, err := mathRandSrc.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

func CryptoRandNonNegInt(n int64) int64 {
	x := CryptoRandInt64()
	if x < 0 {
		x = -x
	}
	return x % n
}

var ch = []byte("0123456789abcdefghijklmnopqrstuvwxyz")

func RandomString(n int) string {
	s := make([]byte, n)
	m := int64(len(ch))
	for i := 0; i < n; i++ {
		r := CryptoRandInt64()
		if r < 0 {
			r = -r
		}
		k := r % m
		a := ch[k]
		s[i] = a
	}
	return string(s)
}

var chu = []byte("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandomStringWithUp(n int) string {
	s := make([]byte, n)
	m := int64(len(chu))
	for i := 0; i < n; i++ {
		r := CryptoRandInt64()
		if r < 0 {
			r = -r
		}
		k := r % m
		a := chu[k]
		s[i] = a
	}
	return string(s)
}
