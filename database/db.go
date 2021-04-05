package database

import (
	"crypto/sha256"
	"errors"

	"github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/config"
	"github.com/Sukhavati-Labs/go-miner/pocec"
	"github.com/Sukhavati-Labs/go-miner/wire"
	"golang.org/x/crypto/ripemd160"
)

// Errors that the various database functions may return.
var (
	ErrAddrIndexDoesNotNeedInit = errors.New("address index has been initialized")
	ErrAddrIndexDoesNotExist    = errors.New("address index hasn't been built or is an older version")
	ErrAddrIndexVersionNotFound = errors.New("address index version not found")
	ErrAddrIndexInvalidVersion  = errors.New("address index version is not allowed")
	ErrUnsupportedAddressType   = errors.New("address type is not supported by the address-index")
	ErrPrevShaMissing           = errors.New("previous sha missing from database")
	ErrTxShaMissing             = errors.New("requested transaction does not exist")
	ErrBlockShaMissing          = errors.New("requested block does not exist")
	ErrInvalidBlockHeight       = errors.New("invalid block height buffer")
	ErrDuplicateSha             = errors.New("duplicate insert attempted")
	ErrDbDoesNotExist           = errors.New("non-existent database")
	ErrDbUnknownType            = errors.New("non-existent database type")
	ErrNotImplemented           = errors.New("method has not yet been implemented")
	ErrInvalidBlockStorageMeta  = errors.New("invalid block storage meta")
	ErrInvalidAddrIndexMeta     = errors.New("invalid addr index meta")
	ErrDeleteNonNewestBlock     = errors.New("delete block that is not newest")
)

// DB defines a generic interface that is used to request and insert data into
// the block chain.  This interface is intended to be agnostic to actual
// mechanism used for backend data storage.  The AddDBDriver function can be
// used to add a new backend data storage method.
type DB interface {
	// Close cleanly shuts down the database and syncs all data.
	Close() (err error)

	// RollbackClose discards the recent database changes to the previously
	// saved data at last Sync and closes the database.
	RollbackClose() (err error)
	Rollback()

	// Sync verifies that the database is coherent on disk and no
	// outstanding transactions are in flight.
	Sync() (err error)

	// Commit commits batches in a single transaction
	Commit(blockSha wire.Hash) error

	// InitByGenesisBlock init database by setting genesis block
	InitByGenesisBlock(block *chainutil.Block) (err error)

	// SubmitBlock inserts raw block and transaction data from a block
	// into the database.  The first block inserted into the database
	// will be treated as the genesis block.  Every subsequent block insert
	// requires the referenced parent block to already exist.
	SubmitBlock(block *chainutil.Block, inputTxStore TxReplyStore) (err error)

	// DeleteBlock will remove any blocks from the database after
	// the given block.  It terminates any existing transaction and performs
	// its operations in an atomic transaction which is committed before
	// the function returns.
	DeleteBlock(blockSha *wire.Hash) (err error)

	// ExistsSha returns whether or not the given block hash is present in
	// the database.
	ExistsBlockSha(blockSha *wire.Hash) (exists bool, err error)

	// FetchBlockBySha returns a Block.  The implementation may
	// cache the underlying data if desired.
	FetchBlockBySha(blockSha *wire.Hash) (block *chainutil.Block, err error)

	// FetchBlockHeightBySha returns the block height for the given hash.
	FetchBlockHeightBySha(blockSha *wire.Hash) (height uint64, err error)
	// FetchBlockHeaderBySha returns a wire.BlockHeader for the given
	// sha.  The implementation may cache the underlying data if desired.
	FetchBlockHeaderBySha(blockSha *wire.Hash) (blockHeader *wire.BlockHeader, err error)
	// FetchBlockShaByHeight returns a block hash based on its height in the
	// block chain.
	FetchBlockShaByHeight(height uint64) (blockSha *wire.Hash, err error)
	// FetchBlockLocByHeight
	FetchBlockLocByHeight(height uint64) (*BlockLoc, error)
	// ExistsTxSha returns whether or not the given tx hash is present in
	// the database
	ExistsTxSha(txSha *wire.Hash) (exists bool, err error)
	// FetchTxByLoc
	FetchTxByLoc(blockHeight uint64, txOffset int, txLen int) (*wire.MsgTx, error)
	// FetchTxByFileLoc returns transactions saved in file, including
	// those revoked with chain reorganization, for file is in APPEND mode.
	FetchTxByFileLoc(blockLoc *BlockLoc, txLoc *wire.TxLoc) (*wire.MsgTx, error)
	// FetchTxBySha returns some data for the given transaction hash. The
	// implementation may cache the underlying data if desired.
	FetchTxBySha(txSha *wire.Hash) ([]*TxReply, error)
	// GetTxData returns the block height, txOffset, txLen for the given transaction hash,
	// including unspent and fully spent transaction
	GetUnspentTxData(txSha *wire.Hash) (uint64, int, int, error)
	// IsTxOutSpent
	IsTxOutSpent(txSha *wire.Hash, index int) (bool, error)
	// fetch unspent staking pool tx
	//FetchUnspentStakingPoolTx()([]*TxReply, error)
	// FetchTxByShaList returns a TxReply given an array of transaction
	// hashes.  The implementation may cache the underlying data if desired.
	// This differs from FetchUnSpentTxByShaList in that it will return
	// the most recent known Tx, if it is fully spent or not.
	//
	// NOTE: This function does not return an error directly since it MUST
	// return at least one TxReply instance for each requested
	// transaction.  Each TxReply instance then contains an Err field
	// which can be used to detect errors.
	FetchTxByShaList(txShaList []*wire.Hash) []*TxReply

	// Returns database.ErrTxShaMissing if no transaction found
	FetchLastFullySpentTxBeforeHeight(txSha *wire.Hash, height uint64) (tx *wire.MsgTx, blockHeight uint64, blockSha *wire.Hash, err error)

	// FetchUnSpentTxByShaList returns a TxReply given an array of
	// transaction hashes.  The implementation may cache the underlying
	// data if desired. Fully spent transactions will not normally not
	// be returned in this operation.
	//
	// NOTE: This function does not return an error directly since it MUST
	// return at least one TxReply instance for each requested
	// transaction.  Each TxReply instance then contains an Err field
	// which can be used to detect errors.
	FetchUnSpentTxByShaList(txShaList []*wire.Hash) []*TxReply
	// FetchUnSpentStakingPoolTxOutByHeight
	FetchUnSpentStakingPoolTxOutByHeight(startHeight uint64, endHeight uint64) ([]*TxOutReply, error)
	// FetchUnexpiredStakingRank returns only currently unexpired staking rank at
	// target height. This function is for mining and validating block.
	FetchUnexpiredStakingRank(height uint64, onlyOnList bool) ([]Rank, error)
	// Fetch
	FetchStakingStakingRewardInfo(height uint64) (*StakingRewardInfo, error)

	// FetchStakingAwardedRecordByTime
	FetchStakingAwardedRecordByTime(queryTime uint64) ([]StakingAwardedRecord, error)
	// InsertGovernConfig
	InsertGovernConfig(id uint32, height, activeHeight uint64, txSha *wire.Hash, data []byte) error
	// FetchStakingRank returns staking rank at any height. This
	// function may be slow.
	FetchStakingRank(height uint64, onlyOnList bool) ([]Rank, error)

	// fetch a map of all staking transactions in database
	FetchStakingTxMap() (StakingNodes, error)

	FetchExpiredStakingTxListByHeight(height uint64) (StakingNodes, error)

	FetchHeightRange(startHeight, endHeight uint64) ([]wire.Hash, error)

	// NewestSha returns the hash and block height of the most recent (end)
	// block of the block chain.  It will return the zero hash, -1 for
	// the block height, and no error (nil) if there are not any blocks in
	// the database yet.
	NewestSha() (blockSha *wire.Hash, height uint64, err error)

	// FetchFaultPubKeyBySha returns the FaultPubKey by Hash of that PubKey.
	// Hash = DoubleHashB(PubKey.SerializeUnCompressed()).
	// It will return ErrNotFound if this PubKey is not banned.
	FetchFaultPubKeyBySha(sha *wire.Hash) (faultPubKey *wire.FaultPubKey, height uint64, err error)

	FetchAllFaultPubKeys() ([]*wire.FaultPubKey, []uint64, error)

	// FetchFaultPubKeyListByHeight returns the newly added FaultPubKey List
	// on given height. It will return ErrNotFound if this height has not
	// mined. And will return empty slice if there aren't any new banned Pks.
	FetchFaultPubKeyListByHeight(blockHeight uint64) ([]*wire.FaultPubKey, error)

	// ExistsFaultPubKey returns whether or not the given FaultPubKey hash is present in
	// the database.
	ExistsFaultPubKey(sha *wire.Hash) (bool, error)

	// FetchAllPunishment returns all faultPubKey stored in db, with random order.
	FetchAllPunishment() ([]*wire.FaultPubKey, error)

	// ExistsPunishment returns whether or not the given PoC PublicKey is present in
	// the database.
	ExistsPunishment(pubKey *pocec.PublicKey) (bool, error)

	// InsertPunishment insert a fpk into punishment storage instantly.
	InsertPunishment(faultPubKey *wire.FaultPubKey) error

	FetchMinedBlocks(pubKey *pocec.PublicKey) ([]uint64, error)

	// FetchAddrIndexTip returns the hash and block height of the most recent
	// block which has had its address index populated. It will return
	// ErrAddrIndexDoesNotExist along with a zero hash, and math.MaxUint64 if
	// it hasn't yet been built up.
	FetchAddrIndexTip() (sha *wire.Hash, height uint64, err error)

	SubmitAddrIndex(hash *wire.Hash, height uint64, addrIndexData *AddrIndexData) (err error)

	DeleteAddrIndex(hash *wire.Hash, height uint64) (err error)

	// FetchScriptHashRelatedTx  returns all relevant txHash mapped by block height
	FetchScriptHashRelatedTx(scriptHashes [][]byte, startBlock, stopBlock uint64) (map[uint64][]*wire.TxLoc, error)

	CheckScriptHashUsed(scriptHash []byte) (bool, error)

	FetchScriptHashRelatedBindingTx(scriptHash []byte, chainParams *config.Params) ([]*BindingTxReply, error)
	// For testing purpose
	ExportDbEntries() map[string][]byte

	IndexPubKeyBLHeight(rebuild bool) error

	GetPubkeyBLHeightRecord(*pocec.PublicKey) ([]*BLHeight, error)
}

// TxReply is used to return individual transaction information when
// data about multiple transactions is requested in a single call.
// see also TxData
type TxReply struct {
	TxSha    *wire.Hash
	Tx       *wire.MsgTx
	BlockSha *wire.Hash
	Height   uint64
	TxSpent  []bool
	Err      error
}

type TxReplyStore map[wire.Hash]*TxReply

type UtxoReply struct {
	TxSha    *wire.Hash
	Height   uint64
	Coinbase bool
	Index    uint32
	Value    chainutil.Amount
}

// Staking Awarded Record
type StakingAwardedRecord struct {
	AwardedTime uint64    // award timestamp
	TxId        wire.Hash // award tx sha
}

type StakingRewardInfo struct {
	CurrentTime     uint64
	LastRecord      StakingAwardedRecord
	RewardAddresses []Rank
}

//
// only Tx Out info
type TxOutReply struct {
	TxSha    *wire.Hash // Tx id
	BlockSha *wire.Hash // block sha
	Spent    bool       // spent
	Height   uint64     // block height
	Coinbase bool       // is coinbase
	Index    uint32     // tx out index
	Value    int64      // tx out value
	PkScript []byte     // pubKey script
}

// AddrIndexKeySize is the number of bytes used by keys into the BlockAddrIndex.
// tianwei.cai: type + addr
const AddrIndexKeySize = ripemd160.Size + 1

type AddrIndexOutPoint struct {
	TxLoc *wire.TxLoc
	Index uint32
}

// BindingTx --> BTx
type BindingTxSpent struct {
	SpentTxLoc     *wire.TxLoc
	BTxBlockHeight uint64
	BTxLoc         *wire.TxLoc
	BTxIndex       uint32
}

type BindingTxReply struct {
	Height     uint64
	TxSha      *wire.Hash
	IsCoinbase bool
	Value      int64
	Index      uint32
}

type SenateEquities []SenateEquity

type StakingTxOutAtHeight map[uint64]map[wire.OutPoint]StakingTxInfo

type StakingAwardRecordAtTime map[uint64]map[wire.Hash]StakingAwardedRecord

// Returns false if already exists
func (sh StakingTxOutAtHeight) Put(op wire.OutPoint, stk StakingTxInfo) (success bool) {
	m, ok := sh[stk.BlockHeight]
	if !ok {
		m = make(map[wire.OutPoint]StakingTxInfo)
		sh[stk.BlockHeight] = m
	}
	if _, exist := m[op]; exist {
		return false
	}
	m[op] = stk
	return true
}

func (sar StakingAwardRecordAtTime) Put(record StakingAwardedRecord) (success bool) {
	d := record.AwardedTime / 86400
	m, ok := sar[d]
	if !ok {
		m = make(map[wire.Hash]StakingAwardedRecord)
		sar[d] = m
	}
	if _, exist := m[record.TxId]; exist {
		return false
	}
	m[record.TxId] = record
	return true
}

type StakingNodes map[[sha256.Size]byte]StakingTxOutAtHeight

func (nodes StakingNodes) Get(key [sha256.Size]byte) StakingTxOutAtHeight {
	m, ok := nodes[key]
	if !ok {
		m = make(StakingTxOutAtHeight)
		nodes[key] = m
	}
	return m
}

func (nodes StakingNodes) IsEmpty(key [sha256.Size]byte) bool {
	for _, v := range nodes[key] {
		if len(v) > 0 {
			return false
		}
	}
	return true
}

func (nodes StakingNodes) Delete(key [sha256.Size]byte, blockHeight uint64, op wire.OutPoint) bool {
	m1, ok := nodes[key]
	if !ok {
		return false
	}

	m2, ok := m1[blockHeight]
	if !ok {
		return false
	}

	_, ok = m2[op]
	if !ok {
		return false
	}
	delete(m2, op)
	return true
}

// BlockAddrIndex represents the indexing structure for addresses.
// It maps a hash160 to a list of transaction locations within a block that
// either pays to or spends from the passed UTXO for the hash160.
type BlockAddrIndex map[[AddrIndexKeySize]byte][]*AddrIndexOutPoint

// BlockTxAddrIndex represents the indexing structure for txs
type TxAddrIndex map[[sha256.Size]byte][]*wire.TxLoc

type BindingTxAddrIndex map[[ripemd160.Size]byte][]*AddrIndexOutPoint

type BindingTxSpentAddrIndex map[[ripemd160.Size]byte][]*BindingTxSpent

type AddrIndexData struct {
	TxIndex             TxAddrIndex
	BindingTxIndex      BindingTxAddrIndex
	BindingTxSpentIndex BindingTxSpentAddrIndex
}

type BLHeight struct {
	BitLength   int    // Bit Length Of Public Key
	BlockHeight uint64 // Block Height
}

// BlockLoc
type BlockLoc struct {
	Height uint64    // Block Height
	Hash   wire.Hash // Block Hash
	File   uint32    // Block File Number
	Offset uint64    // Block File Offset
	Length uint64    // Block Length
}
