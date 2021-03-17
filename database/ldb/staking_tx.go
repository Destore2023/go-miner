package ldb

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/Sukhavati-Labs/go-miner/consensus"
	"github.com/Sukhavati-Labs/go-miner/database"
	"github.com/Sukhavati-Labs/go-miner/database/storage"
	"github.com/Sukhavati-Labs/go-miner/logging"
	"github.com/Sukhavati-Labs/go-miner/wire"
)

var (
	// stakingTx
	recordStakingTx        = []byte("TXL")
	recordExpiredStakingTx = []byte("TXU")
	recordStakingAwarded   = []byte("TXR")
	recordStakingPoolTx    = []byte("TXP")
)

const (
	//
	// Each stakingTx/expiredStakingTx key is 55 bytes:
	// ----------------------------------------------------------------
	// |  Prefix | expiredHeight | blockHeight |   TxID   | TxOutIndex |
	// ----------------------------------------------------------------
	// | 3 bytes |    8 bytes   |   8 bytes  | 32 bytes |   4 bytes   |
	// ----------------------------------------------------------------
	stakingTxKeyLength = 55

	// ----------------------------------------------------------------
	// |  Prefix |     day      |        TxID                         |
	// ----------------------------------------------------------------
	// | 3 bytes |    8 bytes   |       32 bytes                      |
	// ----------------------------------------------------------------
	stakingAwardedSearchKeyLength = 43

	// Each stakingTx/expiredStakingTx value is 40 bytes:
	// -----------------------
	// | ScriptHash | Value  |
	// -----------------------
	// |  32 bytes  | 8 byte |
	// -----------------------

	stakingTxValueLength = 40

	// Each stakingTx/expiredStakingTx search key is 11 bytes:
	// ----------------------------
	// |   Prefix  | expiredHeight |
	// ----------------------------
	// |  3 bytes  |    8 byte    |
	// ----------------------------
	stakingTxSearchKeyLength = 11

	// Each stakingTx/expiredStakingTx value is 44 bytes:
	// ---------------------------------------
	// | blockHeight |   TxID   | TxOutIndex |
	// ---------------------------------------
	// |   8 bytes  | 32 bytes |    4 byte  |
	// ---------------------------------------
	stakingTxMapKeyLength = 44
)

// stakingTx
//
type stakingTx struct {
	txSha         *wire.Hash        //  bytes
	index         uint32            //  4 bytes
	expiredHeight uint64            //  8 bytes
	scriptHash    [sha256.Size]byte // 32 bytes
	value         uint64            //  8 bytes
	blockHeight   uint64            //  8 bytes
	delete        bool              //  1 bytes
}

//  Key:
//  +--------+------+-------+----------+
//  | height | type | txId  | index    |
//  +--------+------+-------+----------+
//  value:
//  +-------------+-------------+------------+
//  | scriptHash  | isCoinbase  | spent      |
//  +-------------+-------------+------------+
type stakingPoolTx struct {
	txSha      *wire.Hash        // txId
	index      uint32            // Tx Out Index
	scriptHash [sha256.Size]byte // 32 bytes
	value      uint64            // value
	isCoinbase bool              // is coinbase
	spent      bool              // spent
	delete     bool              // database tag
}

// mustDecodeStakingTxValue
// 0                      32              40
// +----------------------+---------------+
// | script hash 32 bytes | value 8 bytes |
// +----------------------+---------------+
func mustDecodeStakingTxValue(bs []byte) (scriptHash [sha256.Size]byte, value uint64) {
	if length := len(bs); length != stakingTxValueLength {
		logging.CPrint(logging.FATAL, "invalid raw StakingTxVale Data", logging.LogFormat{"length": length, "data": bs})
		panic("invalid raw StakingTxVale Data")
		// should not reach
	}
	copy(scriptHash[:], bs[:32])
	value = binary.LittleEndian.Uint64(bs[32:40])
	return
}

type stakingTxMapKey struct {
	blockHeight uint64    // 8  bytes
	txID        wire.Hash // 32 bytes
	index       uint32    // 4  bytes
}
type stakingAwardedRecordMapKey struct {
	day  uint64    // 8 bytes   day
	txID wire.Hash // 32 bytes  TxId
}

func mustDecodeStakingTxKey(buf []byte) (expiredHeight uint64, mapKey stakingTxMapKey) {
	if length := len(buf); length != stakingTxKeyLength {
		logging.CPrint(logging.FATAL, "invalid raw stakingTxKey Data", logging.LogFormat{"length": length, "data": buf})
		panic("invalid raw stakingTxKey Data") // should not reach
	}
	expiredHeight = binary.LittleEndian.Uint64(buf[3:11])
	height := binary.LittleEndian.Uint64(buf[11:19])
	var txid wire.Hash
	copy(txid[:], buf[19:51])
	index := binary.LittleEndian.Uint32(buf[51:])
	mapKey = stakingTxMapKey{
		blockHeight: height,
		txID:        txid,
		index:       index,
	}
	return
}

// mustDecodeStakingTxMapKey
// 0                 8                40
// +-----------------+----------------+---------------+
// | height  8 bytes | tx id 32 bytes | index 4 bytes |
// +-----------------+----------------+---------------+
func mustDecodeStakingTxMapKey(bs []byte) stakingTxMapKey {
	if length := len(bs); length != stakingTxMapKeyLength {
		logging.CPrint(logging.FATAL, "invalid raw stakingTxMapKey Data", logging.LogFormat{"length": length, "data": bs})
		panic("invalid raw stakingTxMapKey Data") // should not reach
	}
	height := binary.LittleEndian.Uint64(bs[:8])
	var txId wire.Hash
	copy(txId[:], bs[8:40])
	return stakingTxMapKey{
		blockHeight: height,
		txID:        txId,
		index:       binary.LittleEndian.Uint32(bs[40:]),
	}
}

// mustEncodeStakingTxMapKey
// 0                 8                40              44
// +-----------------+----------------+---------------+
// | height  8 bytes | tx id 32 bytes | index 4 bytes |
// +-----------------+----------------+---------------+
func mustEncodeStakingTxMapKey(mapKey stakingTxMapKey) []byte {
	var key [stakingTxMapKeyLength]byte
	binary.LittleEndian.PutUint64(key[:8], mapKey.blockHeight)
	copy(key[8:40], mapKey.txID[:])
	binary.LittleEndian.PutUint32(key[40:44], mapKey.index)
	return key[:]
}

// heightStakingTxToKey
//
// +--------+----------------+-----------------+----------------+---------------+
// | prefix | expired height | height  8 bytes | tx id 32 bytes | index 4 bytes |
// +--------+----------------+-----------------+----------------+---------------+
func heightStakingTxToKey(expiredHeight uint64, mapKey stakingTxMapKey) []byte {
	mapKeyData := mustEncodeStakingTxMapKey(mapKey)
	key := make([]byte, stakingTxKeyLength)

	copy(key[:len(recordStakingTx)], recordStakingTx)
	binary.LittleEndian.PutUint64(key[len(recordStakingTx):len(recordStakingTx)+8], expiredHeight)
	copy(key[len(recordStakingTx)+8:], mapKeyData)
	return key
}

// heightExpiredStakingTxToKey
// 0         3                        11
// +---------+------------------------+
// | prefix  | expired height 8 bytes |
// +---------+------------------------+
func heightExpiredStakingTxToKey(expiredHeight uint64, mapKey stakingTxMapKey) []byte {
	mapKeyData := mustEncodeStakingTxMapKey(mapKey)
	key := make([]byte, stakingTxKeyLength)

	copy(key[:len(recordExpiredStakingTx)], recordExpiredStakingTx)
	binary.LittleEndian.PutUint64(key[len(recordExpiredStakingTx):len(recordExpiredStakingTx)+8], expiredHeight)
	copy(key[len(recordExpiredStakingTx)+8:], mapKeyData)
	return key
}

// stakingAwardedRecordToKey
// 0         3                        11                         43
// +---------+------------------------+---------------------------+
// | prefix  | day    8 bytes         | tx id 32 bytes            |
// +---------+------------------------+---------------------------+
func stakingAwardedRecordToKey(mapKey stakingAwardedRecordMapKey) []byte {
	key := make([]byte, stakingAwardedSearchKeyLength)
	copy(key[:len(recordStakingAwarded)], recordStakingAwarded) //3 bytes
	binary.LittleEndian.PutUint64(key[len(recordStakingAwarded):len(recordStakingAwarded)+8], mapKey.day)
	copy(key[len(recordStakingAwarded)+8:stakingAwardedSearchKeyLength], mapKey.txID[:])
	return key
}

// stakingTxSearchKey
// 0                3                        11
// +----------------+------------------------+
// | prefix 3 bytes | expired height 8 bytes |
// +----------------+------------------------+
func stakingTxSearchKey(expiredHeight uint64) []byte {
	prefix := make([]byte, stakingTxSearchKeyLength)
	copy(prefix[:len(recordStakingTx)], recordStakingTx)
	binary.LittleEndian.PutUint64(prefix[len(recordStakingTx):stakingTxSearchKeyLength], expiredHeight)
	return prefix
}

// stakingAwardedSearchKey
// 0                3                        11
// +----------------+------------------------+
// | prefix 3 bytes |  day 8 bytes           |
// +----------------+------------------------+
func stakingAwardedSearchKey(queryTime uint64) []byte {
	day := queryTime / 86400
	prefix := make([]byte, stakingAwardedSearchKeyLength)
	copy(prefix[:len(recordStakingAwarded)], recordStakingAwarded)
	binary.LittleEndian.PutUint64(prefix[len(recordStakingAwarded):stakingAwardedSearchKeyLength], day)
	return prefix
}

// expiredStakingTxSearchKey
// 0                3                        11
// +----------------+------------------------+
// | prefix 3 bytes | expired height 8 bytes |
// +----------------+------------------------+
func expiredStakingTxSearchKey(expiredHeight uint64) []byte {
	prefix := make([]byte, stakingTxSearchKeyLength)
	copy(prefix[:len(recordExpiredStakingTx)], recordExpiredStakingTx)
	binary.LittleEndian.PutUint64(prefix[len(recordExpiredStakingTx):stakingTxSearchKeyLength], expiredHeight)
	return prefix
}

func (db *ChainDb) insertStakingTx(txSha *wire.Hash, index uint32, frozenPeriod uint64, blockHeight uint64, rsh [sha256.Size]byte, value int64) (err error) {
	var txL stakingTx
	var mapKey = stakingTxMapKey{
		blockHeight: blockHeight,
		txID:        *txSha,
		index:       index,
	}

	txL.txSha = txSha
	txL.index = index
	txL.expiredHeight = frozenPeriod + blockHeight
	txL.scriptHash = rsh
	txL.value = uint64(value)
	txL.blockHeight = blockHeight
	logging.CPrint(logging.DEBUG, "insertStakingTx in the height", logging.LogFormat{"startHeight": blockHeight, "expiredHeight": txL.expiredHeight})
	db.stakingTxMap[mapKey] = &txL

	return nil
}

// insertStakingPoolTx
//
func (db *ChainDb) insertStakingPoolTx(txSha *wire.Hash, index uint32, height uint64, rawScriptHash [sha256.Size]byte, value int64) error {
	return nil
}

func (db *ChainDb) FetchUnSpentStakingPoolTxOutByHeight(startHeight uint64, endHeight uint64) ([]*database.TxOutReply, error) {
	if endHeight < startHeight {
		return nil, fmt.Errorf("error height startHeight:%d > endHeight:%d ", startHeight, endHeight)
	}

	blockShaList, err := db.FetchHeightRange(startHeight, endHeight)
	if err != nil {
		return nil, err
	}
	for _, blockSha := range blockShaList {
		_, err := db.fetchBlockBySha(&blockSha)
		if err != nil {
			return nil, err
		}
		//unspentTxShaList = append(unspentTxShaList, block.Transactions()[0].Hash())
	}
	//unspentStakingPoolTxs := chain.db.FetchUnSpentTxByShaList(unspentTxShaList)
	replies := make([]*database.TxOutReply, 0)
	return replies, nil
}

func (db *ChainDb) formatSTx(stx *stakingTx) []byte {
	rsh := stx.scriptHash
	value := stx.value

	txW := make([]byte, 40)
	copy(txW[:32], rsh[:])
	binary.LittleEndian.PutUint64(txW[32:40], value)

	return txW
}

func (db *ChainDb) FetchExpiredStakingTxListByHeight(expiredHeight uint64) (database.StakingNodes, error) {

	nodes := make(database.StakingNodes)

	searchKey := expiredStakingTxSearchKey(expiredHeight)

	iter := db.localStorage.NewIterator(storage.BytesPrefix(searchKey))
	defer iter.Release()

	for iter.Next() {
		// key
		key := iter.Key()
		mapKey := mustDecodeStakingTxMapKey(key[11:])
		blockHeight := mapKey.blockHeight
		outPoint := wire.OutPoint{
			Hash:  mapKey.txID,
			Index: mapKey.index,
		}

		// data
		data := iter.Value()
		scriptHash, value := mustDecodeStakingTxValue(data)

		stakingInfo := database.StakingTxInfo{
			Value:        value,
			FrozenPeriod: expiredHeight - blockHeight,
			BlockHeight:  blockHeight,
		}

		if !nodes.Get(scriptHash).Put(outPoint, stakingInfo) {
			return nil, ErrCheckStakingDuplicated
		}
	}
	if err := iter.Error(); err != nil {
		return nil, err
	}

	return nodes, nil
}

func (db *ChainDb) FetchStakingTxMap() (database.StakingNodes, error) {

	nodes := make(database.StakingNodes)

	iter := db.localStorage.NewIterator(storage.BytesPrefix(recordStakingTx))
	defer iter.Release()

	for iter.Next() {

		// key
		key := iter.Key()
		expiredHeight, mapKey := mustDecodeStakingTxKey(key)
		blockHeight := mapKey.blockHeight
		outPoint := wire.OutPoint{
			Hash:  mapKey.txID,
			Index: mapKey.index,
		}

		// data
		data := iter.Value()
		scriptHash, value := mustDecodeStakingTxValue(data)

		stakingInfo := database.StakingTxInfo{
			Value:        value,
			FrozenPeriod: expiredHeight - blockHeight,
			BlockHeight:  blockHeight,
		}

		if !nodes.Get(scriptHash).Put(outPoint, stakingInfo) {
			return nil, ErrCheckStakingDuplicated
		}
	}

	if err := iter.Error(); err != nil {
		return nil, err
	}

	return nodes, nil
}

// fetchActiveStakingTxFromUnexpired returns currently unexpired staking at 'height'
func (db *ChainDb) fetchActiveStakingTxFromUnexpired(height uint64) (map[[sha256.Size]byte][]database.StakingTxInfo, error) {

	stakingTxInfos := make(map[[sha256.Size]byte][]database.StakingTxInfo)

	iter := db.localStorage.NewIterator(storage.BytesPrefix(recordStakingTx))
	defer iter.Release()
	recordStakingTxLen := len(recordStakingTx)
	for iter.Next() {
		key := iter.Key()
		expiredHeight := binary.LittleEndian.Uint64(key[recordStakingTxLen : recordStakingTxLen+8])
		mapKey := mustDecodeStakingTxMapKey(key[recordStakingTxLen+8:])
		blockHeight := mapKey.blockHeight

		value := iter.Value()
		scriptHash, amount := mustDecodeStakingTxValue(value)

		if height > blockHeight &&
			height <= expiredHeight &&
			height-blockHeight >= consensus.StakingTxRewardStart {
			stakingTxInfos[scriptHash] = append(stakingTxInfos[scriptHash], database.StakingTxInfo{
				Value:        amount,
				FrozenPeriod: expiredHeight - blockHeight,
				BlockHeight:  blockHeight})
		}
	}
	if err := iter.Error(); err != nil {
		return nil, err
	}

	return stakingTxInfos, nil
}

//
//fetchStakingAwardedRecordByTime
// +-----------+-----------+--------------+   +-----------+
// | prefix(3) |  day(8)   | TxId(32)     | ->| timestamp |
// +-----------+-----------+--------------+   +-----------+
func (db *ChainDb) FetchStakingAwardedRecordByTime(queryTime uint64) ([]database.StakingAwardedRecord, error) {
	stakingAwardedRecords := make([]database.StakingAwardedRecord, 0)
	iter := db.localStorage.NewIterator(storage.BytesPrefix(recordStakingAwarded))
	defer iter.Release()
	expectedDay := queryTime / 86400
	for iter.Next() {
		key := iter.Key()
		day := binary.LittleEndian.Uint64(key[len(recordStakingAwarded) : len(recordStakingAwarded)+8])
		if day != expectedDay {
			continue
		}
		value := iter.Value()
		timestamp := binary.LittleEndian.Uint64(value)
		record := database.StakingAwardedRecord{
			AwardedTime: timestamp,
		}
		copy(record.TxId[:], key[len(recordStakingAwarded)+8:stakingAwardedSearchKeyLength])
		stakingAwardedRecords = append(stakingAwardedRecords, record)
	}
	if err := iter.Error(); err != nil {
		return nil, err
	}
	return stakingAwardedRecords, nil
}

func (db *ChainDb) insertStakingAwardedRecord(txSha *wire.Hash, awardedTime uint64) (err error) {
	day := awardedTime / 86400
	var mapKey = stakingAwardedRecordMapKey{
		txID: *txSha,
		day:  day,
	}
	db.stakingAwardedRecordMap[mapKey] = &database.StakingAwardedRecord{
		TxId:        *txSha,
		AwardedTime: awardedTime,
	}
	logging.CPrint(logging.DEBUG, "insertStakingAwardedRecord in the time", logging.LogFormat{"timestamp": awardedTime, "txId": *txSha})
	return nil
}

// FetchUnexpiredStakingRank returns only currently unexpired staking rank at
// target height. This function is for mining and validating block.
func (db *ChainDb) FetchUnexpiredStakingRank(height uint64, onlyOnList bool) ([]database.Rank, error) {
	stakingTxInfos, err := db.fetchActiveStakingTxFromUnexpired(height)
	if err != nil {
		return nil, err
	}
	sortedStakingTx, err := database.SortMap(stakingTxInfos, height, onlyOnList)
	if err != nil {
		return nil, err
	}
	count := len(sortedStakingTx)
	rankList := make([]database.Rank, count)
	for i := 0; i < count; i++ {
		rankList[i].Rank = int32(i)
		rankList[i].Value = sortedStakingTx[i].Value
		rankList[i].ScriptHash = sortedStakingTx[i].Key
		rankList[i].Weight = sortedStakingTx[i].Weight
		rankList[i].StakingTx = stakingTxInfos[sortedStakingTx[i].Key]
	}
	return rankList, nil
}

func (db *ChainDb) FetchStakingStakingRewardInfo(height uint64) (*database.StakingRewardInfo, error) {
	info := new(database.StakingRewardInfo)
	return info, nil
}

// fetchActiveStakingTxFromExpired returns once unexpired staking at 'height'
func (db *ChainDb) fetchActiveStakingTxFromExpired(height uint64) (map[[sha256.Size]byte][]database.StakingTxInfo, error) {
	stakingTxInfos := make(map[[sha256.Size]byte][]database.StakingTxInfo)

	iter := db.localStorage.NewIterator(storage.BytesPrefix(recordExpiredStakingTx))
	defer iter.Release()

	for iter.Next() {
		key := iter.Key()
		expiredHeight := binary.LittleEndian.Uint64(key[len(recordExpiredStakingTx) : len(recordExpiredStakingTx)+8])
		mapKey := mustDecodeStakingTxMapKey(key[len(recordExpiredStakingTx)+8:])
		blockHeight := mapKey.blockHeight

		value := iter.Value()
		scriptHash, amount := mustDecodeStakingTxValue(value)

		if height > blockHeight &&
			height <= expiredHeight &&
			height-blockHeight >= consensus.StakingTxRewardStart {
			stakingTxInfos[scriptHash] = append(stakingTxInfos[scriptHash], database.StakingTxInfo{
				Value:        amount,
				FrozenPeriod: expiredHeight - blockHeight,
				BlockHeight:  blockHeight,
			})
		}
	}
	if err := iter.Error(); err != nil {
		return nil, err
	}

	return stakingTxInfos, nil
}

// FetchStakingRank returns staking rank at any height. This
// function may be slow.
func (db *ChainDb) FetchStakingRank(height uint64, onlyOnList bool) ([]database.Rank, error) {
	stakingTxInfos, err := db.fetchActiveStakingTxFromUnexpired(height)
	if err != nil {
		return nil, err
	}

	expired, err := db.fetchActiveStakingTxFromExpired(height)
	if err != nil {
		return nil, err
	}
	for expiredK, expiredV := range expired {

		if _, ok := stakingTxInfos[expiredK]; !ok {
			stakingTxInfos[expiredK] = expiredV
			continue
		}
		stakingTxInfos[expiredK] = append(stakingTxInfos[expiredK], expiredV...)
	}

	sortedStakingTx, err := database.SortMap(stakingTxInfos, height, onlyOnList)
	if err != nil {
		return nil, err
	}
	count := len(sortedStakingTx)
	rankList := make([]database.Rank, count)
	for i := 0; i < count; i++ {
		rankList[i].Rank = int32(i)
		rankList[i].Value = sortedStakingTx[i].Value
		rankList[i].ScriptHash = sortedStakingTx[i].Key
		rankList[i].Weight = sortedStakingTx[i].Weight
		rankList[i].StakingTx = stakingTxInfos[sortedStakingTx[i].Key]
	}
	return rankList, nil
}

func (db *ChainDb) expireStakingTx(currentHeight uint64) error {
	searchKey := stakingTxSearchKey(currentHeight)

	iter := db.localStorage.NewIterator(storage.BytesPrefix(searchKey))
	defer iter.Release()

	for iter.Next() {
		mapKeyBytes := iter.Key()[stakingTxSearchKeyLength:]
		value := iter.Value()
		mapKey := mustDecodeStakingTxMapKey(mapKeyBytes)

		scriptHash, val := mustDecodeStakingTxValue(value)

		// stakingTxMap
		db.stakingTxMap[mapKey] = &stakingTx{
			txSha:         &mapKey.txID,
			index:         mapKey.index,
			expiredHeight: currentHeight,
			scriptHash:    scriptHash,
			value:         val,
			blockHeight:   mapKey.blockHeight,
			delete:        true,
		}

		// expiredStakingTxMap
		db.expiredStakingTxMap[mapKey] = &stakingTx{
			txSha:         &mapKey.txID,
			index:         mapKey.index,
			expiredHeight: currentHeight,
			scriptHash:    scriptHash,
			value:         val,
			blockHeight:   mapKey.blockHeight,
		}

	}
	if err := iter.Error(); err != nil {
		return err
	}
	return nil
}

func (db *ChainDb) freezeStakingTx(currentHeight uint64) error {
	searchKey := expiredStakingTxSearchKey(currentHeight)

	iter := db.localStorage.NewIterator(storage.BytesPrefix(searchKey))
	defer iter.Release()

	for iter.Next() {
		mapKeyBytes := iter.Key()[stakingTxSearchKeyLength:]
		value := iter.Value()
		mapKey := mustDecodeStakingTxMapKey(mapKeyBytes)

		scriptHash, val := mustDecodeStakingTxValue(value)

		// expiredStakingTxMap
		db.expiredStakingTxMap[mapKey] = &stakingTx{
			txSha:         &mapKey.txID,
			index:         mapKey.index,
			expiredHeight: currentHeight,
			scriptHash:    scriptHash,
			value:         val,
			blockHeight:   mapKey.blockHeight,
			delete:        true,
		}

		// stakingTxMap
		db.stakingTxMap[mapKey] = &stakingTx{
			txSha:         &mapKey.txID,
			index:         mapKey.index,
			expiredHeight: currentHeight,
			scriptHash:    scriptHash,
			value:         val,
			blockHeight:   mapKey.blockHeight,
		}

	}

	if err := iter.Error(); err != nil {
		return err
	}
	return nil
}
