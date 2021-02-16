package ldb

import (
	"bytes"
	"encoding/binary"
	"sort"

	"github.com/Sukhavati-Labs/go-miner/config"
	"github.com/Sukhavati-Labs/go-miner/database"
	"github.com/Sukhavati-Labs/go-miner/database/storage"
	"golang.org/x/crypto/ripemd160"
)

var (
	bindingTxIndexPrefix            = []byte("STG")
	bindingShIndexPrefix            = []byte("HTGS")
	bindingTxSpentIndexPrefix       = []byte("STSG")
	bindingTxIndexKeyLen            = 3 + ripemd160.Size + 8 + 4 + 4 + 4
	bindingTxIndexSearchKeyLen      = 3 + ripemd160.Size
	bindingTxIndexDeleteKeyLen      = 3 + ripemd160.Size + 8
	bindingTxSpentIndexKeyLen       = 4 + 8 + 4 + 4 + ripemd160.Size + 8 + 4 + 4 + 4
	bindingTxSpentIndexSearchKeyLen = 4 + 8
	bindingShIndexKeyLen            = 4 + 8 + ripemd160.Size
	bindingShIndexSearchKeyLen      = 4 + 8
)

type bindingTxIndex struct {
	scriptHash  [ripemd160.Size]byte
	blockHeight uint64
	txOffset    uint32
	txLen       uint32
	index       uint32
}

type bindingTxSpentIndex struct {
	scriptHash         [ripemd160.Size]byte
	blockHeightSpent   uint64
	txOffsetSpent      uint32
	txLenSpent         uint32
	blockHeightBinding uint64
	txOffsetBinding    uint32
	txLenBinding       uint32
	indexBinding       uint32
}

type bindingShIndex struct {
	blockHeight uint64
	scriptHash  [ripemd160.Size]byte
}

func bindingTxIndexToKey(btxIndex *bindingTxIndex) []byte {
	key := make([]byte, bindingTxIndexKeyLen, bindingTxIndexKeyLen)
	copy(key, bindingTxIndexPrefix)
	copy(key[3:23], btxIndex.scriptHash[:])
	binary.LittleEndian.PutUint64(key[23:31], btxIndex.blockHeight)
	binary.LittleEndian.PutUint32(key[31:35], btxIndex.txOffset)
	binary.LittleEndian.PutUint32(key[35:39], btxIndex.txLen)
	binary.LittleEndian.PutUint32(key[39:43], btxIndex.index)
	return key
}

func mustDecodeBindingTxIndexKey(key []byte) (*bindingTxIndex, error) {
	if len(key) != bindingTxIndexKeyLen {
		return nil, ErrWrongBindingTxIndexLen
	}

	start := 0
	prefix := make([]byte, len(bindingTxIndexPrefix))
	copy(prefix, key[start:start+len(bindingTxIndexPrefix)])
	if !bytes.Equal(prefix, bindingTxIndexPrefix) {
		return nil, ErrWrongBindingTxIndexPrefix
	}
	start += len(bindingTxIndexPrefix)

	var scriptHash [ripemd160.Size]byte
	copy(scriptHash[:], key[start:start+ripemd160.Size])
	start += ripemd160.Size

	blockHeight := binary.LittleEndian.Uint64(key[start : start+8])
	start += 8
	txOffset := binary.LittleEndian.Uint32(key[start : start+4])
	start += 4
	txLen := binary.LittleEndian.Uint32(key[start : start+4])
	start += 4
	index := binary.LittleEndian.Uint32(key[start:bindingTxIndexKeyLen])

	return &bindingTxIndex{
		scriptHash:  scriptHash,
		blockHeight: blockHeight,
		txOffset:    txOffset,
		txLen:       txLen,
		index:       index,
	}, nil
}

func bindingTxIndexSearchKey(scriptHash [ripemd160.Size]byte) []byte {
	key := make([]byte, bindingTxIndexSearchKeyLen, bindingTxIndexSearchKeyLen)
	copy(key, bindingTxIndexPrefix)
	copy(key[3:23], scriptHash[:])
	return key
}

func bindingTxIndexDeleteKey(scriptHash [ripemd160.Size]byte, blockHeight uint64) []byte {
	key := make([]byte, bindingTxIndexDeleteKeyLen, bindingTxIndexDeleteKeyLen)
	copy(key, bindingTxIndexPrefix)
	copy(key[3:23], scriptHash[:])
	binary.LittleEndian.PutUint64(key[23:31], blockHeight)
	return key
}

func bindingTxSpentIndexToKey(btxSpentIndex *bindingTxSpentIndex) []byte {
	key := make([]byte, bindingTxSpentIndexKeyLen, bindingTxSpentIndexKeyLen)
	copy(key, bindingTxSpentIndexPrefix)
	binary.LittleEndian.PutUint64(key[4:12], btxSpentIndex.blockHeightSpent)
	binary.LittleEndian.PutUint32(key[12:16], btxSpentIndex.txOffsetSpent)
	binary.LittleEndian.PutUint32(key[16:20], btxSpentIndex.txLenSpent)
	copy(key[20:40], btxSpentIndex.scriptHash[:])
	binary.LittleEndian.PutUint64(key[40:48], btxSpentIndex.blockHeightBinding)
	binary.LittleEndian.PutUint32(key[48:52], btxSpentIndex.txOffsetBinding)
	binary.LittleEndian.PutUint32(key[52:56], btxSpentIndex.txLenBinding)
	binary.LittleEndian.PutUint32(key[56:60], btxSpentIndex.indexBinding)
	return key
}

func mustDecodeBindingTxSpentIndexKey(key []byte) (*bindingTxSpentIndex, error) {
	if len(key) != bindingTxSpentIndexKeyLen {
		return nil, ErrWrongBindingTxSpentIndexLen
	}

	start := 0
	prefix := make([]byte, len(bindingTxSpentIndexPrefix))
	copy(prefix, key[start:start+len(bindingTxSpentIndexPrefix)])
	if !bytes.Equal(prefix, bindingTxSpentIndexPrefix) {
		return nil, ErrWrongBindingTxSpentIndexPrefix
	}
	start += len(bindingTxSpentIndexPrefix)

	stxBlockHeight := binary.LittleEndian.Uint64(key[start : start+8])
	start += 8
	stxTxOffset := binary.LittleEndian.Uint32(key[start : start+4])
	start += 4
	stxTxLen := binary.LittleEndian.Uint32(key[start : start+4])
	start += 4

	var scriptHash [ripemd160.Size]byte
	copy(scriptHash[:], key[start:start+ripemd160.Size])
	start += ripemd160.Size
	btxBlockHeight := binary.LittleEndian.Uint64(key[start : start+8])
	start += 8
	btxTxOffset := binary.LittleEndian.Uint32(key[start : start+4])
	start += 4
	btxTxLen := binary.LittleEndian.Uint32(key[start : start+4])
	start += 4
	btxIndex := binary.LittleEndian.Uint32(key[start:bindingTxSpentIndexKeyLen])

	return &bindingTxSpentIndex{
		scriptHash:         scriptHash,
		blockHeightSpent:   stxBlockHeight,
		txOffsetSpent:      stxTxOffset,
		txLenSpent:         stxTxLen,
		blockHeightBinding: btxBlockHeight,
		txOffsetBinding:    btxTxOffset,
		txLenBinding:       btxTxLen,
		indexBinding:       btxIndex,
	}, nil
}

func bindingTxSpentIndexSearchKey(blockHeight uint64) []byte {
	key := make([]byte, bindingTxSpentIndexSearchKeyLen, bindingTxSpentIndexSearchKeyLen)
	copy(key, bindingTxSpentIndexPrefix)
	binary.LittleEndian.PutUint64(key[4:12], blockHeight)
	return key
}

func bindingShIndexToKey(bshIndex *bindingShIndex) []byte {
	key := make([]byte, bindingShIndexKeyLen)
	copy(key, bindingShIndexPrefix)
	binary.LittleEndian.PutUint64(key[4:12], bshIndex.blockHeight)
	copy(key[12:32], bshIndex.scriptHash[:])
	return key
}

func mustDecodeBindingShIndexKey(key []byte) (*bindingShIndex, error) {
	if len(key) != bindingShIndexKeyLen {
		return nil, ErrWrongBindingShIndexLen
	}

	start := 0
	prefix := make([]byte, len(bindingShIndexPrefix))
	copy(prefix, key[start:start+len(bindingShIndexPrefix)])
	if !bytes.Equal(prefix, bindingShIndexPrefix) {
		return nil, ErrWrongBindingShIndexPrefix
	}
	start += len(bindingShIndexPrefix)

	blockHeight := binary.LittleEndian.Uint64(key[start : start+8])
	start += 8

	var scriptHash [ripemd160.Size]byte
	copy(scriptHash[:], key[start:bindingShIndexKeyLen])

	return &bindingShIndex{
		blockHeight: blockHeight,
		scriptHash:  scriptHash,
	}, nil
}

func bindingShIndexSearchKey(blockHeight uint64) []byte {
	key := make([]byte, bindingShIndexSearchKeyLen, bindingShIndexSearchKeyLen)
	copy(key, bindingShIndexPrefix)
	binary.LittleEndian.PutUint64(key[4:12], blockHeight)
	return key
}

func (db *ChainDb) FetchScriptHashRelatedBindingTx(scriptHash []byte, chainParams *config.Params) ([]*database.BindingTxReply, error) {
	bindingTxs := make([]*database.BindingTxReply, 0)
	var sh [ripemd160.Size]byte
	if len(scriptHash) != ripemd160.Size {
		return nil, ErrWrongScriptHashLength
	}
	copy(sh[:], scriptHash)
	btxSearchKeyPrefix := bindingTxIndexSearchKey(sh)
	iter := db.localStorage.NewIterator(storage.BytesPrefix(btxSearchKeyPrefix))
	defer iter.Release()
	for iter.Next() {
		key := iter.Key()
		blockHeight := binary.LittleEndian.Uint64(key[23:31])
		txOffset := binary.LittleEndian.Uint32(key[31:35])
		txLen := binary.LittleEndian.Uint32(key[35:39])
		index := binary.LittleEndian.Uint32(key[39:43])
		msgTx, _, err := db.fetchTxDataByLoc(blockHeight, int(txOffset), int(txLen))
		if err != nil {
			return nil, err
		}
		txSha := msgTx.TxHash()
		bindingTxs = append(bindingTxs, &database.BindingTxReply{
			Height: blockHeight,
			TxSha:  &txSha,
			Index:  index,
			Value:  msgTx.TxOut[index].Value,
		})
	}
	if err := iter.Error(); err != nil {
		return nil, err
	}

	sort.Stable(sort.Reverse(bindingTxList(bindingTxs)))

	return bindingTxs, nil
}

type bindingTxList []*database.BindingTxReply

func (gl bindingTxList) Swap(i, j int) {
	gl[i], gl[j] = gl[j], gl[i]
}

func (gl bindingTxList) Len() int {
	return len(gl)
}

func (gl bindingTxList) Less(i, j int) bool {
	return gl[i].Value < gl[j].Value
}
