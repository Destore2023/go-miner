package ldb_test

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/config"
	"github.com/Sukhavati-Labs/go-miner/database"
	"github.com/Sukhavati-Labs/go-miner/database/ldb"
	_ "github.com/Sukhavati-Labs/go-miner/database/ldb"
	_ "github.com/Sukhavati-Labs/go-miner/database/memdb"
	"github.com/Sukhavati-Labs/go-miner/errors"
	"github.com/Sukhavati-Labs/go-miner/wire"
)

// testDbRoot is the root directory used to create all test databases.
const (
	testDbRoot = "testdbs"
	dbtype     = "leveldb"
)

var (
	blocks200 []*chainutil.Block
)

func init() {
	f, err := os.Open("./data/mockBlks.dat")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		buf, err := hex.DecodeString(scanner.Text())
		if err != nil {
			panic(err)
		}

		block, err := chainutil.NewBlockFromBytes(buf, wire.Packet)
		if err != nil {
			panic(err)
		}
		blocks200 = append(blocks200, block)
	}
}

// the 1st block is genesis
func loadNthBlock(nth int) (*chainutil.Block, error) {
	if nth <= 0 || nth > len(blocks200) {
		return nil, errors.New("invalid nth")
	}
	return blocks200[nth-1], nil
}

// the 1st block is genesis
func loadTopNBlock(n int) ([]*chainutil.Block, error) {
	if n <= 0 || n > len(blocks200) {
		return nil, errors.New("invalid n")
	}

	return blocks200[:n], nil
}

func insertBlock(db database.DB, block *chainutil.Block) error {
	err := db.SubmitBlock(block)
	if err != nil {
		return err
	}
	cdb := db.(*ldb.ChainDb)
	cdb.Batch(1).Set(*block.Hash())
	cdb.Batch(1).Done()
	return db.Commit(*block.Hash())
}

func initBlocks(db database.DB, numBlocks int) error {
	blocks, err := loadTopNBlock(numBlocks)
	if err != nil {
		return err
	}

	for i, block := range blocks {
		if i == 0 {
			continue
		}

		err = db.SubmitBlock(block)
		if err != nil {
			return err
		}
		cdb := db.(*ldb.ChainDb)
		cdb.Batch(1).Set(*block.Hash())
		cdb.Batch(1).Done()
		err = db.Commit(*block.Hash())
		if err != nil {
			return err
		}
	}

	_, height, err := db.NewestSha()
	if err != nil {
		return err
	}
	if height != uint64(numBlocks-1) {
		return errors.New("incorrect best height")
	}
	return nil
}

func GetDb(dbName string) (database.DB, func(), error) {
	// Create the root directory for test databases.
	if !fileExists(testDbRoot) {
		if err := os.MkdirAll(testDbRoot, 0700); err != nil {
			err := fmt.Errorf("unable to create test db "+"root: %v", err)
			return nil, nil, err
		}
	}
	dbPath := filepath.Join(testDbRoot, dbName)
	_ = os.RemoveAll(dbPath)
	db, err := database.Create(dbtype, dbPath)
	if err != nil {
		fmt.Println("create db error: ", err)
		return nil, nil, err
	}

	// Get the latest block height from the database.
	_, height, err := db.NewestSha()
	if err != nil {
		db.Close()
		return nil, nil, err
	}

	// Insert the appropriate genesis block for the miner network being
	// connected to if needed.
	if height == math.MaxUint64 {
		block, err := loadNthBlock(1)
		if err != nil {
			return nil, nil, err
		}
		err = db.InitByGenesisBlock(block)
		if err != nil {
			db.Close()
			return nil, nil, err
		}
		height = block.Height()
		copy(config.ChainParams.GenesisHash[:], block.Hash()[:])
	}

	// Setup a tearDown function for cleaning up.  This function is
	// returned to the caller to be invoked when it is done testing.
	tearDown := func() {
		dbVersionPath := filepath.Join(testDbRoot, dbName+".ver")
		db.Sync()
		db.Close()
		os.RemoveAll(dbPath)
		os.Remove(dbVersionPath)
		os.RemoveAll(testDbRoot)
	}
	return db, tearDown, nil
}

// filesExists returns whether or not the named file or directory exists.
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// // openDB is used to open an existing database based on the database type and
// // name.
// func openDB(dbType, dbName string) (database.Db, error) {
// 	// Handle memdb specially since it has no files on disk.
// 	if dbType == "memdb" {
// 		db, err := database.OpenDB(dbType)
// 		if err != nil {
// 			return nil, fmt.Errorf("error opening db: %v", err)
// 		}
// 		return db, nil
// 	}

// 	dbPath := filepath.Join(testDbRoot, dbName)
// 	db, err := database.OpenDB(dbType, dbPath)
// 	if err != nil {
// 		return nil, fmt.Errorf("error opening db: %v", err)
// 	}

// 	return db, nil
// }

// // createDB creates a new db instance and returns a teardown function the caller
// // should invoke when done testing to clean up.  The close flag indicates
// // whether or not the teardown function should sync and close the database
// // during teardown.
// func createDB(dbType, dbName string, close bool) (database.Db, func(), error) {
// 	// Handle memory database specially since it doesn't need the disk
// 	// specific handling.
// 	if dbType == "memdb" {
// 		db, err := database.CreateDB(dbType)
// 		if err != nil {
// 			return nil, nil, fmt.Errorf("error creating db: %v", err)
// 		}

// 		// Setup a teardown function for cleaning up.  This function is
// 		// returned to the caller to be invoked when it is done testing.
// 		teardown := func() {
// 			if close {
// 				db.Close()
// 			}
// 		}

// 		return db, teardown, nil
// 	}

// 	// Create the root directory for test databases.
// 	if !fileExists(testDbRoot) {
// 		if err := os.MkdirAll(testDbRoot, 0700); err != nil {
// 			err := fmt.Errorf("unable to create test db "+
// 				"root: %v", err)
// 			return nil, nil, err
// 		}
// 	}

// 	// Create a new database to store the accepted blocks into.
// 	dbPath := filepath.Join(testDbRoot, dbName)
// 	_ = os.RemoveAll(dbPath)
// 	db, err := database.CreateDB(dbType, dbPath)
// 	if err != nil {
// 		return nil, nil, fmt.Errorf("error creating db: %v", err)
// 	}

// 	// Setup a teardown function for cleaning up.  This function is
// 	// returned to the caller to be invoked when it is done testing.
// 	teardown := func() {
// 		dbVersionPath := filepath.Join(testDbRoot, dbName+".ver")
// 		if close {
// 			db.Sync()
// 			db.Close()
// 		}
// 		os.RemoveAll(dbPath)
// 		os.Remove(dbVersionPath)
// 		os.RemoveAll(testDbRoot)
// 	}

// 	return db, teardown, nil
// }

// // setupDB is used to create a new db instance with the genesis block already
// // inserted.  In addition to the new db instance, it returns a teardown function
// // the caller should invoke when done testing to clean up.
// func setupDB(dbType, dbName string) (database.Db, func(), error) {
// 	db, teardown, err := createDB(dbType, dbName, true)
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	// Insert the main network genesis block.  This is part of the initial
// 	// database setup.
// 	genesisBlock := chainutil.NewBlock(config.ChainParams.GenesisBlock)
// 	err = db.SubmitBlock(genesisBlock)
// 	if err != nil {
// 		teardown()
// 		err := fmt.Errorf("failed to insert genesis block: %v", err)
// 		return nil, nil, err
// 	}

// 	return db, teardown, nil
// }

// // loadBlocks loads the blocks contained in the testdata directory and returns
// // a slice of them.
// func loadBlocks(t *testing.T) ([]*chainutil.Block, error) {
// 	if len(savedBlocks) != 0 {
// 		return savedBlocks, nil
// 	}

// 	var dr io.Reader
// 	fi, err := os.Open(blockDataFile)
// 	if err != nil {
// 		t.Errorf("failed to open file %v, err %v", blockDataFile, err)
// 		return nil, err
// 	}
// 	if strings.HasSuffix(blockDataFile, ".bz2") {
// 		z := bzip2.NewReader(fi)
// 		dr = z
// 	} else {
// 		dr = fi
// 	}

// 	defer func() {
// 		if err := fi.Close(); err != nil {
// 			t.Errorf("failed to close file %v %v", blockDataFile, err)
// 		}
// 	}()

// 	// Set the first block as the genesis block.
// 	blocks := make([]*chainutil.Block, 0, 256)
// 	genesis := chainutil.NewBlock(config.ChainParams.GenesisBlock)
// 	blocks = append(blocks, genesis)

// 	for height := int64(1); err == nil; height++ {
// 		var rintbuf uint32
// 		err := binary.Read(dr, binary.LittleEndian, &rintbuf)
// 		if err == io.EOF {
// 			// hit end of file at expected offset: no warning
// 			//height--
// 			//err = nil
// 			break
// 		}
// 		if err != nil {
// 			t.Errorf("failed to load network type, err %v", err)
// 			break
// 		}
// 		if rintbuf != uint32(network) {
// 			t.Errorf("Block doesn't match network: %v expects %v",
// 				rintbuf, network)
// 			break
// 		}
// 		err = binary.Read(dr, binary.LittleEndian, &rintbuf)
// 		if err != nil {
// 			t.Errorf("failed to read length of block")
// 			break
// 		}
// 		blocklen := rintbuf

// 		rbytes := make([]byte, blocklen)

// 		// read block
// 		dr.Read(rbytes)

// 		block, err := chainutil.NewBlockFromBytes(rbytes, wire.DB)
// 		if err != nil {
// 			t.Errorf("failed to parse block %v", height)
// 			return nil, err
// 		}
// 		blocks = append(blocks, block)
// 	}

// 	savedBlocks = blocks
// 	return blocks, nil
// }
