package msgp

import (
	"bytes"
	"encoding/binary"
	"math"
	"time"
)

var big = binary.BigEndian

// NextType returns the type of the next
// object in the slice. If the length
// of the input is zero, it returns
// InvalidType.
func NextType(b []byte) Type {
	if len(b) == 0 {
		return InvalidType
	}
	spec := sizes[b[0]]
	t := spec.typ
	if t == ExtensionType && len(b) > int(spec.size) {
		var tp int8
		if spec.extra == constsize {
			tp = int8(b[1])
		} else {
			tp = int8(b[spec.size-1])
		}
		switch tp {
		case TimeExtension:
			return TimeType
		case Complex128Extension:
			return Complex128Type
		case Complex64Extension:
			return Complex64Type
		case DurationExtension:
			return DurationType
		default:
			return ExtensionType
		}
	}
	return t
}

// IsNil returns true if len(b)>0 and
// the leading byte is a 'nil' MessagePack
// byte; false otherwise
func IsNil(b []byte) bool {
	if len(b) != 0 && b[0] == mnil {
		return true
	}
	return false
}

func IsEmptyMap(b []byte) bool {
	if len(b) != 0 && b[0] == mfixmap {
		return true
	}
	return false
}

// DidConsumeNil returns true without changing
// b if nbs.AlwaysNil is true.
//
// Otherwise, if (*b)[0] is 0xc0, then ConsumeNil
// returns true and consumes the nil from (*b).
//
// This convenience method avoids having to
// have separate consume-the-Nil-or-not logic
// after an IsNil call.
//
func (nbs *NilBitsStack) DidConsumeNil(b *[]byte) bool {
	if nbs != nil && nbs.AlwaysNil {
		return true
	}
	if len(*b) != 0 && (*b)[0] == mnil {
		(*b) = (*b)[1:]
		return true
	}
	return false
}

func (nbs *NilBitsStack) PeekNil(b []byte) bool {
	if nbs != nil && nbs.AlwaysNil {
		return true
	}
	if len(b) != 0 && b[0] == mnil {
		return true
	}
	return false
}

// Raw is raw MessagePack.
// Raw allows you to read and write
// data without interpreting its contents.
type Raw []byte

// MarshalMsg implements msgp.Marshaler.
// It appends the raw contents of 'raw'
// to the provided byte slice. If 'raw'
// is 0 bytes, 'nil' will be appended instead.
func (r Raw) MarshalMsg(b []byte) ([]byte, error) {
	i := len(r)
	if i == 0 {
		return AppendNil(b), nil
	}
	o, l := ensure(b, i)
	copy(o[l:], []byte(r))
	return o, nil
}

// UnmarshalMsg implements msgp.Unmarshaler.
// It sets the contents of *Raw to be the next
// object in the provided byte slice.
func (r *Raw) UnmarshalMsg(b []byte) ([]byte, error) {
	l := len(b)
	out, err := Skip(b)
	if err != nil {
		return b, err
	}
	rlen := l - len(out)
	if cap(*r) < rlen {
		*r = make(Raw, rlen)
	} else {
		*r = (*r)[0:rlen]
	}
	copy(*r, b[:rlen])
	return out, nil
}

// EncodeMsg implements msgp.Encodable.
// It writes the raw bytes to the writer.
// If r is empty, it writes 'nil' instead.
func (r Raw) EncodeMsg(w *Writer) error {
	if len(r) == 0 {
		return w.WriteNil()
	}
	_, err := w.Write([]byte(r))
	return err
}

// DecodeMsg implements msgp.Decodable.
// It sets the value of *Raw to be the
// next object on the wire.
func (r *Raw) DecodeMsg(f *Reader) error {
	*r = (*r)[:0]
	return appendNext(f, (*[]byte)(r))
}

// Msgsize implements msgp.Sizer
func (r Raw) Msgsize() int {
	l := len(r)
	if l == 0 {
		return 1 // for 'nil'
	}
	return l
}

func appendNext(f *Reader, d *[]byte) error {
	amt, o, err := getNextSize(f.R)
	if err != nil {
		return err
	}
	var i int
	*d, i = ensure(*d, int(amt))
	_, err = f.R.ReadFull((*d)[i:])
	if err != nil {
		return err
	}
	for o > 0 {
		err = appendNext(f, d)
		if err != nil {
			return err
		}
		o--
	}
	return nil
}

// MarshalJSON implements json.Marshaler
func (r *Raw) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	_, err := UnmarshalAsJSON(&buf, []byte(*r))
	return buf.Bytes(), err
}

// ReadMapHeaderBytes reads a map header size
// from 'b' and returns the remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a map)
func (nbs *NilBitsStack) ReadMapHeaderBytes(b []byte) (sz uint32, o []byte, err error) {
	if nbs != nil && nbs.AlwaysNil {
		return 0, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return 0, b[1:], nil
	}

	l := len(b)
	if l < 1 {
		err = ErrShortBytes
		return
	}

	lead := b[0]
	if isfixmap(lead) {
		sz = uint32(rfixmap(lead))
		o = b[1:]
		return
	}

	switch lead {
	case mmap16:
		if l < 3 {
			err = ErrShortBytes
			return
		}
		sz = uint32(big.Uint16(b[1:]))
		o = b[3:]
		return

	case mmap32:
		if l < 5 {
			err = ErrShortBytes
			return
		}
		sz = big.Uint32(b[1:])
		o = b[5:]
		return

	default:
		err = badPrefix(MapType, lead)
		return
	}
}

// ReadMapKeyZC attempts to read a map key
// from 'b' and returns the key bytes and the remaining bytes
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a str or bin)
func (nbs *NilBitsStack) ReadMapKeyZC(b []byte) ([]byte, []byte, error) {
	if nbs != nil && nbs.AlwaysNil {
		//fmt.Printf("\n ReadMapKeyZC sees nbs.AlwaysNil\n")
		return nil, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		//fmt.Printf("\n ReadMapKeyZC sees mnil as b[0]\n")
		return nil, b[1:], nil
	}
	//fmt.Printf("\n ReadMapKeyZC did not see nil.\n")

	o, b, err := nbs.ReadStringZC(b)
	if err != nil {
		if tperr, ok := err.(TypeError); ok && tperr.Encoded == BinType {
			return nbs.ReadBytesZC(b)
		}
		return nil, b, err
	}
	return o, b, nil
}

// ReadArrayHeaderBytes attempts to read
// the array header size off of 'b' and return
// the size and remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not an array)
func (nbs *NilBitsStack) ReadArrayHeaderBytes(b []byte) (sz uint32, o []byte, err error) {
	if nbs != nil && nbs.AlwaysNil {
		return 0, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return 0, b[1:], nil
	}

	if len(b) < 1 {
		return 0, nil, ErrShortBytes
	}
	lead := b[0]
	if isfixarray(lead) {
		sz = uint32(rfixarray(lead))
		o = b[1:]
		return
	}

	switch lead {
	case marray16:
		if len(b) < 3 {
			err = ErrShortBytes
			return
		}
		sz = uint32(big.Uint16(b[1:]))
		o = b[3:]
		return

	case marray32:
		if len(b) < 5 {
			err = ErrShortBytes
			return
		}
		sz = big.Uint32(b[1:])
		o = b[5:]
		return

	default:
		err = badPrefix(ArrayType, lead)
		return
	}
}

// ReadNilBytes tries to read a "nil" byte
// off of 'b' and return the remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a 'nil')
// - InvalidPrefixError
func (nbs *NilBitsStack) ReadNilBytes(b []byte) ([]byte, error) {
	if nbs != nil && nbs.AlwaysNil {
		return b, nil
	}

	if len(b) < 1 {
		return nil, ErrShortBytes
	}
	if b[0] != mnil {
		return b, badPrefix(NilType, b[0])
	}
	return b[1:], nil
}

// ReadFloat64Bytes tries to read a float64
// from 'b' and return the value and the remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a float64)
func (nbs *NilBitsStack) ReadFloat64Bytes(b []byte) (f float64, o []byte, err error) {
	if nbs != nil && nbs.AlwaysNil {
		return 0, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return 0, b[1:], nil
	}

	if len(b) < 9 {
		if len(b) >= 5 && b[0] == mfloat32 {
			var tf float32
			tf, o, err = nbs.ReadFloat32Bytes(b)
			f = float64(tf)
			return
		}
		err = ErrShortBytes
		return
	}

	if b[0] != mfloat64 {
		if b[0] == mfloat32 {
			var tf float32
			tf, o, err = nbs.ReadFloat32Bytes(b)
			f = float64(tf)
			return
		}
		err = badPrefix(Float64Type, b[0])
		return
	}

	f = math.Float64frombits(getMuint64(b))
	o = b[9:]
	return
}

// ReadFloat32Bytes tries to read a float64
// from 'b' and return the value and the remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a float32)
func (nbs *NilBitsStack) ReadFloat32Bytes(b []byte) (f float32, o []byte, err error) {
	if nbs != nil && nbs.AlwaysNil {
		return 0, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return 0, b[1:], nil
	}

	if len(b) < 5 {
		err = ErrShortBytes
		return
	}

	if b[0] != mfloat32 {
		err = TypeError{Method: Float32Type, Encoded: getType(b[0])}
		return
	}

	f = math.Float32frombits(getMuint32(b))
	o = b[5:]
	return
}

// ReadBoolBytes tries to read a bool
// from 'b' and return the value and the remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a bool)
func (nbs *NilBitsStack) ReadBoolBytes(b []byte) (bool, []byte, error) {
	if nbs != nil && nbs.AlwaysNil {
		return false, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return false, b[1:], nil
	}

	if len(b) < 1 {
		return false, b, ErrShortBytes
	}
	switch b[0] {
	case mtrue:
		return true, b[1:], nil
	case mfalse:
		return false, b[1:], nil
	default:
		return false, b, badPrefix(BoolType, b[0])
	}
}

// ReadInt64Bytes tries to read an int64
// from 'b' and return the value and the remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError (not a int)
func (nbs *NilBitsStack) ReadInt64Bytes(b []byte) (i int64, o []byte, err error) {
	if nbs != nil && nbs.AlwaysNil {
		return 0, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return 0, b[1:], nil
	}

	l := len(b)
	if l < 1 {
		return 0, nil, ErrShortBytes
	}

	lead := b[0]
	if isfixint(lead) {
		i = int64(rfixint(lead))
		o = b[1:]
		return
	}
	if isnfixint(lead) {
		i = int64(rnfixint(lead))
		o = b[1:]
		return
	}

	switch lead {
	case mint8:
		if l < 2 {
			err = ErrShortBytes
			return
		}
		i = int64(getMint8(b))
		o = b[2:]
		return

	case mint16:
		if l < 3 {
			err = ErrShortBytes
			return
		}
		i = int64(getMint16(b))
		o = b[3:]
		return

	case mint32:
		if l < 5 {
			err = ErrShortBytes
			return
		}
		i = int64(getMint32(b))
		o = b[5:]
		return

	case mint64:
		if l < 9 {
			err = ErrShortBytes
			return
		}
		i = getMint64(b)
		o = b[9:]
		return

	default:
		err = badPrefix(IntType, lead)
		return
	}
}

// ReadInt32Bytes tries to read an int32
// from 'b' and return the value and the remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a int)
// - IntOverflow{} (value doesn't fit in int32)
func (nbs *NilBitsStack) ReadInt32Bytes(b []byte) (int32, []byte, error) {
	if nbs != nil && nbs.AlwaysNil {
		return 0, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return 0, b[1:], nil
	}

	i, o, err := nbs.ReadInt64Bytes(b)
	if i > math.MaxInt32 || i < math.MinInt32 {
		return 0, o, IntOverflow{Value: i, FailedBitsize: 32}
	}
	return int32(i), o, err
}

// ReadInt16Bytes tries to read an int16
// from 'b' and return the value and the remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a int)
// - IntOverflow{} (value doesn't fit in int16)
func (nbs *NilBitsStack) ReadInt16Bytes(b []byte) (int16, []byte, error) {
	if nbs != nil && nbs.AlwaysNil {
		return 0, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return 0, b[1:], nil
	}

	i, o, err := nbs.ReadInt64Bytes(b)
	if i > math.MaxInt16 || i < math.MinInt16 {
		return 0, o, IntOverflow{Value: i, FailedBitsize: 16}
	}
	return int16(i), o, err
}

// ReadInt8Bytes tries to read an int16
// from 'b' and return the value and the remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a int)
// - IntOverflow{} (value doesn't fit in int8)
func (nbs *NilBitsStack) ReadInt8Bytes(b []byte) (int8, []byte, error) {
	if nbs != nil && nbs.AlwaysNil {
		return 0, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return 0, b[1:], nil
	}

	i, o, err := nbs.ReadInt64Bytes(b)
	if i > math.MaxInt8 || i < math.MinInt8 {
		return 0, o, IntOverflow{Value: i, FailedBitsize: 8}
	}
	return int8(i), o, err
}

// ReadIntBytes tries to read an int
// from 'b' and return the value and the remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a int)
// - IntOverflow{} (value doesn't fit in int; 32-bit platforms only)
func (nbs *NilBitsStack) ReadIntBytes(b []byte) (int, []byte, error) {
	if nbs != nil && nbs.AlwaysNil {
		return 0, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return 0, b[1:], nil
	}

	if smallint {
		i, b, err := nbs.ReadInt32Bytes(b)
		return int(i), b, err
	}
	i, b, err := nbs.ReadInt64Bytes(b)
	return int(i), b, err
}

// ReadUint64Bytes tries to read a uint64
// from 'b' and return the value and the remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a uint)
func (nbs *NilBitsStack) ReadUint64Bytes(b []byte) (u uint64, o []byte, err error) {
	if nbs != nil && nbs.AlwaysNil {
		return 0, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return 0, b[1:], nil
	}

	l := len(b)
	if l < 1 {
		return 0, nil, ErrShortBytes
	}

	lead := b[0]
	if isfixint(lead) {
		u = uint64(rfixint(lead))
		o = b[1:]
		return
	}

	switch lead {
	case muint8:
		if l < 2 {
			err = ErrShortBytes
			return
		}
		u = uint64(getMuint8(b))
		o = b[2:]
		return

	case muint16:
		if l < 3 {
			err = ErrShortBytes
			return
		}
		u = uint64(getMuint16(b))
		o = b[3:]
		return

	case muint32:
		if l < 5 {
			err = ErrShortBytes
			return
		}
		u = uint64(getMuint32(b))
		o = b[5:]
		return

	case muint64:
		if l < 9 {
			err = ErrShortBytes
			return
		}
		u = getMuint64(b)
		o = b[9:]
		return

	default:
		err = badPrefix(UintType, lead)
		return
	}
}

// ReadUint32Bytes tries to read a uint32
// from 'b' and return the value and the remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a uint)
// - UintOverflow{} (value too large for uint32)
func (nbs *NilBitsStack) ReadUint32Bytes(b []byte) (uint32, []byte, error) {
	if nbs != nil && nbs.AlwaysNil {
		return 0, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return 0, b[1:], nil
	}

	v, o, err := nbs.ReadUint64Bytes(b)
	if v > math.MaxUint32 {
		return 0, nil, UintOverflow{Value: v, FailedBitsize: 32}
	}
	return uint32(v), o, err
}

// ReadUint16Bytes tries to read a uint16
// from 'b' and return the value and the remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a uint)
// - UintOverflow{} (value too large for uint16)
func (nbs *NilBitsStack) ReadUint16Bytes(b []byte) (uint16, []byte, error) {
	if nbs != nil && nbs.AlwaysNil {
		return 0, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return 0, b[1:], nil
	}

	v, o, err := nbs.ReadUint64Bytes(b)
	if v > math.MaxUint16 {
		return 0, nil, UintOverflow{Value: v, FailedBitsize: 16}
	}
	return uint16(v), o, err
}

// ReadUint8Bytes tries to read a uint8
// from 'b' and return the value and the remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a uint)
// - UintOverflow{} (value too large for uint8)
func (nbs *NilBitsStack) ReadUint8Bytes(b []byte) (uint8, []byte, error) {
	if nbs != nil && nbs.AlwaysNil {
		return 0, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return 0, b[1:], nil
	}

	v, o, err := nbs.ReadUint64Bytes(b)
	if v > math.MaxUint8 {
		return 0, nil, UintOverflow{Value: v, FailedBitsize: 8}
	}
	return uint8(v), o, err
}

// ReadUintBytes tries to read a uint
// from 'b' and return the value and the remaining bytes.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a uint)
// - UintOverflow{} (value too large for uint; 32-bit platforms only)
func (nbs *NilBitsStack) ReadUintBytes(b []byte) (uint, []byte, error) {
	if nbs != nil && nbs.AlwaysNil {
		return 0, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return 0, b[1:], nil
	}

	if smallint {
		u, b, err := nbs.ReadUint32Bytes(b)
		return uint(u), b, err
	}
	u, b, err := nbs.ReadUint64Bytes(b)
	return uint(u), b, err
}

// ReadByteBytes is analogous to ReadUint8Bytes
func (nbs *NilBitsStack) ReadByteBytes(b []byte) (byte, []byte, error) {
	if nbs != nil && nbs.AlwaysNil {
		return 0, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return 0, b[1:], nil
	}

	return nbs.ReadUint8Bytes(b)
}

// ReadBytesBytes reads a 'bin' object
// from 'b' and returns its vaue and
// the remaining bytes in 'b'.
// Possible errors:
// - ErrShortBytes (too few bytes)
// - TypeError{} (not a 'bin' object)
func (nbs *NilBitsStack) ReadBytesBytes(b []byte, scratch []byte) (v []byte, o []byte, err error) {
	if nbs != nil && nbs.AlwaysNil {
		return nil, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return nil, b[1:], nil
	}

	return readBytesBytes(b, scratch, false)
}

func readBytesBytes(b []byte, scratch []byte, zc bool) (v []byte, o []byte, err error) {
	l := len(b)
	if l < 1 {
		return nil, nil, ErrShortBytes
	}

	lead := b[0]
	var read int
	switch lead {
	case mbin8:
		if l < 2 {
			err = ErrShortBytes
			return
		}

		read = int(b[1])
		b = b[2:]

	case mbin16:
		if l < 3 {
			err = ErrShortBytes
			return
		}
		read = int(big.Uint16(b[1:]))
		b = b[3:]

	case mbin32:
		if l < 5 {
			err = ErrShortBytes
			return
		}
		read = int(big.Uint32(b[1:]))
		b = b[5:]

	default:
		err = badPrefix(BinType, lead)
		return
	}

	if len(b) < read {
		err = ErrShortBytes
		return
	}

	// zero-copy
	if zc {
		v = b[0:read]
		o = b[read:]
		return
	}

	if cap(scratch) >= read {
		v = scratch[0:read]
	} else {
		v = make([]byte, read)
	}

	o = b[copy(v, b):]
	return
}

// ReadBytesZC extracts the messagepack-encoded
// binary field without copying. The returned []byte
// points to the same memory as the input slice.
// Possible errors:
// - ErrShortBytes (b not long enough)
// - TypeError{} (object not 'bin')
func (nbs *NilBitsStack) ReadBytesZC(b []byte) (v []byte, o []byte, err error) {
	if nbs != nil && nbs.AlwaysNil {
		return nil, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return nil, b[1:], nil
	}

	return readBytesBytes(b, nil, true)
}

func (nbs *NilBitsStack) ReadExactBytes(b []byte, into []byte) (o []byte, err error) {
	if nbs != nil && nbs.AlwaysNil {
		for i := range into {
			into[i] = 0
		}
		return b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		for i := range into {
			into[i] = 0
		}
		return b[1:], nil
	}

	l := len(b)
	if l < 1 {
		err = ErrShortBytes
		return
	}

	lead := b[0]
	var read uint32
	var skip int
	switch lead {
	case mbin8:
		if l < 2 {
			err = ErrShortBytes
			return
		}

		read = uint32(b[1])
		skip = 2

	case mbin16:
		if l < 3 {
			err = ErrShortBytes
			return
		}
		read = uint32(big.Uint16(b[1:]))
		skip = 3

	case mbin32:
		if l < 5 {
			err = ErrShortBytes
			return
		}
		read = uint32(big.Uint32(b[1:]))
		skip = 5

	default:
		err = badPrefix(BinType, lead)
		return
	}

	if read != uint32(len(into)) {
		err = ArrayError{Wanted: uint32(len(into)), Got: read}
		return
	}

	o = b[skip+copy(into, b[skip:]):]
	return
}

// ReadStringZC reads a messagepack string field
// without copying. The returned []byte points
// to the same memory as the input slice.
// Possible errors:
// - ErrShortBytes (b not long enough)
// - TypeError{} (object not 'str')
func (nbs *NilBitsStack) ReadStringZC(b []byte) (v []byte, o []byte, err error) {
	if nbs != nil && nbs.AlwaysNil {
		return nil, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return nil, b[1:], nil
	}

	l := len(b)
	if l < 1 {
		return nil, nil, ErrShortBytes
	}

	lead := b[0]
	var read int

	if isfixstr(lead) {
		read = int(rfixstr(lead))
		b = b[1:]
	} else {
		switch lead {
		case mstr8:
			if l < 2 {
				err = ErrShortBytes
				return
			}
			read = int(b[1])
			b = b[2:]

		case mstr16:
			if l < 3 {
				err = ErrShortBytes
				return
			}
			read = int(big.Uint16(b[1:]))
			b = b[3:]

		case mstr32:
			if l < 5 {
				err = ErrShortBytes
				return
			}
			read = int(big.Uint32(b[1:]))
			b = b[5:]

		default:
			err = TypeError{Method: StrType, Encoded: getType(lead)}
			return
		}
	}

	if len(b) < read {
		err = ErrShortBytes
		return
	}

	v = b[0:read]
	o = b[read:]
	return
}

// ReadStringBytes reads a 'str' object
// from 'b' and returns its value and the
// remaining bytes in 'b'.
// Possible errors:
// - ErrShortBytes (b not long enough)
// - TypeError{} (not 'str' type)
// - InvalidPrefixError
func (nbs *NilBitsStack) ReadStringBytes(b []byte) (string, []byte, error) {
	if nbs != nil && nbs.AlwaysNil {
		return "", b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return "", b[1:], nil
	}
	v, o, err := nbs.ReadStringZC(b)
	if err != nil && len(v) > 0 && nbs != nil && nbs.UnsafeZeroCopy {
		return UnsafeString(v), o, err
	}
	return string(v), o, err
}

// ReadStringAsBytes reads a 'str' object
// into a slice of bytes. 'v' is the value of
// the 'str' object, which may reside in memory
// pointed to by 'scratch.' 'o' is the remaining bytes
// in 'b.''
// Possible errors:
// - ErrShortBytes (b not long enough)
// - TypeError{} (not 'str' type)
// - InvalidPrefixError (unknown type marker)
func (nbs *NilBitsStack) ReadStringAsBytes(b []byte, scratch []byte) (v []byte, o []byte, err error) {
	if nbs != nil && nbs.AlwaysNil {
		return nil, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return nil, b[1:], nil
	}

	var tmp []byte
	tmp, o, err = nbs.ReadStringZC(b)
	v = append(scratch[:0], tmp...)
	return
}

// ReadComplex128Bytes reads a complex128
// extension object from 'b' and returns the
// remaining bytes.
// Possible errors:
// - ErrShortBytes (not enough bytes in 'b')
// - TypeError{} (object not a complex128)
// - InvalidPrefixError
// - ExtensionTypeError{} (object an extension of the correct size, but not a complex128)
func (nbs *NilBitsStack) ReadComplex128Bytes(b []byte) (c complex128, o []byte, err error) {
	if nbs != nil && nbs.AlwaysNil {
		return 0, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return 0, b[1:], nil
	}

	if len(b) < 18 {
		err = ErrShortBytes
		return
	}
	if b[0] != mfixext16 {
		err = badPrefix(Complex128Type, b[0])
		return
	}
	if int8(b[1]) != Complex128Extension {
		err = errExt(int8(b[1]), Complex128Extension)
		return
	}
	c = complex(math.Float64frombits(big.Uint64(b[2:])),
		math.Float64frombits(big.Uint64(b[10:])))
	o = b[18:]
	return
}

// ReadComplex64Bytes reads a complex64
// extension object from 'b' and returns the
// remaining bytes.
// Possible errors:
// - ErrShortBytes (not enough bytes in 'b')
// - TypeError{} (object not a complex64)
// - ExtensionTypeError{} (object an extension of the correct size, but not a complex64)
func (nbs *NilBitsStack) ReadComplex64Bytes(b []byte) (c complex64, o []byte, err error) {
	if nbs != nil && nbs.AlwaysNil {
		return 0, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return 0, b[1:], nil
	}

	if len(b) < 10 {
		err = ErrShortBytes
		return
	}
	if b[0] != mfixext8 {
		err = badPrefix(Complex64Type, b[0])
		return
	}
	if b[1] != Complex64Extension {
		err = errExt(int8(b[1]), Complex64Extension)
		return
	}
	c = complex(math.Float32frombits(big.Uint32(b[2:])),
		math.Float32frombits(big.Uint32(b[6:])))
	o = b[10:]
	return
}

// ReadTimeBytes reads a time.Time
// extension object from 'b' and returns the
// remaining bytes.
// Possible errors:
// - ErrShortBytes (not enough bytes in 'b')
// - TypeError{} (object not a complex64)
// - ExtensionTypeError{} (object an extension of the correct size, but not a time.Time)
func (nbs *NilBitsStack) ReadTimeBytes(b []byte) (t time.Time, o []byte, err error) {
	if nbs != nil && nbs.AlwaysNil {
		return time.Time{}, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return time.Time{}, b[1:], nil
	}

	if len(b) < 15 {
		err = ErrShortBytes
		return
	}
	if b[0] != mext8 || b[1] != 12 {
		err = badPrefix(TimeType, b[0])
		return
	}
	if int8(b[2]) != TimeExtension {
		err = errExt(int8(b[2]), TimeExtension)
		return
	}
	sec, nsec := getUnix(b[3:])
	t = time.Unix(sec, int64(nsec)).Local()
	o = b[15:]
	return
}

// ReadDurationBytes reads a time.Duration
// extension object from 'b' and returns the
// remaining bytes.
// Possible errors:
// - ErrShortBytes (not enough bytes in 'b')
// - TypeError{} (object not a complex64)
// - ExtensionTypeError{} (object an extension of the correct size, but not a time.Duration)
func (nbs *NilBitsStack) ReadDurationBytes(b []byte) (t time.Duration, o []byte, err error) {
	if nbs != nil && nbs.AlwaysNil {
		return 0, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return 0, b[1:], nil
	}

	if len(b) < 4 {
		err = ErrShortBytes
		return
	}

	if b[0] != mext8 {
		err = badPrefix(DurationType, b[0])
		return
	}
	n := int(b[1])
	if n > 9 {
		// type error of expected Duration and got Duration will
		// have to mean this byte count was way out of line.
		err = badPrefix(DurationType, b[0])
		return
	}
	if len(b) < n+3 {
		err = ErrShortBytes
		return
	}
	if int8(b[2]) != DurationExtension {
		err = errExt(int8(b[2]), DurationExtension)
		return
	}
	var n64 int64
	n64, o, err = nbs.ReadInt64Bytes(b[3:(3 + n)])
	if err != nil {
		return
	}
	t = time.Duration(n64)
	o = b[3+n:]
	return
}

// ReadMapStrIntfBytes reads a map[string]interface{}
// out of 'b' and returns the map and remaining bytes.
// If 'old' is non-nil, the values will be read into that map.
func (nbs *NilBitsStack) ReadMapStrIntfBytes(b []byte, old map[string]interface{}) (v map[string]interface{}, o []byte, err error) {
	if nbs != nil && nbs.AlwaysNil {
		if old != nil {
			for key := range old {
				delete(old, key)
			}
		}
		return old, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		if old != nil {
			for key := range old {
				delete(old, key)
			}
		}
		return old, b[1:], nil
	}

	var sz uint32
	o = b
	sz, o, err = nbs.ReadMapHeaderBytes(o)

	if err != nil {
		return
	}

	if old != nil {
		for key := range old {
			delete(old, key)
		}
		v = old
	} else {
		v = make(map[string]interface{}, int(sz))
	}

	for z := uint32(0); z < sz; z++ {
		if len(o) < 1 {
			err = ErrShortBytes
			return
		}
		var key []byte
		key, o, err = nbs.ReadMapKeyZC(o)
		if err != nil {
			return
		}
		var val interface{}
		val, o, err = nbs.ReadIntfBytes(o)
		if err != nil {
			return
		}
		v[string(key)] = val
	}
	return
}

// ReadIntfBytes attempts to read
// the next object out of 'b' as a raw interface{} and
// return the remaining bytes.
func (nbs *NilBitsStack) ReadIntfBytes(b []byte) (i interface{}, o []byte, err error) {
	if nbs != nil && nbs.AlwaysNil {
		return nil, b, nil
	}
	if len(b) != 0 && b[0] == mnil {
		return nil, b[1:], nil
	}

	if len(b) < 1 {
		err = ErrShortBytes
		return
	}

	k := NextType(b)

	switch k {
	case MapType:
		i, o, err = nbs.ReadMapStrIntfBytes(b, nil)
		return

	case ArrayType:
		var sz uint32
		sz, o, err = nbs.ReadArrayHeaderBytes(b)
		if err != nil {
			return
		}
		j := make([]interface{}, int(sz))
		i = j
		for d := range j {
			j[d], o, err = nbs.ReadIntfBytes(o)
			if err != nil {
				return
			}
		}
		return

	case Float32Type:
		i, o, err = nbs.ReadFloat32Bytes(b)
		return

	case Float64Type:
		i, o, err = nbs.ReadFloat64Bytes(b)
		return

	case IntType:
		i, o, err = nbs.ReadInt64Bytes(b)
		return

	case UintType:
		i, o, err = nbs.ReadUint64Bytes(b)
		return

	case BoolType:
		i, o, err = nbs.ReadBoolBytes(b)
		return

	case TimeType:
		i, o, err = nbs.ReadTimeBytes(b)
		return

	case DurationType:
		i, o, err = nbs.ReadDurationBytes(b)
		return

	case Complex64Type:
		i, o, err = nbs.ReadComplex64Bytes(b)
		return

	case Complex128Type:
		i, o, err = nbs.ReadComplex128Bytes(b)
		return

	case ExtensionType:
		var t int8
		t, err = peekExtension(b)
		if err != nil {
			return
		}
		// use a user-defined extension,
		// if it's been registered
		f, ok := extensionReg[t]
		if ok {
			e := f()
			o, err = nbs.ReadExtensionBytes(b, e)
			i = e
			return
		}
		// last resort is a raw extension
		e := RawExtension{}
		e.Type = int8(t)
		o, err = nbs.ReadExtensionBytes(b, &e)
		i = &e
		return

	case NilType:
		o, err = nbs.ReadNilBytes(b)
		return

	case BinType:
		i, o, err = nbs.ReadBytesBytes(b, nil)
		return

	case StrType:
		i, o, err = nbs.ReadStringBytes(b)
		return

	default:
		err = InvalidPrefixError(b[0])
		return
	}
}

// Skip skips the next object in 'b' and
// returns the remaining bytes. If the object
// is a map or array, all of its elements
// will be skipped.
// Possible Errors:
// - ErrShortBytes (not enough bytes in b)
// - InvalidPrefixError (bad encoding)
func Skip(b []byte) ([]byte, error) {
	sz, asz, err := getSize(b)
	if err != nil {
		return b, err
	}
	if uintptr(len(b)) < sz {
		return b, ErrShortBytes
	}
	b = b[sz:]
	for asz > 0 {
		b, err = Skip(b)
		if err != nil {
			return b, err
		}
		asz--
	}
	return b, nil
}

// returns (skip N bytes, skip M objects, error)
func getSize(b []byte) (uintptr, uintptr, error) {
	l := len(b)
	if l == 0 {
		return 0, 0, ErrShortBytes
	}
	lead := b[0]
	spec := &sizes[lead] // get type information
	size, mode := spec.size, spec.extra
	if size == 0 {
		return 0, 0, InvalidPrefixError(lead)
	}
	if mode >= 0 { // fixed composites
		return uintptr(size), uintptr(mode), nil
	}
	if l < int(size) {
		return 0, 0, ErrShortBytes
	}
	switch mode {
	case extra8:
		return uintptr(size) + uintptr(b[1]), 0, nil
	case extra16:
		return uintptr(size) + uintptr(big.Uint16(b[1:])), 0, nil
	case extra32:
		return uintptr(size) + uintptr(big.Uint32(b[1:])), 0, nil
	case map16v:
		return uintptr(size), 2 * uintptr(big.Uint16(b[1:])), nil
	case map32v:
		return uintptr(size), 2 * uintptr(big.Uint32(b[1:])), nil
	case array16v:
		return uintptr(size), uintptr(big.Uint16(b[1:])), nil
	case array32v:
		return uintptr(size), uintptr(big.Uint32(b[1:])), nil
	default:
		return 0, 0, fatal
	}
}

// NextTypeName inspects the next time, assuming it is a
// struct (map) for its (-1: name) key-value pair, and returns the name
// or empty string if not found. Also empty string if not
// a map type.
func (nbs *NilBitsStack) NextStructName(o []byte) (string, []byte) {
	ty := NextType(o)
	if ty != MapType {
		return "", o
	}
	if len(o) < 3 {
		return "", o
	}

	// map header can be of varying size
	hdsz := 1
	p := o[:2]
	lead := p[0]
	if isfixmap(lead) {
		hdsz = 1
	} else {
		switch lead {
		case mmap16:
			hdsz = 3
		case mmap32:
			hdsz = 5
		default:
			//err = badPrefix(MapType, lead)
			return "", o
		}
	}

	// now we've got the map header out of the
	// way in hdsz bytes
	p = o[:hdsz+3]
	keystart := p[hdsz:]
	if keystart[0] != 0xa1 || keystart[1] != 0x40 {
		return "", o // not the "@" key with value the struct name
	}

	valstart := hdsz + 2
	valTypeBytes := 1
	skip := valstart + valTypeBytes
	lead = p[valstart]
	var read int
	if isfixstr(lead) {
		// lead is a fixstr, good
		read = int(rfixstr(lead))
	} else {
		switch lead {
		case mstr8, mbin8:
			valTypeBytes = 2
			skip = valstart + valTypeBytes
			p = o[:skip]
			read = int(p[valstart+1])
		case mstr16, mbin16:
			valTypeBytes = 3
			skip = valstart + valTypeBytes
			p = o[:skip]
			read = int(big.Uint16(p[valstart+1:]))
		case mstr32, mbin32:
			valTypeBytes = 5
			skip = valstart + valTypeBytes
			p = o[:skip]
			read = int(big.Uint32(p[valstart+1:]))
		default:
			return "", o // not a string
		}
	}
	// read string strictly through peaking!
	if read == 0 {
		return "", o
	}
	if len(o) < skip+read {
		return "", o
	}

	//	return string(o[skip : skip+read]), o[skip+read:]
	return string(o[skip : skip+read]), o
}
