package ldb

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sync"

	"github.com/Sukhavati-Labs/go-miner/database"
	"github.com/Sukhavati-Labs/go-miner/database/disk"
	"github.com/Sukhavati-Labs/go-miner/database/storage"
	_ "github.com/Sukhavati-Labs/go-miner/database/storage/ldbstorage"
	"github.com/Sukhavati-Labs/go-miner/errors"
	"github.com/Sukhavati-Labs/go-miner/logging"
	"github.com/Sukhavati-Labs/go-miner/wire"
)

const (
	// params for ldb batch
	blockBatch       = 0
	addrIndexBatch   = 1
	chainGovernBatch = 2
	dbBatchCount     = 2

	blockStorageMetaDataLength = 40
)

var (
	recordSuffixTx      = []byte("TXD") // tx record suffix
	recordSuffixSpentTx = []byte("TXS") // tx spent record suffix | full spent Tx
)

// ChainDb holds internal state for database.
type ChainDb struct {
	// lock preventing multiple entry
	dbLock sync.Mutex

	localStorage    storage.Storage
	blockFileKeeper *disk.BlockFileKeeper

	dbBatch       storage.Batch
	batches       [dbBatchCount]*LBatch
	dbStorageMeta dbStorageMeta

	// async update map
	txUpdateMap      map[wire.Hash]*txUpdateEntry
	txSpentUpdateMap map[wire.Hash]*spentTxUpdate

	stakingTxMap            map[stakingTxMapKey]*stakingTx
	expiredStakingTxMap     map[stakingTxMapKey]*stakingTx
	stakingAwardedRecordMap map[stakingAwardedRecordMapKey]*stakingAwardedRecord
	governConfigMap         map[governConfigMapKey]*governConfig
}

func init() {
	for _, dbType := range storage.RegisteredDbTypes() {
		database.RegisterDriver(database.Driver{
			DbType: dbType,
			Create: func(args ...interface{}) (database.DB, error) {
				if len(args) < 1 {
					return nil, storage.ErrInvalidArgument
				}
				dbpath, ok := args[0].(string)
				if !ok {
					return nil, storage.ErrInvalidArgument
				}

				dbStorage, err := storage.CreateStorage(dbType, dbpath, nil)
				if err != nil {
					return nil, err
				}
				return NewChainDb(dbpath, dbStorage)
			},
			Open: func(args ...interface{}) (database.DB, error) {
				if len(args) < 1 {
					return nil, storage.ErrInvalidArgument
				}
				dbpath, ok := args[0].(string)
				if !ok {
					return nil, storage.ErrInvalidArgument
				}

				dbStorage, err := storage.OpenStorage(dbType, dbpath, nil)
				if err != nil {
					return nil, err
				}
				return NewChainDb(dbpath, dbStorage)
			},
		})
	}
}

func NewChainDb(dbpath string, dbStorage storage.Storage) (*ChainDb, error) {
	cdb := &ChainDb{
		localStorage:            dbStorage,
		dbBatch:                 dbStorage.NewBatch(),
		txUpdateMap:             make(map[wire.Hash]*txUpdateEntry),
		txSpentUpdateMap:        make(map[wire.Hash]*spentTxUpdate),
		stakingTxMap:            make(map[stakingTxMapKey]*stakingTx),
		expiredStakingTxMap:     make(map[stakingTxMapKey]*stakingTx),
		stakingAwardedRecordMap: make(map[stakingAwardedRecordMapKey]*stakingAwardedRecord),
		governConfigMap:         make(map[governConfigMapKey]*governConfig),
	}

	blockMeta, err := cdb.getBlockStorageMeta()
	if err != nil {
		if err != storage.ErrNotFound {
			return nil, err
		}
		blockMeta = dbStorageMeta{
			currentHeight: UnknownHeight,
		}
	}
	cdb.dbStorageMeta = blockMeta

	// Load address indexer
	if err = cdb.checkAddrIndexVersion(); err == nil {
		logging.CPrint(logging.INFO, "address index good, continuing")
	} else {
		if err != storage.ErrNotFound {
			return nil, err
		}
		var b2 [2]byte
		binary.LittleEndian.PutUint16(b2[:], uint16(addrIndexCurrentVersion))
		if err = cdb.localStorage.Put(addrIndexVersionKey, b2[:]); err != nil {
			return nil, err
		}
	}

	// init or load blk*.dat
	blockDir := filepath.Join(filepath.Dir(dbpath), storage.BlockDbDirName)
	fi, err := os.Stat(blockDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		err = os.MkdirAll(blockDir, 0755)
		if err != nil {
			return nil, err
		}
	} else if !fi.IsDir() {
		return nil, errors.New("file already exist: " + blockDir)
	}
	records, err := cdb.getAllBlockFileMeta()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		file0, err := cdb.initBlockFileMeta()
		if err != nil {
			logging.CPrint(logging.ERROR, "init block file meta failed", logging.LogFormat{"err": err})
			return nil, err
		}
		records = append(records, file0)
	}
	cdb.blockFileKeeper = disk.NewBlockFileKeeper(blockDir, records)
	return cdb, nil
}

func (db *ChainDb) close() error {
	return db.localStorage.Close()
}

// Sync verifies that the database is coherent on disk,
// and no outstanding transactions are in flight.
func (db *ChainDb) Sync() error {
	// while specified by the API, does nothing
	// however does grab lock to verify it does not return until other operations are complete.
	return nil
}

// Close cleanly shuts down database, syncing all data.
func (db *ChainDb) Close() error {
	db.blockFileKeeper.Close()
	return db.close()
}

func (db *ChainDb) Batch(index int) *LBatch {
	if db.batches[index] == nil {
		db.batches[index] = NewLBatch(db.dbBatch)
	}
	return db.batches[index]
}

// For testing purpose
func (db *ChainDb) ExportDbEntries() map[string][]byte {
	it := db.localStorage.NewIterator(nil)
	defer it.Release()

	all := make(map[string][]byte)
	for it.Next() {
		all[string(it.Key())] = it.Value()
	}
	return all
}
