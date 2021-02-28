package pocutil

import (
	"encoding/binary"
	"math/big"

	"github.com/Sukhavati-Labs/go-miner/pocec"
)

// CutBigInt cuts big.Int to uint64, masking result to bitLength value.
func CutBigInt(bi *big.Int, bl int) PoCValue {
	mask := PoCValue((1 << uint(bl)) - 1)
	return mask & PoCValue(bi.Uint64())
}

// CutHash cuts the least significant 8 bytes of Hash, decoding it
// as little endian uint64, and masking result to bitLength value.
func CutHash(hash Hash, bl int) PoCValue {
	mask := PoCValue((1 << uint(bl)) - 1)
	h64 := binary.LittleEndian.Uint64(hash[:8])
	return mask & PoCValue(h64)
}

// FlipValue flips(bit-flip) the bitLength bits of a PoCValue.
func FlipValue(v PoCValue, bl int) PoCValue {
	mask := PoCValue((1 << uint(bl)) - 1)
	return mask & (mask ^ v)
}

// PoCValue2Bytes encodes a PoCValue to bytes,
// size depending on its bitLength.
func PoCValue2Bytes(v PoCValue, bitLength int) []byte {
	size := RecordSize(bitLength)
	mask := PoCValue((1 << uint(bitLength)) - 1)
	v = mask & v
	var vb [8]byte
	binary.LittleEndian.PutUint64(vb[:], uint64(v))
	return vb[:size]
}

// Bytes2PoCValue decodes a PoCValue from bytes,
// size depending on its bitLength.
func Bytes2PoCValue(vb []byte, bl int) PoCValue {
	mask := PoCValue((1 << uint(bl)) - 1)
	var b8 [8]byte
	copy(b8[:], vb)
	v := PoCValue(binary.LittleEndian.Uint64(b8[:8]))
	return mask & v
}

// RecordSize returns the size of a bitLength record,
// in byte unit.
// mask 7   -> 0000 0111   +                >>3
//      24  -> 0001 1000   -> 31  0001 1111     3 0000 0011
//      26  -> 0001 1010   -> 33  0010 0001     4 0000 0100
//      28  -> 0001 1100   -> 35  0010 0011     4 0000 0100
//      32  -> 0010 0000   -> 39  0010 0111     4 0000 0100
//      34  -> 0010 0010   -> 41  0010 1001     5 0000 0101
//      36  -> 0010 0100   -> 43  0010 1011     5 0000 0101
//      38  -> 0010 0110   -> 45  0010 1101     5 0000 0101
//      40  -> 0010 1000   -> 47  0010 1111     5 0000 0101
func RecordSize(bitLength int) int {
	return (bitLength + 7) >> 3
}

// NormalizePoCBytes sets vb and returns vb according to its bitLength,
// upper bits would be set to zero.
// 0              size -1    size
// +----------------+--------+-------------+
// |                | 8 bits |      0      |
// +----------------+--------+-------------+
func NormalizePoCBytes(vb []byte, bitLength int) []byte {
	size := RecordSize(bitLength)
	if len(vb) < size {
		return vb
	}
	//
	// 24%8 = 0
	// 26%8 = 2  -->  1   0000 0001
	// 28%8 = 4       7   0000 0111
	// 30%8 = 6       31  0001 1111
	// 32%8 = 0
	// 34%8 = 2       1   0000 0001
	// 36%8 = 4       7   0000 0111
	// 38%8 = 6       31  0001 1111
	// 40%8 = 0
	if rem := bitLength % 8; rem != 0 {
		vb[size-1] &= 1<<uint(rem) - 1
	}
	for i := size; i < len(vb); i++ {
		vb[i] = 0
	}
	return vb
}

// PubKeyHash returns the hash of pubKey.
func PubKeyHash(pubKey *pocec.PublicKey) Hash {
	if pubKey == nil {
		return Hash{}
	}
	return DoubleSHA256(pubKey.SerializeCompressed())
}
