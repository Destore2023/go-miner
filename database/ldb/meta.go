package ldb

import (
	"encoding/binary"

	"github.com/Sukhavati-Labs/go-miner/database"
	"github.com/Sukhavati-Labs/go-miner/logging"
	"github.com/Sukhavati-Labs/go-miner/wire"
)

var (
	dbStorageMetaDataKey = []byte("METADATA") // db meta data
)

type dbStorageMeta struct {
	currentHeight uint64    // current block height
	currentHash   wire.Hash // current block hash
}

// decodeDBStorageMetaData - blockStorageMetaDataLength 40 bytes
// +-----------------------+-------------------------+
// | currentHash 32 bytes  | currentHeight 8 bytes   |
// +-----------------------+-------------------------+
func decodeDBStorageMetaData(bs []byte) (meta dbStorageMeta, err error) {
	if length := len(bs); length != blockStorageMetaDataLength {
		logging.CPrint(logging.ERROR, "invalid blockStorageMetaData", logging.LogFormat{"length": length, "data": bs})
		return dbStorageMeta{}, database.ErrInvalidBlockStorageMeta
	}
	copy(meta.currentHash[:], bs[:32])
	meta.currentHeight = binary.LittleEndian.Uint64(bs[32:])
	return meta, nil
}

func encodeDBStorageMetaData(meta dbStorageMeta) []byte {
	bs := make([]byte, blockStorageMetaDataLength)
	copy(bs, meta.currentHash[:])
	binary.LittleEndian.PutUint64(bs[32:], meta.currentHeight)
	return bs
}

func (db *ChainDb) getBlockStorageMeta() (dbStorageMeta, error) {
	if data, err := db.localStorage.Get(dbStorageMetaDataKey); err == nil {
		return decodeDBStorageMetaData(data)
	} else {
		return dbStorageMeta{}, err
	}
}
