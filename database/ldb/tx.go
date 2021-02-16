package ldb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/Sukhavati-Labs/go-miner/database"
	"github.com/Sukhavati-Labs/go-miner/database/storage"
	"github.com/Sukhavati-Labs/go-miner/debug"
	"github.com/Sukhavati-Labs/go-miner/logging"
	"github.com/Sukhavati-Labs/go-miner/wire"
)

type txUpdateEntry struct {
	txSha       *wire.Hash // txId
	blockHeight uint64     // height
	txOffset    int        // TxLocation TxStart
	txLen       int        // TxLocation TxLen
	ntxOut      int        //
	spentData   []byte     // spent data
	delete      bool       // queue tag
}

// spentTx --> STX
type spentTx struct {
	blockHeight uint64 // height
	txOffset    int    // TxLocation TxStart
	txLen       int    // TxLocation TxLen
	numTxOut    int    // num of txOut
	delete      bool   // queue tag
}

type spentTxUpdate struct {
	txList []*spentTx
	delete bool
}

// txShaToKey
// find Tx by txSha
// +---------------+-------------------+
// |TXD (3 bytes ) |  txSha (32 bytes) |
// +---------------+-------------------+
func txShaToKey(txSha *wire.Hash) []byte {
	key := make([]byte, len(recordSuffixTx)+len(txSha))
	copy(key, recordSuffixTx)
	copy(key[len(recordSuffixTx):], txSha[:])
	return key
}

// find full spent tx by txSha
// TxOut all tx spent
// +---------------+---------------------+
// |TXS (3 bytes)  |  txSha (32 bytes)   |
// +---------------+---------------------+
func spentTxShaToKey(txSha *wire.Hash) []byte {
	key := make([]byte, len(recordSuffixSpentTx)+len(txSha))
	copy(key, recordSuffixSpentTx)
	copy(key[len(recordSuffixSpentTx):], txSha[:])
	return key
}

// formatTx generates the value buffer for the Tx db.
// Value:
// +-----------------------+-----------------------+----------------------+
// | blockHeight (8 bytes) |  txOffset (4 bytes)   |  txLen (4 bytes)     |
// +-----------------------+-----------------------+----------------------+
func (db *ChainDb) formatTx(txu *txUpdateEntry) []byte {
	blockHeight := txu.blockHeight
	txOffset := uint32(txu.txOffset)
	txLen := uint32(txu.txLen)
	spentBuf := txu.spentData

	txW := make([]byte, 16+len(spentBuf))
	binary.LittleEndian.PutUint64(txW[0:8], blockHeight)
	binary.LittleEndian.PutUint32(txW[8:12], txOffset)
	binary.LittleEndian.PutUint32(txW[12:16], txLen)
	copy(txW[16:], spentBuf)

	return txW[:]
}

// insertTx inserts a tx hash and its associated data into the database.
// Must be called with db lock held.
// see also
//  - processBlockBatch
//  - formatTx
// Key:  txShaToKey
// Value: formatTx
func (db *ChainDb) insertTx(txSha *wire.Hash, height uint64, txOffset int, txLen int, spentBuf []byte) (err error) {
	var txU txUpdateEntry

	txU.txSha = txSha
	txU.blockHeight = height
	txU.txOffset = txOffset
	txU.txLen = txLen
	txU.spentData = spentBuf

	db.txUpdateMap[*txSha] = &txU

	return nil
}

// getTxData unspent tx
//
// Key：
// +---------------+-------------------+
// |TXD (3 bytes ) |  txSha (32 bytes) |
// +---------------+-------------------+
// Value:
// +-------------------+---------------+-----------------+----------------------+
// | height ( 8 bytes) |txOff(4 bytes) |  txLen (4 bytes)| spentBuf             |
// +-------------------+---------------+-----------------+----------------------+
func (db *ChainDb) getUnspentTxData(txSha *wire.Hash) (uint64, int, int, []byte, error) {
	key := txShaToKey(txSha)
	buf, err := db.localStorage.Get(key)
	if err != nil {
		return 0, 0, 0, nil, err
	}

	blockHeight := binary.LittleEndian.Uint64(buf[0:8])
	txOff := binary.LittleEndian.Uint32(buf[8:12])
	txLen := binary.LittleEndian.Uint32(buf[12:16])

	spentBuf := make([]byte, len(buf)-16)
	copy(spentBuf, buf[16:])

	return blockHeight, int(txOff), int(txLen), spentBuf, nil
}

func (db *ChainDb) GetUnspentTxData(txSha *wire.Hash) (uint64, int, int, error) {
	// TODO: lock?
	height, txOffset, txLen, _, err := db.getUnspentTxData(txSha)
	if err != nil {
		return 0, 0, 0, err
	} else {
		return height, txOffset, txLen, nil
	}
}

// Returns database.ErrTxShaMissing if txsha not exist
func (db *ChainDb) FetchLastFullySpentTxBeforeHeight(txSha *wire.Hash,
	height uint64) (msgTx *wire.MsgTx, blockHeight uint64, blockSha *wire.Hash, err error) {

	key := spentTxShaToKey(txSha)
	value, err := db.localStorage.Get(key)
	if err != nil {
		if err == storage.ErrNotFound {
			err = database.ErrTxShaMissing
		}
		return nil, 0, nil, err
	}
	if len(value)%20 != 0 {
		return nil, 0, nil, fmt.Errorf("expected multiple of 20 bytes, but get %d bytes", len(value))
	}

	i := len(value)/20 - 1
	for ; i >= 0; i-- {
		offset := i * 20
		blockHeight = binary.LittleEndian.Uint64(value[offset : offset+8])
		if blockHeight >= height {
			continue
		}
		txOffset := binary.LittleEndian.Uint32(value[offset+8 : offset+12])
		txLen := binary.LittleEndian.Uint32(value[offset+12 : offset+16])
		msgTx, blockSha, err = db.fetchTxDataByLoc(blockHeight, int(txOffset), int(txLen))
		if err != nil {
			return nil, 0, nil, err
		}
		if debug.DevMode() {
			actualSha := msgTx.TxHash()
			if !bytes.Equal(actualSha[:], txSha[:]) {
				logging.CPrint(logging.FATAL, fmt.Sprintf("mismatched tx hash, expect: %v, actual: %v", txSha, actualSha))
			}
		}
		break
	}
	return
}

// Returns storage.ErrNotFound if txsha not exist
// Key:
// +---------------+---------------------+
// |TXS (3 bytes)  |  txSha (32 bytes)   |
// +---------------+---------------------+
//
// Value:
// |<--------------------------------- fragment ----------------------------------------->|<------
// +-----------------------+----------------------+-------------------+-------------------+-------
// | blockHeight (8 bytes) | txOffset (4 bytes)   |  txLen (4 bytes)  | numTxOut (4 bytes)|
// +-----------------------+----------------------+-------------------+-------------------+-------
func (db *ChainDb) getTxFullySpent(txsha *wire.Hash) ([]*spentTx, error) {

	var badTxList, spentTxList []*spentTx

	key := spentTxShaToKey(txsha)
	buf, err := db.localStorage.Get(key)
	if err != nil {
		return badTxList, err
	}
	txListLen := len(buf) / 20

	spentTxList = make([]*spentTx, txListLen)
	for i := range spentTxList {
		offset := i * 20

		blockHeight := binary.LittleEndian.Uint64(buf[offset : offset+8])
		txOffset := binary.LittleEndian.Uint32(buf[offset+8 : offset+12])
		txLen := binary.LittleEndian.Uint32(buf[offset+12 : offset+16])
		numTxOut := binary.LittleEndian.Uint32(buf[offset+16 : offset+20])

		sTx := spentTx{
			blockHeight: blockHeight,
			txOffset:    int(txOffset),
			txLen:       int(txLen),
			numTxOut:    int(numTxOut),
		}

		spentTxList[i] = &sTx
	}

	return spentTxList, nil
}

func (db *ChainDb) formatTxFullySpent(sTxList []*spentTx) []byte {
	txW := make([]byte, 20*len(sTxList))

	for i, sTx := range sTxList {
		blockHeight := sTx.blockHeight
		txOffset := uint32(sTx.txOffset)
		txLen := uint32(sTx.txLen)
		numTxO := uint32(sTx.numTxOut)
		offset := i * 20

		binary.LittleEndian.PutUint64(txW[offset:offset+8], blockHeight)
		binary.LittleEndian.PutUint32(txW[offset+8:offset+12], txOffset)
		binary.LittleEndian.PutUint32(txW[offset+12:offset+16], txLen)
		binary.LittleEndian.PutUint32(txW[offset+16:offset+20], numTxO)
	}

	return txW
}

// ExistsTxSha returns if the given tx sha exists in the database
func (db *ChainDb) ExistsTxSha(txSha *wire.Hash) (bool, error) {

	return db.existsTxSha(txSha)
}

// existsTxSha returns if the given tx sha exists in the database.o
// Must be called with the db lock held.
func (db *ChainDb) existsTxSha(txSha *wire.Hash) (bool, error) {
	key := txShaToKey(txSha)

	return db.localStorage.Has(key)
}

// FetchTxByShaList returns the most recent tx of the name fully spent or not
func (db *ChainDb) FetchTxByShaList(txShaList []*wire.Hash) []*database.TxReply {

	// until the fully spent separation of tx is complete this is identical
	// to FetchUnSpentTxByShaList
	replies := make([]*database.TxReply, len(txShaList))
	for i, txSha := range txShaList {
		tx, blockSha, height, txSpent, err := db.fetchTxDataBySha(txSha)
		var btxSpent []bool
		if err == nil {
			btxSpent = make([]bool, len(tx.TxOut))
			for idx := range tx.TxOut {
				byteIdx := idx / 8
				byteOff := uint(idx % 8)
				btxSpent[idx] = (txSpent[byteIdx] & (byte(1) << byteOff)) != 0
			}
		} else {
			btxSpent = make([]bool, 0)
		}
		if err == database.ErrTxShaMissing {
			// if the unspent pool did not have the tx,
			// look in the fully spent pool (only last instance)
			tx, height, blockSha, err = db.FetchLastFullySpentTxBeforeHeight(txSha, math.MaxUint64)
			if err != nil && err != database.ErrTxShaMissing {
				logging.CPrint(logging.WARN, "FetchLastFullySpentTxBeforeHeight error",
					logging.LogFormat{"err": err, "tx": txSha.String()})
			}
			if err == nil {
				btxSpent = make([]bool, len(tx.TxOut))
				for i := range btxSpent {
					btxSpent[i] = true
				}
			}
		}
		txlre := database.TxReply{TxSha: txSha, Tx: tx, BlockSha: blockSha, Height: height, TxSpent: btxSpent, Err: err}
		replies[i] = &txlre
	}
	return replies
}

// FetchUnSpentTxByShaList given a array of Hash, look up the transactions
// and return them in a TxReply array.
func (db *ChainDb) FetchUnSpentTxByShaList(txShaList []*wire.Hash) []*database.TxReply {
	replies := make([]*database.TxReply, len(txShaList))
	for i, txsha := range txShaList {
		tx, blockSha, height, txSpent, err := db.fetchTxDataBySha(txsha)
		var btxSpent []bool
		if err == nil {
			btxSpent = make([]bool, len(tx.TxOut))
			for idx := range tx.TxOut {
				byteIdx := idx / 8
				byteOff := uint(idx % 8)
				btxSpent[idx] = (txSpent[byteIdx] & (byte(1) << byteOff)) != 0
			}
		} else {
			btxSpent = make([]bool, 0)
		}
		txlre := database.TxReply{TxSha: txsha, Tx: tx, BlockSha: blockSha, Height: height, TxSpent: btxSpent, Err: err}
		replies[i] = &txlre
	}
	return replies
}

// IsTxOutSpent
// index --> 32 bits
// +------------------------------------------------------
// |              index  | spent flag offset ( 8 bits )  |
// +-----------------------------------------------------+
//
//  txSpent
//  index(offset) --> spent
// +-----------------------------------------------------+
// | 1 | 0 |                                             |
// +-----------------------------------------------------+
func (db *ChainDb) IsTxOutSpent(txSha *wire.Hash, index int) (bool, error) {
	_, _, _, txSpent, err := db.fetchTxDataBySha(txSha)
	if err != nil {
		return false, err
	}
	if index < 0 || index > len(txSpent) {
		return false, errors.New("Error Tx spent index ")
	}
	byteIdx := index / 8
	byteOff := uint(index % 8)
	return (txSpent[byteIdx] & (byte(1) << byteOff)) != 0, nil
}

func (db *ChainDb) FetchUnSpentStaking(txShaList []*wire.Hash) []*database.TxReply {
	replies := make([]*database.TxReply, len(txShaList))
	for i, txSha := range txShaList {
		tx, blockSha, height, txSpent, err := db.fetchTxDataBySha(txSha)
		var btxSpent []bool
		if err == nil {
			btxSpent = make([]bool, len(tx.TxOut))
			for idx := range tx.TxOut {
				byteIdx := idx / 8
				byteOff := uint(idx % 8)
				btxSpent[idx] = (txSpent[byteIdx] & (byte(1) << byteOff)) != 0
			}
		} else {
			btxSpent = make([]bool, 0)
		}
		replies[i] = &database.TxReply{TxSha: txSha, Tx: tx, BlockSha: blockSha, Height: height, TxSpent: btxSpent, Err: err}
	}
	return replies
}

// fetchTxDataBySha returns several pieces of data regarding the given sha.
func (db *ChainDb) fetchTxDataBySha(txSha *wire.Hash) (rawTx *wire.MsgTx, rawBlockSha *wire.Hash, rawHeight uint64, rawTxSpent []byte, err error) {
	var txOff, txLen int

	rawHeight, txOff, txLen, rawTxSpent, err = db.getUnspentTxData(txSha)
	if err != nil {
		if err == storage.ErrNotFound {
			err = database.ErrTxShaMissing
		}
		return
	}
	rawTx, rawBlockSha, err = db.fetchTxDataByLoc(rawHeight, txOff, txLen)
	if err != nil {
		rawTxSpent = nil
	}
	if debug.DevMode() {
		actualSha := rawTx.TxHash()
		if !bytes.Equal(actualSha[:], txSha[:]) {
			logging.CPrint(logging.FATAL, fmt.Sprintf("mismatched tx hash, expect: %v, actual: %v", txSha, actualSha))
		}
	}
	return
}

// fetchTxDataByLoc returns several pieces of data regarding the given tx
// located by the block/offset/size location
func (db *ChainDb) fetchTxDataByLoc(blockHeight uint64, txOff int, txLen int) (rawTx *wire.MsgTx, rawBlockSha *wire.Hash, err error) {

	blockSha, fileNo, blockOffset, _, err := db.getBlockLocationByHeight(blockHeight)
	if err != nil {
		return nil, nil, err
	}
	buf, err := db.blockFileKeeper.ReadRawTx(fileNo, blockOffset, int64(txOff), txLen)
	if err != nil {
		return nil, nil, err
	}

	var tx wire.MsgTx
	err = tx.SetBytes(buf, wire.DB)
	if err != nil {
		logging.CPrint(logging.WARN, "unable to decode tx",
			logging.LogFormat{"blockHash": blockSha, "blockHeight": blockHeight, "txOff": txOff, "txLen": txLen})
		return
	}

	return &tx, blockSha, nil
}

func (db *ChainDb) FetchTxByLoc(blockHeight uint64, txOff int, txLen int) (*wire.MsgTx, error) {
	msgTx, _, err := db.fetchTxDataByLoc(blockHeight, txOff, txLen)
	if err != nil {
		return nil, err
	}
	return msgTx, nil
}

func (db *ChainDb) FetchTxByFileLoc(blockLocation *database.BlockLocation, txLoc *wire.TxLoc) (*wire.MsgTx, error) {
	buf, err := db.blockFileKeeper.ReadRawTx(blockLocation.File, int64(blockLocation.Offset), int64(txLoc.TxStart), txLoc.TxLen)
	if err != nil {
		return nil, err
	}

	var tx wire.MsgTx
	err = tx.SetBytes(buf, wire.DB)
	if err != nil {
		logging.CPrint(logging.WARN, "unable to decode tx",
			logging.LogFormat{
				"err":       err,
				"blockFile": blockLocation.File,
				"blockOff":  blockLocation.Offset,
				"blockLen":  blockLocation.Length,
				"txOff":     txLoc.TxStart,
				"txLen":     txLoc.TxLen,
			})
		return nil, err
	}
	return &tx, nil
}

// FetchTxBySha returns some data for the given Tx Sha.
// Be careful, main chain may be revoked during invocation.
func (db *ChainDb) FetchTxBySha(txSha *wire.Hash) ([]*database.TxReply, error) {

	// fully spent
	sTxList, fullySpentErr := db.getTxFullySpent(txSha)
	if fullySpentErr != nil && fullySpentErr != storage.ErrNotFound {
		return []*database.TxReply{}, fullySpentErr
	}

	replies := make([]*database.TxReply, 0, len(sTxList)+1)
	for _, stx := range sTxList {
		tx, blockSha, err := db.fetchTxDataByLoc(stx.blockHeight, stx.txOffset, stx.txLen)
		if err != nil {
			if err != database.ErrTxShaMissing {
				return []*database.TxReply{}, err
			}
			continue
		}
		if debug.DevMode() {
			actualSha := tx.TxHash()
			if !bytes.Equal(actualSha[:], txSha[:]) {
				logging.CPrint(logging.FATAL, fmt.Sprintf("mismatched tx hash, expect: %v, actual: %v", txSha, actualSha))
			}
		}

		btxSpent := make([]bool, len(tx.TxOut))
		for i := range btxSpent {
			btxSpent[i] = true

		}
		txlre := database.TxReply{TxSha: txSha, Tx: tx, BlockSha: blockSha, Height: stx.blockHeight, TxSpent: btxSpent, Err: nil}
		replies = append(replies, &txlre)
	}

	// not fully spent
	tx, blockSha, height, txSpent, txErr := db.fetchTxDataBySha(txSha)
	if txErr != nil && txErr != database.ErrTxShaMissing {
		return []*database.TxReply{}, txErr
	}
	if txErr == nil {
		btxSpent := make([]bool, len(tx.TxOut), len(tx.TxOut))
		for idx := range tx.TxOut {
			byteIdx := idx / 8
			byteOff := uint(idx % 8)
			btxSpent[idx] = (txSpent[byteIdx] & (byte(1) << byteOff)) != 0
		}

		txlre := database.TxReply{TxSha: txSha, Tx: tx, BlockSha: blockSha, Height: height, TxSpent: btxSpent, Err: nil}
		replies = append(replies, &txlre)
	}
	return replies, nil
}

// addrIndexToKey serializes the passed txAddrIndex for storage within the DB.
// We want to use BigEndian to store at least block height and TX offset
// in order to ensure that the transactions are sorted in the index.
// This gives us the ability to use the index in more client-side
// applications that are order-dependent (specifically by dependency).
//func addrIndexToKey(index *txAddrIndex) []byte {
//	record := make([]byte, addrIndexKeyLength, addrIndexKeyLength)
//	copy(record[0:3], addrIndexKeyPrefix)
//	record[3] = index.addrVersion
//	copy(record[4:24], index.hash160[:])
//
//	// The index itself.
//	binary.LittleEndian.PutUint64(record[24:32], index.blockHeight)
//	binary.LittleEndian.PutUint32(record[32:36], uint32(index.txoffset))
//	binary.LittleEndian.PutUint32(record[36:40], uint32(index.txlen))
//	binary.LittleEndian.PutUint32(record[40:44], index.index)
//
//	return record
//}

// unpackTxIndex deserializes the raw bytes of a address tx index.
//func unpackTxIndex(rawIndex [20]byte) *txAddrIndex {
//	return &txAddrIndex{
//		blockHeight: binary.LittleEndian.Uint64(rawIndex[0:8]),
//		txOffset:  int(binary.LittleEndian.Uint32(rawIndex[8:12])),
//		txLen:     int(binary.LittleEndian.Uint32(rawIndex[12:16])),
//		index:     binary.LittleEndian.Uint32(rawIndex[16:20]),
//	}
//}
// doSpend iterates all TxIn in a transaction marking each associated
// TxOut as spent.
//
// coinbaseTx
// vin:
// TODO:
func (db *ChainDb) doSpend(tx *wire.MsgTx) error {
	if tx == nil {
		return nil
	}
	if tx.IsCoinBaseTx() {
		for _, txIn := range tx.TxIn[1:] {
			inTxSha := txIn.PreviousOutPoint.Hash
			inTxIdx := txIn.PreviousOutPoint.Index
			if inTxIdx == wire.MaxPrevOutIndex {
				continue
			}

			err := db.setSpentData(&inTxSha, inTxIdx)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, txIn := range tx.TxIn {
		inTxSha := txIn.PreviousOutPoint.Hash
		inTxIdx := txIn.PreviousOutPoint.Index

		if inTxIdx == wire.MaxPrevOutIndex {
			continue
		}

		err := db.setSpentData(&inTxSha, inTxIdx)
		if err != nil {
			return err
		}
	}
	return nil
}

// unSpend iterates all TxIn in a skt transaction marking each associated
// TxOut as unspent.
func (db *ChainDb) unSpend(tx *wire.MsgTx) error {
	for txInIdx := range tx.TxIn {
		txIn := tx.TxIn[txInIdx]

		inTxSha := txIn.PreviousOutPoint.Hash
		inTxIdx := txIn.PreviousOutPoint.Index

		if inTxIdx == wire.MaxPrevOutIndex {
			continue
		}

		err := db.clearSpentData(&inTxSha, inTxIdx)
		if err != nil {
			return err
		}
	}
	return nil
}

//
func (db *ChainDb) setSpentData(sha *wire.Hash, idx uint32) error {
	return db.setClearSpentData(sha, idx, true)
}

func (db *ChainDb) clearSpentData(sha *wire.Hash, idx uint32) error {
	return db.setClearSpentData(sha, idx, false)
}

func (db *ChainDb) setClearSpentData(txSha *wire.Hash, idx uint32, isSpent bool) error {
	var txUo *txUpdateEntry
	var ok bool

	if txUo, ok = db.txUpdateMap[*txSha]; !ok {
		// not cached, load from db
		var txU txUpdateEntry
		blockHeight, txOffset, txLen, spentData, err := db.getUnspentTxData(txSha)
		if err != nil {
			// setting a fully spent tx is an error.
			if err != storage.ErrNotFound || isSpent {
				return err
			}
			// if we are clearing a tx and it wasn't found
			// in the tx table, it could be in the fully spent
			// (duplicates) table.
			spentTxList, err := db.getTxFullySpent(txSha)
			if err != nil {
				return err
			}
			// need to re slice the list to exclude the most recent.
			sTx := spentTxList[len(spentTxList)-1]
			if len(spentTxList) == 1 {
				// write entry to delete tx from spent pool
				db.txSpentUpdateMap[*txSha] = &spentTxUpdate{delete: true}
			} else {
				db.txSpentUpdateMap[*txSha] = &spentTxUpdate{txList: spentTxList[:len(spentTxList)-1]}
			}
			// Create 'new' Tx update data.
			blockHeight = sTx.blockHeight
			txOffset = sTx.txOffset
			txLen = sTx.txLen
			spentBufLen := (sTx.numTxOut + 7) / 8
			spentData = make([]byte, spentBufLen)
			for i := range spentData {
				spentData[i] = ^byte(0)
			}
		}

		txU.txSha = txSha
		txU.blockHeight = blockHeight
		txU.txOffset = txOffset
		txU.txLen = txLen
		txU.spentData = spentData

		txUo = &txU
	}

	byteIdx := idx / 8 // 29 bits
	byteOff := idx % 8 // 3 bits

	//
	if isSpent {
		txUo.spentData[byteIdx] |= byte(1) << byteOff
	} else {
		txUo.spentData[byteIdx] &= ^(byte(1) << byteOff)
	}

	// check for fully spent Tx
	fullySpent := true
	for _, val := range txUo.spentData {
		if val != ^byte(0) {
			fullySpent = false
			break
		}
	}
	if fullySpent {
		var txSu *spentTxUpdate
		// Look up Tx in fully spent table
		if txSuOld, ok := db.txSpentUpdateMap[*txSha]; ok {
			txSu = txSuOld
		} else {
			txSu = &spentTxUpdate{}
			txSuOld, err := db.getTxFullySpent(txSha)
			if err == nil {
				txSu.txList = txSuOld
			} else if err != storage.ErrNotFound {
				return err
			}
		}

		// Fill in spentTx
		var sTx spentTx
		sTx.blockHeight = txUo.blockHeight
		sTx.txOffset = txUo.txOffset
		sTx.txLen = txUo.txLen
		// XXX -- there is no way to compute the real TxOut
		// from the spent array.
		sTx.numTxOut = 8 * len(txUo.spentData)

		// append this txData to fully spent txList
		txSu.txList = append(txSu.txList, &sTx)

		// mark txsha as deleted in the txUpdateMap
		logging.CPrint(logging.TRACE, "tx %v is fully spent", logging.LogFormat{"txHash": txSha})

		db.txSpentUpdateMap[*txSha] = txSu

		txUo.delete = true
	}

	db.txUpdateMap[*txSha] = txUo
	return nil
}
