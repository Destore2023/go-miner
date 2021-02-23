package ldb

// bl --> bit length

import (
	"encoding/binary"
	"fmt"

	"github.com/Sukhavati-Labs/go-miner/database"
	"github.com/Sukhavati-Labs/go-miner/database/storage"
	"github.com/Sukhavati-Labs/go-miner/errors"
	"github.com/Sukhavati-Labs/go-miner/pocec"
)

var (
	pubKeyBitLengthAndHeightKeyPrefix = "PUBKBL"
	updatePubKeyBitLengthAndHEightKey = []byte("UPPKBL")

	// prefix + pk(compressed)
	pubKeyBitLengthAndHeightKeyPrefixLen = len(pubKeyBitLengthAndHeightKeyPrefix)
	pubKeyBitLengthKeyLen                = pubKeyBitLengthAndHeightKeyPrefixLen + 33

	// bl + blockHeight
	bitLengthAndHeightLen = 1 + 8
)

// makePubKeyBlockKey
// +-------------+---------------------------+
// |  PUBKBL     | poc  publicKey (33 bytes) |
// +-------------+---------------------------+
func makePubKeyBitLengthKey(publicKey *pocec.PublicKey) []byte {
	key := make([]byte, pubKeyBitLengthKeyLen)
	copy(key, pubKeyBitLengthAndHeightKeyPrefix)
	copy(key[pubKeyBitLengthAndHeightKeyPrefixLen:pubKeyBitLengthKeyLen], publicKey.SerializeCompressed())
	return key
}

// serializeBitLenAndBlockHeight
// +--------------------+----------------------------+
// | bitLength(1 byte)  |  blockHeight (8 bytes)     |
// +--------------------+----------------------------+
func serializeBitLengthAndBlockHeight(bitLength uint8, blockHeight uint64) []byte {
	buf := make([]byte, bitLengthAndHeightLen)
	buf[0] = bitLength
	binary.LittleEndian.PutUint64(buf[1:bitLengthAndHeightLen], blockHeight)
	return buf
}

// deserializeBLHeight
func deserializeBLHeight(buf []byte) []*database.BLHeight {
	count := len(buf) / bitLengthAndHeightLen
	bitLengthAndHeights := make([]*database.BLHeight, count)
	for i := 0; i < count; i++ {
		bitLengthAndHeights[i] = &database.BLHeight{
			BitLength:   int(buf[i*bitLengthAndHeightLen]),
			BlockHeight: binary.LittleEndian.Uint64(buf[i*bitLengthAndHeightLen+1 : (i+1)*bitLengthAndHeightLen]),
		}
	}
	return bitLengthAndHeights
}

// publicKey --> (bitLength + blockHeight)
func (db *ChainDb) insertPubKeyBLHeightToBatch(batch storage.Batch, publicKey *pocec.PublicKey, bitLength int, blockHeight uint64) error {
	key := makePubKeyBitLengthKey(publicKey)
	buf := serializeBitLengthAndBlockHeight(uint8(bitLength), blockHeight)
	v, err := db.localStorage.Get(key)
	if err != nil {
		if err == storage.ErrNotFound {
			return batch.Put(key, buf)
		}
		return err
	}
	bitLengthAndHeights := deserializeBLHeight(v)
	lastBitLength := bitLengthAndHeights[len(bitLengthAndHeights)-1].BitLength
	if bitLength < lastBitLength {
		return errors.New(fmt.Sprintf("insertPubkblToBatch: unexpected bitLength %d, last %d, height %d",
			bitLength, lastBitLength, blockHeight))
	}
	if bitLength > lastBitLength {
		v = append(v, buf...)
		return batch.Put(key, v)
	}
	return nil
}

func (db *ChainDb) removePubKeyBitLengthAndHeightWithCheck(batch storage.Batch, publicKey *pocec.PublicKey, bitLength int, blockHeight uint64) error {
	key := makePubKeyBitLengthKey(publicKey)
	buf, err := db.localStorage.Get(key)
	if err != nil {
		return err
	}
	l := len(buf)
	bl := buf[l-bitLengthAndHeightLen]
	h := binary.LittleEndian.Uint64(buf[l-bitLengthAndHeightLen+1:])
	if int(bl) == bitLength && h == blockHeight {
		if l == bitLengthAndHeightLen {
			return batch.Delete(key)
		}
		v := buf[:l-bitLengthAndHeightLen]
		return batch.Put(key, v)
	}
	return nil
}

func (db *ChainDb) GetPubkeyBLHeightRecord(publicKey *pocec.PublicKey) ([]*database.BLHeight, error) {
	key := makePubKeyBitLengthKey(publicKey)
	v, err := db.localStorage.Get(key)
	if err != nil {
		if err == storage.ErrNotFound {
			empty := make([]*database.BLHeight, 0)
			return empty, nil
		}
		return nil, err
	}
	blh := deserializeBLHeight(v)
	return blh, nil
}

func (db *ChainDb) insertPubKeyBLHeight(publicKey *pocec.PublicKey, bitLength int, blockHeight uint64) error {
	key := makePubKeyBitLengthKey(publicKey)
	buf := serializeBitLengthAndBlockHeight(uint8(bitLength), blockHeight)
	v, err := db.localStorage.Get(key)
	if err != nil {
		if err == storage.ErrNotFound {
			return db.localStorage.Put(key, buf)
		}
		return err
	}
	blhs := deserializeBLHeight(v)
	lastBl := blhs[len(blhs)-1].BitLength
	if bitLength < lastBl {
		return errors.New(fmt.Sprintf("insertPubkbl: unexpected bl %d, last %d, height %d",
			bitLength, lastBl, blockHeight))
	}
	if bitLength > lastBl {
		v = append(v, buf...)
		return db.localStorage.Put(key, v)
	}
	return nil
}

func (db *ChainDb) fetchPubKeyBitLengthAndHeightIndexProgress() (uint64, error) {
	buf, err := db.localStorage.Get(updatePubKeyBitLengthAndHEightKey)
	if err != nil {
		if err == storage.ErrNotFound {
			return 0, nil
		}
		return 0, err
	}
	return binary.LittleEndian.Uint64(buf), nil
}

func (db *ChainDb) updatePubKeyBitLengthAndHeightIndexProgress(height uint64) error {
	value := make([]byte, 8)
	binary.LittleEndian.PutUint64(value, height)
	return db.localStorage.Put(updatePubKeyBitLengthAndHEightKey, value)
}

func (db *ChainDb) deletePubKeyBitLengthAndHeightIndexProgress() error {
	return db.localStorage.Delete(updatePubKeyBitLengthAndHEightKey)
}

func (db *ChainDb) clearPubKeyBitLengthAndHeight() error {
	iter := db.localStorage.NewIterator(storage.BytesPrefix([]byte(pubKeyBitLengthAndHeightKeyPrefix)))
	defer iter.Release()

	for iter.Next() {
		err := db.localStorage.Delete(iter.Key())
		if err != nil {
			return err
		}
	}
	return iter.Error()
}
