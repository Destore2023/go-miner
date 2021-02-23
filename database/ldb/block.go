//  LSN  (log sequence number)
package ldb

import (
	"bytes"
	"encoding/binary"
	"math"

	"github.com/Sukhavati-Labs/go-miner/blockchain"

	"github.com/Sukhavati-Labs/go-miner/database/storage"
	"github.com/Sukhavati-Labs/go-miner/debug"
	"github.com/Sukhavati-Labs/go-miner/errors"
	"github.com/Sukhavati-Labs/go-miner/logging"
	"github.com/Sukhavati-Labs/go-miner/txscript"
	wirepb "github.com/Sukhavati-Labs/go-miner/wire/pb"

	"github.com/golang/protobuf/proto"

	"github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/database"
	"github.com/Sukhavati-Labs/go-miner/wire"
)

var (
	// +------------+----------------+      +---------------+------------+----------+------------+
	// |  "BLKHGT"  |  block height  |      |   block hash  |   file no  |  offset  | block size |
	// |   6-bytes  |     8-bytes    |  ->  |    32-bytes   |   4-bytes  |  8-bytes |   8-bytes  |
	// +------------+----------------+      +---------------+------------+----------+------------+
	blockHeightKeyPrefix = []byte("BLKHGT")

	// +------------+--------------+      +----------------+
	// |  "BLKSHA"  |  block hash  |      |  block height  |
	// |   6-bytes  |   32-bytes   |  ->  |    8-bytes     |
	// +------------+--------------+      +----------------+
	blockShaKeyPrefix = []byte("BLKSHA")

	//
)

const (
	blockHeightKeyPrefixLength = 6
	blockHeightKeyLength       = blockHeightKeyPrefixLength + 8
	blockShaKeyPrefixLength    = 6
	blockShaKeyLength          = blockShaKeyPrefixLength + 32
	UnknownHeight              = math.MaxUint64
)

//makeBlockHeightKey
// +--------------------+------------------------+
// | BLKHGT (6 bytes )  |  height   (8 bytes)    |
// +--------------------+------------------------+
func makeBlockHeightKey(height uint64) []byte {
	var bs [blockHeightKeyLength]byte
	copy(bs[:], blockHeightKeyPrefix)
	binary.LittleEndian.PutUint64(bs[blockHeightKeyPrefixLength:], height)
	return bs[:]
}

// makeBlockShaKey
// +-------------------+----------------------+
// | BLKSHA (6 bytes)  | blockSha  (32 bytes) |
// +-------------------+----------------------+
func makeBlockShaKey(blockSha *wire.Hash) []byte {
	var bs [blockShaKeyLength]byte
	copy(bs[:], blockShaKeyPrefix)
	copy(bs[blockShaKeyPrefixLength:], (*blockSha)[:])
	return bs[:]
}

func (db *ChainDb) SubmitBlock(block *chainutil.Block, txReplyStore database.TxReplyStore) error {
	db.dbLock.Lock()
	defer db.dbLock.Unlock()

	batch := db.Batch(blockBatch)
	batch.Set(*block.Hash())

	if err := db.preSubmit(blockBatch); err != nil {
		return err
	}

	if err := db.submitBlock(block, txReplyStore); err != nil {
		batch.Reset()
		return err
	}

	batch.Done()
	return nil
}

func (db *ChainDb) submitBlock(block *chainutil.Block, inputTxStore database.TxReplyStore) (err error) {

	defer func() {
		if err == nil {
			err = db.processBlockBatch()
		}
	}()

	batch := db.Batch(blockBatch).Batch()
	blockHash := block.Hash()

	rawMsg, err := block.MsgBlock().Bytes(wire.DB)
	if err != nil {
		logging.CPrint(logging.ERROR, "failed to serialize block", logging.LogFormat{"block": blockHash, "err": err})
		return err
	}
	block.SetSerializedBlockDB(rawMsg)
	txLocations, err := block.TxLoc()
	if err != nil {
		logging.CPrint(logging.ERROR, "failed to get txloc", logging.LogFormat{"block": blockHash, "err": err})
		return err
	}
	// save raw block to disk
	blockFile, offset, err := db.blockFileKeeper.SaveRawBlockToDisk(rawMsg, block.Height(), block.MsgBlock().Header.Timestamp.Unix())
	if err != nil {
		logging.CPrint(logging.ERROR, "save block to disk failed", logging.LogFormat{"block": blockHash, "err": err})
		return err
	}
	err = putRawBlockIndex(batch, block, blockFile, offset, int64(len(rawMsg)))
	if err != nil {
		logging.CPrint(logging.ERROR, "save block file index failed", logging.LogFormat{"block": blockHash, "err": err})
		return err
	}

	// update "META"
	newStorageMeta := dbStorageMeta{
		currentHeight: block.Height(),
		currentHash:   *block.Hash(),
	}
	batch.Put(dbStorageMetaDataKey, encodeDBStorageMetaData(newStorageMeta))

	// update punishments
	if err = insertBlockPunishments(batch, block); err != nil {
		return err
	}

	// put pk, bitLength and height into batch
	if err = db.insertPubKeyBLHeightToBatch(batch, block.MsgBlock().Header.PubKey, block.MsgBlock().Header.Proof.BitLength, block.Height()); err != nil {
		return err
	}

	// put index for pubKey->Block
	if err = updateMinedBlockIndex(batch, true, block.MsgBlock().Header.PubKey, block.Height()); err != nil {
		return err
	}

	// staking award tx record
	transactions := block.Transactions()
	coinbaseTx := transactions[0]
	coinbasePayload := blockchain.NewCoinbasePayload()
	err = coinbasePayload.SetBytes(coinbaseTx.MsgTx().Payload)
	if err != nil {
		return err
	}
	if coinbasePayload.LastStakingAwardedTimestamp() > 0 {
		db.insertStakingAwardedRecord(coinbaseTx.Hash(), coinbasePayload.LastStakingAwardedTimestamp())
	}

	// At least two blocks in the long past were generated by faulty
	// miners, the sha of the transaction exists in a previous block,
	// detect this condition and 'accept' the block.
	for txIdx, tx := range block.Transactions() {
		spentBufLen := (len(tx.TxOut()) + 7) / 8
		spentBuf := make([]byte, spentBufLen)
		if block.Height() == 0 {
			for _, b := range spentBuf {
				for i := uint(0); i < 8; i++ {
					b |= byte(1) << i
				}
			}
		} else {
			if len(tx.TxOut())%8 != 0 {
				for i := uint(len(tx.TxOut()) % 8); i < 8; i++ {
					spentBuf[spentBufLen-1] |= byte(1) << i
				}
			}
		}
		isGovernance := false
		// tx in
		for _, txIn := range tx.TxIn() {
			txReply, ok := inputTxStore[txIn.PreviousOutPoint.Hash]
			if ok {
				pubKeyClass := txscript.GetScriptClass(txReply.Tx.TxOut[txIn.PreviousOutPoint.Index].PkScript)
				if pubKeyClass == txscript.GovernanceScriptHashTy {
					isGovernance = true
					break
				}
			}
		}
		// find and insert staking tx
		for i, txOut := range tx.TxOut() {
			pubKeyInfo := tx.GetPkScriptInfo(i)
			switch pubKeyInfo.Class {
			case byte(txscript.StakingScriptHashTy):
				{
					logging.CPrint(logging.DEBUG, "Insert StakingTx", logging.LogFormat{
						"txid":   tx.Hash(),
						"vout":   i,
						"height": block.Height(),
						"frozen": pubKeyInfo.FrozenPeriod,
					})
					err = db.insertStakingTx(tx.Hash(), uint32(i), pubKeyInfo.FrozenPeriod, block.Height(), pubKeyInfo.ScriptHash, txOut.Value)
					if err != nil {
						return err
					}
				}
			case byte(txscript.GovernanceScriptHashTy):
				{
					// only tx vin is governance
					logging.CPrint(logging.DEBUG, "Insert GovernanceTx", logging.LogFormat{
						"txid":   tx.Hash(),
						"vout":   i,
						"height": block.Height(),
					})
					if isGovernance {
						config, err := database.DecodeGovernanceConfig(tx.MsgTx().Payload)
						if err == nil {
							db.insertGovernanceConfig(0, &config, true)
						}
					}
				}
			}
		}

		err = db.insertTx(tx.Hash(), block.Height(), txLocations[txIdx].TxStart, txLocations[txIdx].TxLen, spentBuf)
		if err != nil {
			logging.CPrint(logging.WARN, "failed to insert tx",
				logging.LogFormat{"block": blockHash, "height": block.Height(), "tx": tx.Hash(), "err": err})
			return err
		}

		err = db.doSpend(tx.MsgTx())
		if err != nil {
			logging.CPrint(logging.WARN, "failed to spend tx",
				logging.LogFormat{"block": blockHash, "height": block.Height(), "tx": tx.Hash(), "err": err})
			return err
		}

		err = db.expireStakingTx(block.Height())
		if err != nil {
			logging.CPrint(logging.ERROR, "block failed to expire tx", logging.LogFormat{
				"block":  blockHash,
				"height": block.Height(),
				"tx":     tx.Hash(),
			})
			return err
		}
	}
	return nil
}

func (db *ChainDb) DeleteBlock(blockSha *wire.Hash) error {
	db.dbLock.Lock()
	defer db.dbLock.Unlock()

	batch := db.Batch(blockBatch)
	batch.Set(*blockSha)

	if err := db.preSubmit(blockBatch); err != nil {
		return err
	}

	if err := db.deleteBlock(blockSha); err != nil {
		batch.Reset()
		return err
	}

	batch.Done()
	return nil
}

func (db *ChainDb) deleteBlock(blockSha *wire.Hash) (err error) {
	defer func() {
		if err == nil {
			err = db.processBlockBatch()
		}
	}()

	// can't remove current block
	if !blockSha.IsEqual(&db.dbStorageMeta.currentHash) {
		logging.CPrint(logging.ERROR, "fail on delete block",
			logging.LogFormat{
				"err":         err,
				"currentHash": db.dbStorageMeta.currentHash,
				"deleteHash":  blockSha,
			})
		return database.ErrDeleteNonNewestBlock
	}

	var batch = db.Batch(blockBatch).Batch()
	var block *chainutil.Block
	height, buf, err := db.getBlock(blockSha)
	if err != nil {
		return err
	}
	block, err = chainutil.NewBlockFromBytes(buf, wire.DB)
	if err != nil {
		return err
	}

	if debug.DevMode() {
		if !bytes.Equal(blockSha[:], block.Hash()[:]) {
			logging.CPrint(logging.FATAL, "unexpected getBlock result",
				logging.LogFormat{
					"expect": blockSha,
					"actual": block.Hash(),
				})
		}
	}

	if err = insertPunishments(batch, block.MsgBlock().Proposals.PunishmentArea); err != nil {
		return err
	}

	if err = db.dropFaultPksByHeight(batch, height); err != nil {
		return err
	}

	for _, tx := range block.MsgBlock().Transactions {
		if err = db.unSpend(tx); err != nil {
			return err
		}
	}

	if err = db.freezeStakingTx(height); err != nil {
		return err
	}

	// remove pk, bitLength and height
	if err = db.removePubKeyBitLengthAndHeightWithCheck(batch, block.MsgBlock().Header.PubKey, block.MsgBlock().Header.Proof.BitLength, height); err != nil {
		return err
	}

	if err = updateMinedBlockIndex(batch, false, block.MsgBlock().Header.PubKey, height); err != nil {
		return err
	}

	// rather than iterate the list of tx backward, do it twice.
	for _, tx := range block.Transactions() {
		var txUo txUpdateEntry
		var txStk stakingTx
		txUo.delete = true
		db.txUpdateMap[*tx.Hash()] = &txUo

		// delete insert stakingTx in the block
		for i, txOut := range tx.MsgTx().TxOut {
			class, pushData := txscript.GetScriptInfo(txOut.PkScript)
			if class == txscript.StakingScriptHashTy {
				frozenPeriod, _, err := txscript.GetParsedOpcode(pushData, class)
				if err != nil {
					return err
				}
				txStk.expiredHeight = height + frozenPeriod
				txStk.delete = true
				var key = stakingTxMapKey{
					blockHeight: height,
					txID:        *tx.Hash(),
					index:       uint32(i),
				}
				db.stakingTxMap[key] = &txStk
			}
		}
	}

	batch.Delete(makeBlockShaKey(blockSha))
	batch.Delete(makeBlockHeightKey(height))
	// If height is 0, reset dbStorageMetaDataKey to initial value.
	// See NewChainDb(...)
	newStorageMeta := dbStorageMeta{
		currentHeight: UnknownHeight,
	}
	if height != 0 {
		lastHash, _, err := db.getBlockByHeight(height - 1)
		if err != nil {
			return err
		}
		newStorageMeta = dbStorageMeta{
			currentHeight: height - 1,
			currentHash:   *lastHash,
		}
	}
	batch.Put(dbStorageMetaDataKey, encodeDBStorageMetaData(newStorageMeta))

	return nil
}

func (db *ChainDb) processBlockBatch() error {
	var batch = db.Batch(blockBatch).Batch()

	if len(db.txUpdateMap) == 0 && len(db.txSpentUpdateMap) == 0 && len(db.stakingTxMap) == 0 && len(db.expiredStakingTxMap) == 0 && len(db.stakingAwardedRecordMap) == 0 {
		return nil
	}

	for txSha, txUp := range db.txUpdateMap {
		key := txShaToKey(&txSha)
		if txUp.delete {
			//log.Tracef("deleting tx %v", txSha)
			batch.Delete(key)
		} else {
			//log.Tracef("inserting tx %v", txSha)
			txDat := db.formatTx(txUp)
			batch.Put(key, txDat)
		}
	}

	for txSha, txSu := range db.txSpentUpdateMap {
		key := spentTxShaToKey(&txSha)
		if txSu.delete {
			//log.Tracef("deleting tx %v", txSha)
			batch.Delete(key)
		} else {
			//log.Tracef("inserting tx %v", txSha)
			txDat := db.formatTxFullySpent(txSu.txList)
			batch.Put(key, txDat)
		}
	}

	for mapKey, txL := range db.stakingTxMap {
		key := heightStakingTxToKey(txL.expiredHeight, mapKey)
		if txL.delete {
			batch.Delete(key)
		} else {
			txDat := db.formatSTx(txL)
			batch.Put(key, txDat)
		}
	}

	for mapKey, txU := range db.expiredStakingTxMap {
		key := heightExpiredStakingTxToKey(txU.expiredHeight, mapKey)
		if txU.delete {
			batch.Delete(key)
		} else {
			txDat := db.formatSTx(txU)
			batch.Put(key, txDat)
		}
	}

	for mapKey, record := range db.stakingAwardedRecordMap {
		key := stakingAwardedRecordToKey(mapKey)
		value := make([]byte, 8)
		binary.LittleEndian.PutUint64(value, record.AwardedTime)
		batch.Put(key, value)
	}

	db.txUpdateMap = map[wire.Hash]*txUpdateEntry{}
	db.txSpentUpdateMap = make(map[wire.Hash]*spentTxUpdate)
	db.stakingTxMap = map[stakingTxMapKey]*stakingTx{}
	db.expiredStakingTxMap = map[stakingTxMapKey]*stakingTx{}
	db.stakingAwardedRecordMap = map[stakingAwardedRecordMapKey]*database.StakingAwardedRecord{}
	return nil
}
func (db *ChainDb) InitByGenesisBlock(block *chainutil.Block) error {
	db.dbLock.Lock()
	defer db.dbLock.Unlock()

	if err := db.submitBlock(block, make(database.TxReplyStore)); err != nil {
		return err
	}

	db.dbStorageMeta.currentHash = *block.Hash()
	db.dbStorageMeta.currentHeight = 0

	return db.localStorage.Write(db.Batch(blockBatch).Batch())
}

// FetchBlockBySha - return a  Block
func (db *ChainDb) FetchBlockBySha(sha *wire.Hash) (block *chainutil.Block, err error) {
	return db.fetchBlockBySha(sha)
}

// fetchBlockBySha - return a  Block
// Must be called with db lock held.
func (db *ChainDb) fetchBlockBySha(sha *wire.Hash) (block *chainutil.Block, err error) {
	buf, height, err := db.fetchSha(sha)
	if err != nil {
		return
	}

	block, err = chainutil.NewBlockFromBytes(buf, wire.DB)
	if err != nil {
		return
	}
	block.SetHeight(height)

	if debug.DevMode() {
		if !bytes.Equal(sha[:], block.Hash()[:]) {
			logging.CPrint(logging.FATAL, "unexpected fetchSha result",
				logging.LogFormat{
					"expect": sha,
					"actual": block.Hash(),
				})
		}
	}
	return
}

// FetchBlockHeightBySha returns the block height for the given hash.  This is
// part of the database.Db interface implementation.
func (db *ChainDb) FetchBlockHeightBySha(blockSha *wire.Hash) (uint64, error) {
	return db.getBlockHeight(blockSha)
}

// FetchBlockHeaderBySha - return a Hash
func (db *ChainDb) FetchBlockHeaderBySha(sha *wire.Hash) (bh *wire.BlockHeader, err error) {

	// Read the raw block from the database.
	buf, _, err := db.fetchSha(sha)
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(buf)

	// Only deserialize the header portion and ensure the transaction count
	// is zero since this is a standalone header.
	// Read BLockBase length
	blockBaseLength, _, err := wire.ReadUint64(r, 0)
	if err != nil {
		return nil, err
	}

	// Read BlockBase
	baseData := make([]byte, blockBaseLength)
	_, err = r.Read(baseData)
	if err != nil {
		return nil, err
	}
	basePb := new(wirepb.BlockBase)
	err = proto.Unmarshal(baseData, basePb)
	if err != nil {
		return nil, err
	}
	base, err := wire.NewBlockBaseFromProto(basePb)
	if err != nil {
		return nil, err
	}

	bh = &base.Header

	return bh, nil
}

// getBlockHeight
// Key:
// +-------------------+----------------------+
// | BLKSHA (6 bytes)  | blockSha  (32 bytes) |
// +-------------------+----------------------+
// Value:
// +------------------------------------------+
// | Height (8 bytes)                         |
// +------------------------------------------+
func (db *ChainDb) getBlockHeight(sha *wire.Hash) (uint64, error) {
	key := makeBlockShaKey(sha)

	data, err := db.localStorage.Get(key)
	if err != nil {
		if err == storage.ErrNotFound {
			err = database.ErrBlockShaMissing
		}
		return 0, err
	}
	if len(data) != 8 {
		return 0, database.ErrInvalidBlockHeight
	}
	// deserialize
	blockHeight := binary.LittleEndian.Uint64(data)

	return blockHeight, nil
}

// getBlockLocationByHeight
// Key:
// +------------------+-----------------------------+
// | prefix (6 bytes) | Height (8 bytes)            |
// +------------------+-----------------------------+
// Value:
// +------------------------------------------------+ 0
// | block Sha (32 bytes )                          |
// +------------------------------------------------+ 32
// | file No (4 bytes)                              |
// +------------------------------------------------+ 36
// | offset (4 bytes)                               |
// +------------------------------------------------+ 44
// | len  (8 bytes)                                 |
// +------------------------------------------------+ 52
func (db *ChainDb) getBlockLocationByHeight(height uint64) (sha *wire.Hash, fileNo uint32, offset int64, size int64, err error) {
	var hgtValue []byte
	key := makeBlockHeightKey(height)
	hgtValue, err = db.localStorage.Get(key)
	if err != nil {
		logging.CPrint(logging.TRACE, "failed to get block loc", logging.LogFormat{"height": height, "err": err})
		return
	}

	if len(hgtValue) != 52 {
		logging.CPrint(logging.ERROR, "too short hgtValue", logging.LogFormat{"actual": len(hgtValue)})
		err = errors.New("too short hgtValue at getBlockLocByHeight")
		return
	}

	fileNo = binary.LittleEndian.Uint32(hgtValue[32:36])
	offset = int64(binary.LittleEndian.Uint64(hgtValue[36:44]))
	size = int64(binary.LittleEndian.Uint64(hgtValue[44:]))
	sha = &wire.Hash{}
	sha.SetBytes(hgtValue[0:32])
	return
}

// getBlockByHeight
// 1. get BlockLoc info from leveldb
// 2. get Block Info form file By Block Location info
func (db *ChainDb) getBlockByHeight(blockHeight uint64) (rawBlockSha *wire.Hash, rawBuf []byte, err error) {

	rawBlockSha, fileNo, offset, blockSize, err := db.getBlockLocationByHeight(blockHeight)
	if err != nil {
		return nil, nil, err
	}

	rawBuf, err = db.blockFileKeeper.ReadRawBlock(fileNo, offset, int(blockSize))
	if err != nil {
		logging.CPrint(logging.ERROR, "failed to read raw block", logging.LogFormat{"height": blockHeight, "err": err})
		return nil, nil, err
	}

	return rawBlockSha, rawBuf, nil
}

// getBlock
// blockSha --> blockHeight --> blockBuffer
// solution:
// A. blockSha --> blockHeight
// B. blockHeight --> blockSha
func (db *ChainDb) getBlock(blockSha *wire.Hash) (blockHeight uint64, rawBuf []byte, err error) {
	blockHeight, err = db.getBlockHeight(blockSha)
	if err != nil {
		return 0, nil, err
	}
	var buf []byte
	_, buf, err = db.getBlockByHeight(blockHeight)
	if err != nil {
		return
	}
	return blockHeight, buf, nil
}

// fetchSha returns the data block for the given Hash.
func (db *ChainDb) fetchSha(blockSha *wire.Hash) (rawBuf []byte,
	rawBlockHeight uint64, err error) {
	rawBlockHeight, rawBuf, err = db.getBlock(blockSha)
	if err != nil {
		return
	}
	return rawBuf, rawBlockHeight, nil
}

// ExistsSha looks up the given block hash
// returns true if it is present in the database.
func (db *ChainDb) ExistsBlockSha(sha *wire.Hash) (bool, error) {
	// not in cache, try database
	return db.blockExistsSha(sha)
}

// blockExistsSha looks up the given block hash
// returns true if it is present in the database.
// CALLED WITH LOCK HELD
func (db *ChainDb) blockExistsSha(sha *wire.Hash) (bool, error) {
	key := makeBlockShaKey(sha)
	return db.localStorage.Has(key)
}

// FetchBlockShaByHeight returns a block hash based on its height in the
// block chain.
func (db *ChainDb) FetchBlockShaByHeight(height uint64) (rawBlockSha *wire.Hash, err error) {
	return db.fetchBlockShaByHeight(height)
}

// FetchBlockLocByHeight
func (db *ChainDb) FetchBlockLocByHeight(height uint64) (*database.BlockLoc, error) {
	blockHash, fileNo, offset, len, err := db.getBlockLocationByHeight(height)
	if err != nil {
		return nil, err
	}
	return &database.BlockLoc{
		Height: height,
		Hash:   *blockHash,
		File:   fileNo,
		Offset: uint64(offset),
		Length: uint64(len),
	}, nil
}

// fetchBlockShaByHeight returns a block hash based on its height in the
// block chain.
func (db *ChainDb) fetchBlockShaByHeight(height uint64) (rawBlockSha *wire.Hash, err error) {
	key := makeBlockHeightKey(height)

	blockVal, err := db.localStorage.Get(key)
	if err != nil {
		logging.CPrint(logging.TRACE, "failed to find block on height", logging.LogFormat{"height": height})
		return // exists ???
	}

	var sha wire.Hash
	sha.SetBytes(blockVal[0:32])

	return &sha, nil
}

// FetchHeightRange looks up a range of blocks by the start and ending
// heights.  Fetch is inclusive of the start height and exclusive of the
// ending height.
func (db *ChainDb) FetchHeightRange(startHeight, endHeight uint64) ([]wire.Hash, error) {
	blockShaList := make([]wire.Hash, 0, endHeight-startHeight)
	for height := startHeight; height < endHeight; height++ {
		key := makeBlockHeightKey(height)

		blockVal, err := db.localStorage.Get(key)
		if err != nil {
			return nil, err
		}

		var blockSha wire.Hash
		blockSha.SetBytes(blockVal[0:32])
		blockShaList = append(blockShaList, blockSha)
	}

	return blockShaList, nil
}

// NewestSha returns the hash and block height of the most recent (end) block of
// the block chain.  It will return the zero hash, UnknownHeight for the block height, and
// no error (nil) if there are not any blocks in the database yet.
func (db *ChainDb) NewestSha() (rawBlockSha *wire.Hash, rawBlockHeight uint64, err error) {

	if db.dbStorageMeta.currentHeight == UnknownHeight {
		return &wire.Hash{}, UnknownHeight, nil
	}
	sha := db.dbStorageMeta.currentHash

	return &sha, db.dbStorageMeta.currentHeight, nil
}

// transition code that will be removed soon
func (db *ChainDb) IndexPubKeyBitLengthAndHeight(rebuild bool) error {
	if db.dbStorageMeta.currentHeight == UnknownHeight {
		return nil
	}
	height, err := db.fetchPubKeyBitLengthAndHeightIndexProgress()
	if err != nil {
		return err
	}
	logging.CPrint(logging.INFO, "build block-bl index start", logging.LogFormat{
		"current": height,
		"rebuild": rebuild,
	})

	if rebuild {
		err = db.deletePubKeyBitLengthAndHeightIndexProgress()
		if err != nil {
			logging.CPrint(logging.ERROR, "deletePubkblIndexProgress error", logging.LogFormat{
				"err": err,
			})
			return err
		}
	}

	// If height is 0, make sure to clear residual pubkbl within last rebuild call.
	if rebuild || height == 0 {
		err = db.clearPubKeyBitLengthAndHeight()
		if err != nil {
			logging.CPrint(logging.ERROR, "clearPubkbl error", logging.LogFormat{
				"err": err,
			})
			return err
		}
	}

	db.dbLock.Lock()
	defer db.dbLock.Unlock()

	i := uint64(0)
	if height != 0 {
		i = height + 1
	}
	for ; i <= db.dbStorageMeta.currentHeight; i++ {
		if i%1000 == 0 {
			logging.CPrint(logging.DEBUG, "updating pk & bl index", logging.LogFormat{
				"height": i,
			})
		}
		_, buf, err := db.getBlockByHeight(i)
		if err != nil {
			return err
		}
		block, err := chainutil.NewBlockFromBytes(buf, wire.DB)
		if err != nil {
			return err
		}

		err = db.insertPubKeyBLHeight(block.MsgBlock().Header.PubKey, block.MsgBlock().Header.Proof.BitLength, block.Height())
		if err != nil {
			return err
		}
		err = db.updatePubKeyBitLengthAndHeightIndexProgress(block.Height())
		if err != nil {
			return err
		}
	}
	logging.CPrint(logging.INFO, "build block-bl index done", logging.LogFormat{
		"current": i - 1,
		"rebuild": rebuild,
	})
	return db.deletePubKeyBitLengthAndHeightIndexProgress()
}
