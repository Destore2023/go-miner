package ldb

import (
	"bytes"
	"encoding/binary"

	"github.com/Sukhavati-Labs/go-miner/database/storage"
	"github.com/Sukhavati-Labs/go-miner/logging"
	"github.com/Sukhavati-Labs/go-miner/wire"
)

var (
	faultPubKeyShaDataPrefix   = []byte("BANPUB") // Sha to Data
	faultPubKeyHeightShaPrefix = []byte("BANHGT") // Height to Sha(s)
)

func shaFaultPubKeyToKey(sha *wire.Hash) []byte {
	key := make([]byte, len(sha)+len(faultPubKeyShaDataPrefix))
	copy(key, faultPubKeyShaDataPrefix)
	copy(key[len(faultPubKeyShaDataPrefix):], sha[:])
	return key
}

func faultPubKeyHeightToKey(blockHeight uint64) []byte {
	var b8 [8]byte
	binary.LittleEndian.PutUint64(b8[0:8], blockHeight)
	key := make([]byte, 8+len(faultPubKeyHeightShaPrefix))
	copy(key, faultPubKeyHeightShaPrefix)
	copy(key[len(faultPubKeyHeightShaPrefix):], b8[:])
	return key
}

//  Structure of fault PubKey Data
//   ------------------------------------------------------------
//  | Appear Height | PubKey Bytes | Testimony 01 | Testimony 02 |
//  |---------------------------------------------|--------------
//  |    8 Byte     |    33 Byte   | HeaderLength | HeaderLength |
//   ------------------------------------------------------------

// FetchFaultPkBySha - return a banned pubKey along with corresponding testimony
func (db *ChainDb) FetchFaultPubKeyBySha(sha *wire.Hash) (fpk *wire.FaultPubKey, height uint64, err error) {
	return db.fetchFaultPubKeyBySha(sha)
}

// FetchFaultPkBySha - return a banned pubKey along with corresponding testimony
// Must be called with db lock held.
func (db *ChainDb) fetchFaultPubKeyBySha(sha *wire.Hash) (fpk *wire.FaultPubKey, height uint64, err error) {
	height, data, err := db.getFaultPkData(sha)
	if err != nil {
		return
	}
	fpk, err = wire.NewFaultPubKeyFromBytes(data, wire.DB)
	return
}

func (db *ChainDb) FetchAllFaultPubKeys() ([]*wire.FaultPubKey, []uint64, error) {

	return db.fetchAllFaultPubKeys()
}

func (db *ChainDb) fetchAllFaultPubKeys() ([]*wire.FaultPubKey, []uint64, error) {
	fpkList := make([]*wire.FaultPubKey, 0)
	heightList := make([]uint64, 0)
	iter := db.localStorage.NewIterator(storage.BytesPrefix(faultPubKeyShaDataPrefix))
	defer iter.Release()
	for iter.Next() {
		height := binary.LittleEndian.Uint64(iter.Value()[:8])
		heightList = append(heightList, height)
		fpk, err := wire.NewFaultPubKeyFromBytes(iter.Value()[41:], wire.DB)
		if err != nil {
			return nil, nil, err
		}
		fpkList = append(fpkList, fpk)
	}
	if err := iter.Error(); err != nil {
		return nil, nil, err
	}
	return fpkList, heightList, nil
}

// FetchFaultPkListByHeight - return newly banned PubKey list on specific height
func (db *ChainDb) FetchFaultPubKeyListByHeight(blockHeight uint64) ([]*wire.FaultPubKey, error) {
	return db.fetchFaultPubKeyListByHeight(blockHeight)
}

// FetchFaultPkListByHeight - return newly banned PubKey list on specific height
// Must be called with db lock held.
func (db *ChainDb) fetchFaultPubKeyListByHeight(blockHeight uint64) ([]*wire.FaultPubKey, error) {
	bufArr, err := db.getFaultPkDataByHeight(blockHeight)
	if err != nil {
		return nil, err
	}
	pkList := make([]*wire.FaultPubKey, 0, len(bufArr))
	for _, buf := range bufArr {
		fpk, err := wire.NewFaultPubKeyFromBytes(buf, wire.DB)
		if err != nil {
			return nil, err
		}
		pkList = append(pkList, fpk)
	}
	return pkList, nil
}

// getFaultPkLoc - return since which height is this PubKey banned
// Must be called with db lock held.
func (db *ChainDb) getFaultPkLoc(sha *wire.Hash) (uint64, error) {
	height, _, err := db.getFaultPkData(sha)
	return height, err
}

// Structure of height to fault PK sha List
//   -----------------------------------
//  | Total Count | PubKey Sha | ...... |
//  |-------------|------------|--------|
//  |    2 Byte   |   32 Byte  | ...... |
//   -----------------------------------
// Total Count may be Zero, thus the Length = 2 Bytes + Count * 32 Bytes

// getFaultPkDataByHeight - return FaultPkData List by height
// Must be called with db lock held.
func (db *ChainDb) getFaultPkDataByHeight(blockHeight uint64) ([][]byte, error) {
	shaList, err := db.getFaultPubKeyShasByHeight(blockHeight)
	if err != nil {
		return nil, err
	}
	bufArr := make([][]byte, 0)
	for _, sha := range shaList {
		_, buf, err := db.getFaultPkData(sha)
		if err != nil {
			return nil, err
		}
		bufArr = append(bufArr, buf)
	}
	return bufArr, nil
}

// getFaultPkShasByHeight - return FaultPk List by height
// Must be called with db lock held.
func (db *ChainDb) getFaultPubKeyShasByHeight(blockHeight uint64) ([]*wire.Hash, error) {
	index := faultPubKeyHeightToKey(blockHeight)
	data, err := db.localStorage.Get(index)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil, nil
		}
		logging.CPrint(logging.TRACE, "failed to find faultPk list on block height", logging.LogFormat{"height": blockHeight})
		return nil, err
	}

	count := binary.LittleEndian.Uint16(data[:2])

	data = data[2:]

	shaList := make([]*wire.Hash, 0, count)

	for i := uint16(0); i < count; i++ {
		sha, err := wire.NewHash(data[i*32 : (i+1)*32])
		if err != nil {
			return nil, err
		}
		shaList = append(shaList, sha)
	}
	return shaList, nil

}

// getFaultPkData - return FaultPkData by sha
// Must be called with db lock held.
func (db *ChainDb) getFaultPkData(sha *wire.Hash) (uint64, []byte, error) {
	index := shaFaultPubKeyToKey(sha)
	faultPKData, err := db.localStorage.Get(index)
	if err != nil {
		logging.CPrint(logging.TRACE, "failed to find faultPk by hash", logging.LogFormat{"hash": sha.String()})
		return 0, nil, err
	}

	height := binary.LittleEndian.Uint64(faultPKData[:8])

	return height, faultPKData[41:], nil
}

// insertFaultPks - insert newly banned faultPk list on specific height
// Must be called with db lock held.
// DEPRECATED since version 1.1.0
func insertFaultPks(batch storage.Batch, blockHeight uint64, faultPks []*wire.FaultPubKey) error {
	count := len(faultPks)
	var b2 [2]byte
	binary.LittleEndian.PutUint16(b2[0:2], uint16(count))

	var shaListData bytes.Buffer
	shaListData.Write(b2[:])

	for _, fpk := range faultPks {
		sha := wire.DoubleHashH(fpk.PubKey.SerializeUncompressed())
		shaListData.Write(sha.Bytes())
		err := insertFaultPk(batch, blockHeight, fpk, &sha)
		if err != nil {
			return err
		}
	}

	if count > 0 {
		heightIndex := faultPubKeyHeightToKey(uint64(blockHeight))
		return batch.Put(heightIndex, shaListData.Bytes())
	}
	return nil
}

// insertFaultPk - insert newly banned faultPk on specific height
// Must be called with db lock held.
func insertFaultPk(batch storage.Batch, blockHeight uint64, faultPk *wire.FaultPubKey, sha *wire.Hash) error {
	data, err := faultPk.Bytes(wire.DB)
	if err != nil {
		return err
	}

	var lh [8]byte
	binary.LittleEndian.PutUint64(lh[0:8], blockHeight)

	var buf bytes.Buffer
	buf.Write(lh[:])
	buf.Write(faultPk.PubKey.SerializeCompressed())
	buf.Write(data)

	key := shaFaultPubKeyToKey(sha)
	return batch.Put(key, buf.Bytes())
}

func (db *ChainDb) dropFaultPksByHeight(batch storage.Batch, blockHeight uint64) error {
	shaList, err := db.getFaultPubKeyShasByHeight(blockHeight)
	if err != nil {
		return err
	}
	for _, sha := range shaList {
		err := dropFaultPkBySha(batch, sha)
		if err != nil {
			return err
		}
	}
	if len(shaList) > 0 {
		index := faultPubKeyHeightToKey(blockHeight)
		return batch.Delete(index)
	}
	return nil
}

func dropFaultPkBySha(batch storage.Batch, sha *wire.Hash) error {
	index := shaFaultPubKeyToKey(sha)
	return batch.Delete(index)
}

// ExistsFaultPk - check whether a specific PubKey has been banned, returns true if banned
func (db *ChainDb) ExistsFaultPubKey(sha *wire.Hash) (bool, error) {
	return db.faultPubKeyExists(sha)
}

// ExistsFaultPk - check whether a specific PubKey has been banned, returns true if banned
// Must be called with db lock held.
func (db *ChainDb) faultPubKeyExists(sha *wire.Hash) (bool, error) {
	key := shaFaultPubKeyToKey(sha)
	return db.localStorage.Has(key)
}
