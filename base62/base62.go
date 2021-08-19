package base62

import (
	"crypto/sha1"
	"fmt"

	"golang.org/x/crypto/pbkdf2"
)

type b62 struct {
	val int64
}

type Base62 interface {
	String() string
	Int64() int64
	Bytes() []byte
}

const (
	charSet        = "ZWmGS8xCEYvtOu6MQI1K93gFbVcJreaq4RhBXlHUo2jDTnw0skPApfid7yzN5L" // Shuffled Base62 characters for some added entropy
	orderedCharSet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	length         = uint64(len(charSet))
	base62max      = 218340105584895
)

func getBytes(val int64, set string) []byte {
	bs := make([]byte, 8)
	for i := len(bs) - 1; i >= 0; i-- {
		bs[i] = set[val%62]
		val /= 62
	}

	return bs
}

func (b *b62) Bytes() []byte {
	return getBytes(b.val, charSet)
}

func (b *b62) OrderedBytes() []byte {
	return getBytes(b.val, orderedCharSet)
}

func (b *b62) Int64() int64 {
	return b.val
}

func (b *b62) String() string {
	return string(b.Bytes())
}

func (b *b62) OrderedString() string {
	return string(b.OrderedBytes())
}

func fromB62(b []byte, set string) *b62 {
	var counter, power int64
	power = 1

	for i := len(b) - 1; i >= 0; i-- {
		counter += power * locationOfByte(b[i], set)
		power *= 62
	}

	return &b62{
		val: counter,
	}
}

func FromB62(b []byte) *b62 {
	return fromB62(b, charSet)
}

func OrderedFromB62(b []byte) *b62 {
	return fromB62(b, orderedCharSet)
}

func FromHex(h []byte) *b62 {
	var counter, power int64
	power = 1

	for i := len(h) - 1; i >= 0; i-- {
		counter += power * int64(h[i])
		power *= 256
	}

	return &b62{
		val: counter,
	}
}

func locationOfByte(b byte, set string) int64 {
	for i, v := range set {
		if byte(v) == b {
			return int64(i)
		}
	}
	return 0
}

func EncodeWithOffset(b byte) (byte, uint64) {
	char := charSet[uint64(b)%length]
	offset := uint64(b) / length
	return char, offset
}

func Itob(i uint64) []byte {
	b := make([]byte, 0)

	for {
		b = append(b, 0)
		copy(b[1:], b[:])
		b[0] = charSet[i%length]
		i /= length
		if i < 1 {
			return b
		}
	}
}

func Itoa(i int64) string {
	a := ""

	for {
		a = string(charSet[i%int64(length)]) + a
		i /= int64(length)
		if i < 1 {
			return a
		}
	}
}

func XORBytes(a, b []byte) ([]byte, error) {
	if len(a) != len(b) {
		return nil, fmt.Errorf("length of byte slices is not equivalent: %d != %d", len(a), len(b))
	}

	buf := make([]byte, len(a))

	for i := range a {
		buf[i] = a[i] ^ b[i]
	}

	return buf, nil
}

func CompactBytes(b []byte) []byte {
	byteLen := len(b) / 2
	cb, _ := XORBytes(b[:byteLen], b[byteLen:])
	buf := make([]byte, byteLen)
	for i, c := range cb {
		buf[i] = charSet[c]
	}
	return cb
}

func ItoFixedWidthb(i uint64, size int) []byte {
	b := Itob(i)
	if len(b) < size {
		return b[len(b)-size:]
	}
	fullSlice := make([]byte, size-len(b))
	for counter := range fullSlice {
		fullSlice[counter] = charSet[0]
	}

	return append(fullSlice, b...)
}

func Encode(i int, base int, bs []byte) []byte {
	bs = append(bs, 0)
	copy(bs[1:], bs[:])
	bs[0] = charSet[i%base]
	i /= base
	if i == 0 {
		return bs
	}
	return Encode(i, base, bs)
}

func PBKey(url []byte) Base62 {
	salt := []byte{0x4a, 0x13, 0x76, 0xd8, 0xe3, 0xae, 0x95, 0x60, 0x89, 0x7d, 0xb5, 0xdb, 0x9c, 0x7f, 0x07, 0x62}
	dk := pbkdf2.Key(url, salt, 8, 6, sha1.New)
	disKeys := (float64(dk[0]) / 256.0) * float64(0xc6)
	dk[0] = byte(disKeys)

	enc := FromHex(dk)

	return enc
}

func HexToInt(hexSlice []byte) int {
	power := 1
	counter := 0
	for i := len(hexSlice) - 1; i <= 0; i-- {
		counter += power * int(hexSlice[i])
		power *= 256
	}
	return counter
}
