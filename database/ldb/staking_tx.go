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
	recordStakingTx            = []byte("TXL")
	recordExpiredStakingTx     = []byte("TXU")
	recordStakingAwardedRecord = []byte("TXR")
	recordStakingPoolTx        = []byte("TXP")
)

const (
	//
	// Each stakingTx/expiredStakingTx key is 55 bytes:
	// 0         3                 11            19             27         59            63
	// +---------+------------------+-------------+-------------+----------+--------------+
	// |  Prefix | expiredTimestamp |  timestamp  | blockHeight |   TxID   | TxOutIndex   |
	// |---------+------------------+-------------+-------------+----------+--------------+
	// | 3 bytes |    8 bytes       |   8 bytes   | 8 bytes     | 32 bytes |   4 bytes    |
	// +---------+------------------+-------------+-------------+----------+--------------+
	stakingTxKeyLength = 63

	// 0        3               11            19
	// +---------+--------------+--------------+
	// |  Prefix | period  day  |  timestamp   |
	// |---------+-----------------------------+
	// | 3 bytes |    8 bytes   |  8 bytes     |
	// +---------+--------------+--------------+
	stakingAwardedSearchKeyLength = 19

	// Each stakingTx/expiredStakingTx value is 40 bytes:
	// 0            32          40
	// +------------+-----------+
	// | ScriptHash | Value     |
	// |------------+-----------|
	// |  32 bytes  | 8 byte    |
	// +------------+-----------+

	stakingTxValueLength = 40

	// Each stakingTx/expiredStakingTx search key is 11 bytes:
	// 0           3                   11
	// +-----------+-------------------+
	// |   Prefix  | expiredTimestamp  |
	// |-----------+-------------------+
	// |  3 bytes  |    8 byte         |
	// +-----------+-------------------+
	stakingTxSearchKeyLength = 11

	// Each stakingTx/expiredStakingTx value is 44 bytes:
	// 0             8          40               44
	// +-------------+----------+----------------+
	// | blockHeight |   TxID   | TxOutIndex     |
	// |-------------+----------+----------------|
	// |   8 bytes   | 32 bytes |    4 byte      |
	// +-------------+----------+----------------+
	stakingTxMapKeyLength = 44
)

// stakingTx
//
type stakingTx struct {
	txID             *wire.Hash        //  bytes
	index            uint32            //  4 bytes
	expiredTimestamp uint64            //  8 bytes expired timestamp
	timestamp        uint64            //  8 bytes staking timestamp
	scriptHash       [sha256.Size]byte // 32 bytes
	value            uint64            //  8 bytes
	blockHeight      uint64            //  8 bytes
	delete           bool              //  1 bytes
}

type stakingAwardedRecordMapKey struct {
	period    uint64 // 8 bytes   day
	timestamp uint64 // award timestamp
}

// Staking Awarded Record
// +----------+------------+     +--------------+
// |  prefix  | timestamp  | --> | tx sha       |
// +----------+------------+     +--------------+
type stakingAwardedRecord struct {
	txID      *wire.Hash // award tx sha
	timestamp uint64     // award timestamp
	delete    bool       // delete from db when this is true
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
	txID       *wire.Hash        // txId
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
	blockHeight uint64    // block height 8  bytes
	txID        wire.Hash // tx id        32 bytes
	index       uint32    // staking tx index  4  bytes
	timestamp   uint64    // staking timestamp 8 bytes
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

// StakingTxToKey
// 0        3                  11                 19
// +--------+-------------------+------------------+-----------------+----------------+---------------+
// | prefix | expired timestamp | timestamp 8 bytes| height  8 bytes | tx id 32 bytes | index 4 bytes |
// +--------+-------------------+------------------+-----------------+----------------+---------------+
func StakingTxToKey(expiredTimestamp uint64, mapKey stakingTxMapKey) []byte {
	mapKeyData := mustEncodeStakingTxMapKey(mapKey)
	key := make([]byte, stakingTxKeyLength)
	recordStakingTxPrefixLen := len(recordStakingTx)
	copy(key[:recordStakingTxPrefixLen], recordStakingTx)
	binary.LittleEndian.PutUint64(key[recordStakingTxPrefixLen:recordStakingTxPrefixLen+8], expiredTimestamp)
	binary.LittleEndian.PutUint64(key[recordStakingTxPrefixLen+8:recordStakingTxPrefixLen+16], mapKey.timestamp)
	copy(key[recordStakingTxPrefixLen+16:], mapKeyData)
	return key
}

// expiredStakingTxToKey
// 0         3                          11
// +---------+---------------------------+
// | prefix  | expired timestamp 8 bytes |
// +---------+---------------------------+
func expiredStakingTxToKey(expiredHeight uint64, mapKey stakingTxMapKey) []byte {
	mapKeyData := mustEncodeStakingTxMapKey(mapKey)
	key := make([]byte, stakingTxKeyLength)

	copy(key[:len(recordExpiredStakingTx)], recordExpiredStakingTx)
	binary.LittleEndian.PutUint64(key[len(recordExpiredStakingTx):len(recordExpiredStakingTx)+8], expiredHeight)
	copy(key[len(recordExpiredStakingTx)+8:], mapKeyData)
	return key
}

// stakingAwardedRecordToKey
// 0         3                        11                         19
// +---------+------------------------+---------------------------+
// | prefix  | period   8 bytes       | timestamp 8 bytes         |
// +---------+------------------------+---------------------------+
func stakingAwardedRecordToKey(mapKey stakingAwardedRecordMapKey) []byte {
	key := make([]byte, stakingAwardedSearchKeyLength)
	recordStakingAwardedRecordLen := len(recordStakingAwardedRecord)
	copy(key[:recordStakingAwardedRecordLen], recordStakingAwardedRecord) //3 bytes
	binary.LittleEndian.PutUint64(key[recordStakingAwardedRecordLen:recordStakingAwardedRecordLen+8], mapKey.period)
	binary.LittleEndian.PutUint64(key[recordStakingAwardedRecordLen+8:stakingAwardedSearchKeyLength], mapKey.timestamp)
	return key
}

// stakingTxSearchKey
// 0                3                        11
// +----------------+------------------------+
// | prefix 3 bytes | expired height 8 bytes |
// +----------------+------------------------+
func stakingTxSearchKey(expiredTimestamp uint64) []byte {
	prefix := make([]byte, stakingTxSearchKeyLength)
	recordStakingTxPrefixLen := len(recordStakingTx)
	copy(prefix[:recordStakingTxPrefixLen], recordStakingTx)
	binary.LittleEndian.PutUint64(prefix[recordStakingTxPrefixLen:stakingTxSearchKeyLength], expiredTimestamp)
	return prefix
}

// stakingAwardedSearchKey
// 0                3                        11                       19
// +----------------+------------------------+------------------------+
// | prefix 3 bytes | expired height 8 bytes | timestamp 8 bytes      |
// +----------------+------------------------+------------------------+
func stakingAwardedRecordSearchKey(queryTime uint64) []byte {
	period := queryTime / 86400
	prefix := make([]byte, stakingAwardedSearchKeyLength)
	recordStakingAwardedLen := len(recordStakingAwardedRecord)
	copy(prefix[:recordStakingAwardedLen], recordStakingAwardedRecord)
	binary.LittleEndian.PutUint64(prefix[recordStakingAwardedLen:stakingAwardedSearchKeyLength], period)
	return prefix
}

// expiredStakingTxSearchKey
// 0                3                          11
// +----------------+---------------------------+
// | prefix 3 bytes | expired timestamp 8 bytes |
// +----------------+---------------------------+
func expiredStakingTxSearchKey(expiredTimestamp uint64) []byte {
	prefix := make([]byte, stakingTxSearchKeyLength)
	recordExpiredStakingTxPrefixLen := len(recordExpiredStakingTx)
	copy(prefix[:recordExpiredStakingTxPrefixLen], recordExpiredStakingTx)
	binary.LittleEndian.PutUint64(prefix[recordExpiredStakingTxPrefixLen:stakingTxSearchKeyLength], expiredTimestamp)
	return prefix
}

// insertStakingTx
// txID --> staking tx id
func (db *ChainDb) insertStakingTx(txID *wire.Hash, index uint32, frozenPeriod uint64, timestamp, blockHeight uint64, rsh [sha256.Size]byte, value int64) (err error) {
	var txL stakingTx
	var mapKey = stakingTxMapKey{
		blockHeight: blockHeight,
		txID:        *txID,
		index:       index,
	}
	txL.txID = txID
	txL.index = index
	txL.timestamp = timestamp
	txL.expiredTimestamp = frozenPeriod + timestamp
	txL.scriptHash = rsh
	txL.value = uint64(value)
	txL.blockHeight = blockHeight
	logging.CPrint(logging.DEBUG, "insertStakingTx",
		logging.LogFormat{
			"startHeight":      blockHeight,
			"stakingTimestamp": timestamp,
			"expiredTimestamp": txL.expiredTimestamp,
			"txID":             txID,
			"value":            value,
		})
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

// formatSTx format staking tx bytes
func (db *ChainDb) formatSTx(stx *stakingTx) []byte {
	rsh := stx.scriptHash
	value := stx.value
	txW := make([]byte, 40)
	copy(txW[:32], rsh[:])
	binary.LittleEndian.PutUint64(txW[32:40], value)
	return txW
}

// formatSTxRecord format staking award record to bytes
func (db *ChainDb) formatSTxAwardedRecord(record *stakingAwardedRecord) []byte {
	buf := make([]byte, 32)
	copy(buf, (*record.txID)[:])
	return buf
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
			height-blockHeight >= consensus.StakingTxRewardStartHeight {
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
// 0           3           11                  19   0                  32
// +-----------+-----------+--------------------+   +------------------+
// | prefix(3) |  day(8)   | timestamp 8 bytes  | ->|  txID  32 bytes  |
// +-----------+-----------+--------------------+   +------------------+
// timestamp seconds
func (db *ChainDb) FetchStakingAwardedRecordByTimestamp(timestamp uint64) ([]database.StakingAwardedRecord, error) {
	stakingAwardedRecords := make([]database.StakingAwardedRecord, 0)
	iter := db.localStorage.NewIterator(storage.BytesPrefix(recordStakingAwardedRecord))
	defer iter.Release()
	queryPeriod := timestamp / consensus.DayPeriod
	recodeLen := len(recordStakingAwardedRecord)
	for iter.Next() {
		key := iter.Key()
		period := binary.LittleEndian.Uint64(key[recodeLen : recodeLen+8])
		if period != queryPeriod {
			continue
		}

		stakingTimestamp := binary.LittleEndian.Uint64(key[recodeLen+8 : 19])
		record := database.StakingAwardedRecord{
			Timestamp: stakingTimestamp,
		}
		value := iter.Value()
		copy(record.TxID[:], value)
		stakingAwardedRecords = append(stakingAwardedRecords, record)
	}
	if err := iter.Error(); err != nil {
		return nil, err
	}
	return stakingAwardedRecords, nil
}

// insertStakingAwardedRecord
//  timestamp : the time of the award
func (db *ChainDb) insertStakingAwardedRecord(txSha *wire.Hash, timestamp uint64) (err error) {
	period := timestamp / consensus.DayPeriod
	var mapKey = stakingAwardedRecordMapKey{
		period:    period,
		timestamp: timestamp,
	}
	db.stakingAwardedRecordMap[mapKey] = &stakingAwardedRecord{
		txID:      txSha,
		timestamp: timestamp,
		delete:    false,
	}
	logging.CPrint(logging.DEBUG, "insertStakingAwardedRecord in the time",
		logging.LogFormat{
			"txSha":     *txSha,
			"period":    period,
			"timestamp": timestamp,
		})
	return nil
}

// FetchUnexpiredStakingRank returns only currently unexpired staking rank at
// target height. This function is for mining and validating block.
func (db *ChainDb) FetchUnexpiredStakingRank(timestamp, height uint64, onlyOnList bool) ([]database.Rank, error) {
	stakingTxInfos, err := db.fetchActiveStakingTxFromUnexpired(height)
	if err != nil {
		return nil, err
	}
	sortedStakingTx, err := database.SortMap(stakingTxInfos, timestamp, height, onlyOnList)
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

// fetchActiveStakingTxFromExpired returns once unexpired staking at 'height'
func (db *ChainDb) fetchActiveStakingTxFromExpired(timestamp, height uint64) (map[[sha256.Size]byte][]database.StakingTxInfo, error) {
	stakingTxInfos := make(map[[sha256.Size]byte][]database.StakingTxInfo)

	iter := db.localStorage.NewIterator(storage.BytesPrefix(recordExpiredStakingTx))
	defer iter.Release()
	recordPrefixLen := len(recordExpiredStakingTx)
	for iter.Next() {
		key := iter.Key()
		expiredTimestamp := binary.LittleEndian.Uint64(key[recordPrefixLen : recordPrefixLen+8])
		stakingTimestamp := binary.LittleEndian.Uint64(key[recordPrefixLen+8 : recordPrefixLen+16])
		frozenPeriod := expiredTimestamp - stakingTimestamp
		mapKey := mustDecodeStakingTxMapKey(key[recordPrefixLen+16:])
		blockHeight := mapKey.blockHeight

		value := iter.Value()
		scriptHash, amount := mustDecodeStakingTxValue(value)

		if height > blockHeight &&
			timestamp <= expiredTimestamp &&
			height-blockHeight >= consensus.StakingTxRewardStartHeight && (frozenPeriod+consensus.StakingTxRewardDeviationTime) < timestamp {
			stakingTxInfos[scriptHash] = append(stakingTxInfos[scriptHash], database.StakingTxInfo{
				Value:        amount,
				FrozenPeriod: frozenPeriod,
				BlockHeight:  blockHeight,
				Timestamp:    stakingTimestamp,
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
func (db *ChainDb) FetchStakingRank(timestamp, height uint64, onlyOnList bool) ([]database.Rank, error) {
	stakingTxInfos, err := db.fetchActiveStakingTxFromUnexpired(height)
	if err != nil {
		return nil, err
	}

	expired, err := db.fetchActiveStakingTxFromExpired(timestamp, height)
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

	sortedStakingTx, err := database.SortMap(stakingTxInfos, timestamp, height, onlyOnList)
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

func (db *ChainDb) expireStakingTx(timestamp, currentHeight uint64) error {
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
			txID:             &mapKey.txID,
			index:            mapKey.index,
			timestamp:        timestamp,
			expiredTimestamp: currentHeight,
			scriptHash:       scriptHash,
			value:            val,
			blockHeight:      mapKey.blockHeight,
			delete:           true,
		}

		// expiredStakingTxMap
		db.expiredStakingTxMap[mapKey] = &stakingTx{
			txID:             &mapKey.txID,
			index:            mapKey.index,
			expiredTimestamp: currentHeight,
			scriptHash:       scriptHash,
			value:            val,
			blockHeight:      mapKey.blockHeight,
		}

	}
	if err := iter.Error(); err != nil {
		return err
	}
	return nil
}

// freezeStakingTx
func (db *ChainDb) freezeStakingTx(currentTimestamp uint64) error {
	searchKey := expiredStakingTxSearchKey(currentTimestamp)

	iter := db.localStorage.NewIterator(storage.BytesPrefix(searchKey))
	defer iter.Release()

	for iter.Next() {
		mapKeyBytes := iter.Key()[stakingTxSearchKeyLength:]
		value := iter.Value()
		mapKey := mustDecodeStakingTxMapKey(mapKeyBytes)

		scriptHash, val := mustDecodeStakingTxValue(value)

		// expiredStakingTxMap
		db.expiredStakingTxMap[mapKey] = &stakingTx{
			txID:             &mapKey.txID,
			index:            mapKey.index,
			expiredTimestamp: currentTimestamp,
			timestamp:        currentTimestamp,
			scriptHash:       scriptHash,
			value:            val,
			blockHeight:      mapKey.blockHeight,
			delete:           true,
		}

		// stakingTxMap
		db.stakingTxMap[mapKey] = &stakingTx{
			txID:             &mapKey.txID,
			index:            mapKey.index,
			expiredTimestamp: currentTimestamp,
			scriptHash:       scriptHash,
			value:            val,
			blockHeight:      mapKey.blockHeight,
		}

	}

	if err := iter.Error(); err != nil {
		return err
	}
	return nil
}
